package budget

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/trippyai/trippy/backend/internal/trips"
)

var (
	ErrInvalidTitle       = errors.New("title must not be empty")
	ErrInvalidAmount      = errors.New("amount must be a positive integer")
	ErrInvalidCurrency    = errors.New("currency must be a 3-letter uppercase code")
	ErrInvalidShare       = errors.New("split share must be non-negative")
	ErrSplitsRequired     = errors.New("at least one split is required")
	ErrDuplicateSplitUser = errors.New("a user appears in more than one split")
	ErrPaidByNotMember    = errors.New("payer is not a trip member")
	ErrSplitUserNotMember = errors.New("split user is not a trip member")
)

// TripAccess is the minimum surface budget needs from the trips package.
// trips.Service satisfies it structurally. Keeping it small avoids an
// import cycle and keeps the budget module testable in isolation.
type TripAccess interface {
	// AssertMember returns the trip's UUID if userID belongs to the trip
	// addressed by slug, else trips.ErrNotFound / trips.ErrForbidden.
	AssertMember(ctx context.Context, slug string, userID uuid.UUID) (uuid.UUID, error)
	// AssertMemberRole returns the caller's trip role alongside the trip id.
	AssertMemberRole(ctx context.Context, slug string, userID uuid.UUID) (uuid.UUID, trips.Role, error)
	// ListMembers returns the trip's members (the caller must be able to
	// see the trip). budget uses it both to validate payer / split users
	// and to label rows in the budget summary.
	ListMembers(ctx context.Context, slug string, userID uuid.UUID) ([]trips.Member, error)
}

type Service struct {
	repo  *Repo
	trips TripAccess
}

func NewService(r *Repo, t TripAccess) *Service {
	return &Service{repo: r, trips: t}
}

// ListExpenses returns the trip's expenses (each with splits) if the
// caller is a trip member.
func (s *Service) ListExpenses(ctx context.Context, callerID uuid.UUID, slug string) ([]Expense, error) {
	tripID, err := s.trips.AssertMember(ctx, slug, callerID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListExpenses(ctx, tripID)
}

// CreateExpense validates and inserts a new expense with its splits. Budget
// managers only.
func (s *Service) CreateExpense(ctx context.Context, callerID uuid.UUID, slug string, in CreateInput) (Expense, error) {
	tripID, err := s.assertBudgetManager(ctx, slug, callerID)
	if err != nil {
		return Expense{}, err
	}

	title := strings.TrimSpace(in.Title)
	if title == "" {
		return Expense{}, ErrInvalidTitle
	}
	if in.AmountCents <= 0 {
		return Expense{}, ErrInvalidAmount
	}
	currency, err := normalizeCurrency(in.Currency)
	if err != nil {
		return Expense{}, err
	}
	category := strings.TrimSpace(in.Category)
	if category == "" {
		category = DefaultCategory
	}
	if len(in.Splits) == 0 {
		return Expense{}, ErrSplitsRequired
	}

	members, err := s.tripMemberSet(ctx, slug, callerID)
	if err != nil {
		return Expense{}, err
	}
	if !members[in.PaidByUserID] {
		return Expense{}, ErrPaidByNotMember
	}
	if err := validateSplitsShape(in.Splits, members); err != nil {
		return Expense{}, err
	}
	// Create knows both the amount and the splits up front, so the
	// sum invariant is checked here. (Update may not know the final
	// amount without a row read, so its sum check lives in the repo tx.)
	if splitsSum(in.Splits) != in.AmountCents {
		return Expense{}, ErrSplitSumMismatch
	}

	return s.repo.CreateExpense(ctx, CreateExpenseRow{
		TripID:          tripID,
		Title:           title,
		AmountCents:     in.AmountCents,
		Currency:        currency,
		Category:        category,
		PaidByUserID:    in.PaidByUserID,
		ExpenseDate:     in.ExpenseDate,
		Notes:           in.Notes,
		CreatedByUserID: callerID,
		Splits:          in.Splits,
	})
}

// UpdateExpense applies a partial update. Per-field shape validation runs
// here; the splits-sum-equals-amount invariant is enforced inside the repo
// transaction (it can depend on the stored amount). Omitted splits are
// kept; provided splits replace the existing set wholesale.
func (s *Service) UpdateExpense(ctx context.Context, callerID uuid.UUID, slug string, expenseID uuid.UUID, in UpdateInput) (Expense, error) {
	tripID, err := s.assertBudgetManager(ctx, slug, callerID)
	if err != nil {
		return Expense{}, err
	}

	// Confirm the expense exists (and belongs to this trip) before any
	// field validation, so a stale-expense 404 wins over field errors.
	if _, err := s.repo.ExpenseByIDForTrip(ctx, tripID, expenseID); err != nil {
		return Expense{}, err
	}

	row := UpdateExpenseRow{}

	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if t == "" {
			return Expense{}, ErrInvalidTitle
		}
		row.Title = &t
	}
	if in.AmountCents != nil {
		if *in.AmountCents <= 0 {
			return Expense{}, ErrInvalidAmount
		}
		row.AmountCents = in.AmountCents
	}
	if in.Currency != nil {
		c := strings.TrimSpace(*in.Currency)
		if !validCurrencyCode(c) {
			return Expense{}, ErrInvalidCurrency
		}
		row.Currency = &c
	}
	if in.Category != nil {
		c := strings.TrimSpace(*in.Category)
		if c == "" {
			c = DefaultCategory
		}
		row.Category = &c
	}
	if in.Notes != nil {
		row.Notes = in.Notes
	}
	if in.SetExpenseDate {
		row.SetExpenseDate = true
		row.ExpenseDate = in.ExpenseDate
	}

	// Payer / split users are validated against the trip's member set;
	// fetch it once, only when one of those fields is being changed.
	if in.PaidByUserID != nil || in.SplitsSet {
		members, err := s.tripMemberSet(ctx, slug, callerID)
		if err != nil {
			return Expense{}, err
		}
		if in.PaidByUserID != nil {
			if !members[*in.PaidByUserID] {
				return Expense{}, ErrPaidByNotMember
			}
			row.PaidByUserID = in.PaidByUserID
		}
		if in.SplitsSet {
			if len(in.Splits) == 0 {
				return Expense{}, ErrSplitsRequired
			}
			if err := validateSplitsShape(in.Splits, members); err != nil {
				return Expense{}, err
			}
			row.SplitsSet = true
			row.Splits = in.Splits
		}
	}

	return s.repo.UpdateExpense(ctx, tripID, expenseID, row)
}

// DeleteExpense hard-deletes an expense (its splits cascade). Budget managers
// only.
func (s *Service) DeleteExpense(ctx context.Context, callerID uuid.UUID, slug string, expenseID uuid.UUID) error {
	tripID, err := s.assertBudgetManager(ctx, slug, callerID)
	if err != nil {
		return err
	}
	return s.repo.DeleteExpense(ctx, tripID, expenseID)
}

func (s *Service) assertBudgetManager(ctx context.Context, slug string, callerID uuid.UUID) (uuid.UUID, error) {
	tripID, role, err := s.trips.AssertMemberRole(ctx, slug, callerID)
	if err != nil {
		return uuid.Nil, err
	}
	if !trips.CanManageBudget(role) {
		return uuid.Nil, trips.ErrForbidden
	}
	return tripID, nil
}

// Summary computes the budget summary for a trip: per-member paid / owed /
// balance plus suggested settlements, grouped by currency (M3 does no
// currency conversion).
func (s *Service) Summary(ctx context.Context, callerID uuid.UUID, slug string) (Summary, error) {
	tripID, err := s.trips.AssertMember(ctx, slug, callerID)
	if err != nil {
		return Summary{}, err
	}
	members, err := s.trips.ListMembers(ctx, slug, callerID)
	if err != nil {
		return Summary{}, err
	}
	expenses, err := s.repo.ListExpenses(ctx, tripID)
	if err != nil {
		return Summary{}, err
	}
	return buildSummary(members, expenses), nil
}

// ---------------------------------------------------------------------------
// validation helpers
// ---------------------------------------------------------------------------

// tripMemberSet returns the trip's member user IDs as a set.
func (s *Service) tripMemberSet(ctx context.Context, slug string, callerID uuid.UUID) (map[uuid.UUID]bool, error) {
	members, err := s.trips.ListMembers(ctx, slug, callerID)
	if err != nil {
		return nil, err
	}
	set := make(map[uuid.UUID]bool, len(members))
	for _, m := range members {
		set[m.UserID] = true
	}
	return set, nil
}

// validateSplitsShape checks the per-split rules that do not depend on the
// expense amount: non-negative shares, no duplicate users, and every user
// is a trip member. The sum-equals-amount rule is checked separately.
func validateSplitsShape(splits []SplitInput, members map[uuid.UUID]bool) error {
	seen := make(map[uuid.UUID]bool, len(splits))
	for _, sp := range splits {
		if sp.ShareCents < 0 {
			return ErrInvalidShare
		}
		if seen[sp.UserID] {
			return ErrDuplicateSplitUser
		}
		seen[sp.UserID] = true
		if !members[sp.UserID] {
			return ErrSplitUserNotMember
		}
	}
	return nil
}

func splitsSum(splits []SplitInput) int64 {
	var sum int64
	for _, sp := range splits {
		sum += sp.ShareCents
	}
	return sum
}

// normalizeCurrency resolves the optional create-time currency: empty ->
// DefaultCurrency, otherwise it must be a 3-letter uppercase code.
func normalizeCurrency(c string) (string, error) {
	c = strings.TrimSpace(c)
	if c == "" {
		return DefaultCurrency, nil
	}
	if !validCurrencyCode(c) {
		return "", ErrInvalidCurrency
	}
	return c, nil
}

// validCurrencyCode reports whether c is exactly three uppercase ASCII
// letters — the same shape the trip_expenses.currency CHECK enforces.
func validCurrencyCode(c string) bool {
	if len(c) != 3 {
		return false
	}
	for _, r := range c {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// summary computation
// ---------------------------------------------------------------------------

// buildSummary turns the trip's members + expenses into a per-currency
// budget summary. Each currency is independent — M3 never converts.
func buildSummary(members []trips.Member, expenses []Expense) Summary {
	type label struct{ username, displayName string }
	labelByID := make(map[uuid.UUID]label, len(members))
	memberOrder := make([]uuid.UUID, 0, len(members))
	for _, m := range members {
		labelByID[m.UserID] = label{m.Username, m.DisplayName}
		memberOrder = append(memberOrder, m.UserID)
	}

	type acc struct{ paid, share int64 }
	type curData struct {
		total int64
		accs  map[uuid.UUID]*acc
		extra []uuid.UUID // participants who are not current trip members
	}
	currencies := make(map[string]*curData)

	getCur := func(c string) *curData {
		cd, ok := currencies[c]
		if !ok {
			cd = &curData{accs: make(map[uuid.UUID]*acc)}
			currencies[c] = cd
		}
		return cd
	}
	getAcc := func(cd *curData, uid uuid.UUID) *acc {
		a, ok := cd.accs[uid]
		if !ok {
			a = &acc{}
			cd.accs[uid] = a
			if _, isMember := labelByID[uid]; !isMember {
				cd.extra = append(cd.extra, uid)
			}
		}
		return a
	}

	for _, e := range expenses {
		cd := getCur(e.Currency)
		cd.total += e.AmountCents
		getAcc(cd, e.PaidByUserID).paid += e.AmountCents
		for _, sp := range e.Splits {
			getAcc(cd, sp.UserID).share += sp.ShareCents
		}
	}

	curOrder := make([]string, 0, len(currencies))
	for c := range currencies {
		curOrder = append(curOrder, c)
	}
	sort.Strings(curOrder)

	out := Summary{Currencies: make([]CurrencySummary, 0, len(curOrder))}
	for _, c := range curOrder {
		cd := currencies[c]
		// Row order: trip members in join order, then any non-member
		// participants (e.g. a member removed after paying) in first-
		// seen order, so balances always net to zero.
		ids := make([]uuid.UUID, 0, len(memberOrder)+len(cd.extra))
		ids = append(ids, memberOrder...)
		ids = append(ids, cd.extra...)

		balances := make([]MemberBalance, 0, len(ids))
		for _, uid := range ids {
			var paid, share int64
			if a := cd.accs[uid]; a != nil {
				paid, share = a.paid, a.share
			}
			balances = append(balances, MemberBalance{
				UserID:       uid,
				Username:     labelByID[uid].username,
				DisplayName:  labelByID[uid].displayName,
				PaidCents:    paid,
				ShareCents:   share,
				BalanceCents: paid - share,
			})
		}

		out.Currencies = append(out.Currencies, CurrencySummary{
			Currency:    c,
			TotalCents:  cd.total,
			Members:     balances,
			Settlements: settle(balances),
		})
	}
	return out
}

// settle greedily matches debtors (negative balance) to creditors
// (positive balance) within a single currency. Debtors and creditors are
// each sorted by amount descending (id as tie-break) so the result is
// deterministic. Because every payer and split user has a row, the
// balances sum to zero and the matching fully settles.
func settle(balances []MemberBalance) []Settlement {
	type bal struct {
		id  uuid.UUID
		amt int64
	}
	var debtors, creditors []bal
	for _, m := range balances {
		switch {
		case m.BalanceCents < 0:
			debtors = append(debtors, bal{m.UserID, -m.BalanceCents})
		case m.BalanceCents > 0:
			creditors = append(creditors, bal{m.UserID, m.BalanceCents})
		}
	}
	byAmountDesc := func(s []bal) func(i, j int) bool {
		return func(i, j int) bool {
			if s[i].amt != s[j].amt {
				return s[i].amt > s[j].amt
			}
			return s[i].id.String() < s[j].id.String()
		}
	}
	sort.Slice(debtors, byAmountDesc(debtors))
	sort.Slice(creditors, byAmountDesc(creditors))

	out := []Settlement{}
	i, j := 0, 0
	for i < len(debtors) && j < len(creditors) {
		pay := min(debtors[i].amt, creditors[j].amt)
		if pay > 0 {
			out = append(out, Settlement{
				DebtorUserID:   debtors[i].id,
				CreditorUserID: creditors[j].id,
				AmountCents:    pay,
			})
		}
		debtors[i].amt -= pay
		creditors[j].amt -= pay
		if debtors[i].amt == 0 {
			i++
		}
		if creditors[j].amt == 0 {
			j++
		}
	}
	return out
}
