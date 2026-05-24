package budget

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ErrExpenseNotFound is returned for a missing expense, or for an expense
// id that belongs to a different trip than the one in the request path.
var ErrExpenseNotFound = errors.New("expense not found")

// ErrSplitSumMismatch is raised inside the update transaction when the
// final set of split shares does not add up to the final expense amount.
// Create-time sum mismatches are caught earlier, in the service.
var ErrSplitSumMismatch = errors.New("splits must sum to the expense amount")

type Repo struct{ db *sqlx.DB }

func NewRepo(db *sqlx.DB) *Repo { return &Repo{db: db} }

// expenseRow is the scan shape for trip_expenses reads. expense_date is
// read via ::text so it never round-trips through time.Time (which would
// otherwise risk a timezone shift on the date-only column).
type expenseRow struct {
	ID              uuid.UUID      `db:"id"`
	TripID          uuid.UUID      `db:"trip_id"`
	Title           string         `db:"title"`
	AmountCents     int64          `db:"amount_cents"`
	Currency        string         `db:"currency"`
	Category        string         `db:"category"`
	PaidByUserID    uuid.UUID      `db:"paid_by_user_id"`
	ExpenseDate     sql.NullString `db:"expense_date"`
	Notes           string         `db:"notes"`
	CreatedByUserID uuid.UUID      `db:"created_by_user_id"`
	CreatedAt       time.Time      `db:"created_at"`
	UpdatedAt       time.Time      `db:"updated_at"`
}

type splitRow struct {
	ID         uuid.UUID `db:"id"`
	ExpenseID  uuid.UUID `db:"expense_id"`
	UserID     uuid.UUID `db:"user_id"`
	ShareCents int64     `db:"share_cents"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

const expenseSelectColumns = `
	id, trip_id, title, amount_cents, currency, category,
	paid_by_user_id, expense_date::text AS expense_date, notes,
	created_by_user_id, created_at, updated_at
`

// expenseListOrder pins the deterministic order required by the spec:
// most recent expense_date first (undated rows last), then newest first,
// then id as a stable final tie-break.
const expenseListOrder = `
	ORDER BY expense_date DESC NULLS LAST, created_at DESC, id ASC
`

func rowToExpense(r expenseRow) Expense {
	e := Expense{
		ID:              r.ID,
		TripID:          r.TripID,
		Title:           r.Title,
		AmountCents:     r.AmountCents,
		Currency:        r.Currency,
		Category:        r.Category,
		PaidByUserID:    r.PaidByUserID,
		Notes:           r.Notes,
		CreatedByUserID: r.CreatedByUserID,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
		Splits:          []Split{},
	}
	if r.ExpenseDate.Valid {
		s := r.ExpenseDate.String
		e.ExpenseDate = &s
	}
	return e
}

func rowToSplit(r splitRow) Split {
	return Split{
		ID:         r.ID,
		ExpenseID:  r.ExpenseID,
		UserID:     r.UserID,
		ShareCents: r.ShareCents,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}

// selectSplits reads one expense's splits in deterministic order. It takes
// a sqlx.QueryerContext so it works against either the pool or an open
// transaction (used by both reads and the post-write re-read).
func selectSplits(ctx context.Context, q sqlx.QueryerContext, expenseID uuid.UUID) ([]Split, error) {
	var rows []splitRow
	err := sqlx.SelectContext(ctx, q, &rows, `
		SELECT id, expense_id, user_id, share_cents, created_at, updated_at
		FROM trip_expense_splits
		WHERE expense_id = $1
		ORDER BY created_at ASC, id ASC
	`, expenseID)
	if err != nil {
		return nil, err
	}
	out := make([]Split, 0, len(rows))
	for _, r := range rows {
		out = append(out, rowToSplit(r))
	}
	return out, nil
}

// loadExpense reads a single expense plus its splits, scoped to its trip.
// Works against the pool or a transaction. Cross-trip ids surface as
// ErrExpenseNotFound rather than returning the wrong row.
func loadExpense(ctx context.Context, q sqlx.QueryerContext, tripID, expenseID uuid.UUID) (Expense, error) {
	var row expenseRow
	err := sqlx.GetContext(ctx, q, &row, `
		SELECT `+expenseSelectColumns+`
		FROM trip_expenses
		WHERE trip_id = $1 AND id = $2
	`, tripID, expenseID)
	if errors.Is(err, sql.ErrNoRows) {
		return Expense{}, ErrExpenseNotFound
	}
	if err != nil {
		return Expense{}, err
	}
	e := rowToExpense(row)
	splits, err := selectSplits(ctx, q, expenseID)
	if err != nil {
		return Expense{}, err
	}
	e.Splits = splits
	return e, nil
}

// insertSplits writes the given splits for an expense inside an open tx.
func insertSplits(ctx context.Context, tx *sqlx.Tx, expenseID uuid.UUID, splits []SplitInput) error {
	for _, s := range splits {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO trip_expense_splits (expense_id, user_id, share_cents)
			VALUES ($1, $2, $3)
		`, expenseID, s.UserID, s.ShareCents); err != nil {
			return err
		}
	}
	return nil
}

// ListExpenses returns every expense on the trip, each with its splits, in
// the spec's deterministic order. Splits are fetched in one extra query
// and grouped in memory to avoid an N+1.
func (r *Repo) ListExpenses(ctx context.Context, tripID uuid.UUID) ([]Expense, error) {
	var erows []expenseRow
	if err := r.db.SelectContext(ctx, &erows, `
		SELECT `+expenseSelectColumns+`
		FROM trip_expenses
		WHERE trip_id = $1
	`+expenseListOrder, tripID); err != nil {
		return nil, err
	}

	expenses := make([]Expense, 0, len(erows))
	indexByID := make(map[uuid.UUID]int, len(erows))
	for i, er := range erows {
		expenses = append(expenses, rowToExpense(er))
		indexByID[er.ID] = i
	}

	var srows []splitRow
	if err := r.db.SelectContext(ctx, &srows, `
		SELECT s.id, s.expense_id, s.user_id, s.share_cents,
		       s.created_at, s.updated_at
		FROM trip_expense_splits s
		JOIN trip_expenses e ON e.id = s.expense_id
		WHERE e.trip_id = $1
		ORDER BY s.created_at ASC, s.id ASC
	`, tripID); err != nil {
		return nil, err
	}
	for _, sr := range srows {
		if i, ok := indexByID[sr.ExpenseID]; ok {
			expenses[i].Splits = append(expenses[i].Splits, rowToSplit(sr))
		}
	}
	return expenses, nil
}

// ExpenseByIDForTrip returns a single expense (with splits) scoped to its
// trip; a missing / cross-trip id surfaces as ErrExpenseNotFound.
func (r *Repo) ExpenseByIDForTrip(ctx context.Context, tripID, expenseID uuid.UUID) (Expense, error) {
	return loadExpense(ctx, r.db, tripID, expenseID)
}

// CreateExpenseRow is the repo-shaped insert payload. The service has
// already validated every field (including that the splits sum to
// AmountCents) by the time this is called.
type CreateExpenseRow struct {
	TripID          uuid.UUID
	Title           string
	AmountCents     int64
	Currency        string
	Category        string
	PaidByUserID    uuid.UUID
	ExpenseDate     *time.Time
	Notes           string
	CreatedByUserID uuid.UUID
	Splits          []SplitInput
}

// CreateExpense inserts an expense and its splits atomically: the expense
// row and every split row commit together or not at all.
func (r *Repo) CreateExpense(ctx context.Context, in CreateExpenseRow) (Expense, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Expense{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var id uuid.UUID
	if err := tx.QueryRowxContext(ctx, `
		INSERT INTO trip_expenses
			(trip_id, title, amount_cents, currency, category,
			 paid_by_user_id, expense_date, notes, created_by_user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`,
		in.TripID, in.Title, in.AmountCents, in.Currency, in.Category,
		in.PaidByUserID, in.ExpenseDate, in.Notes, in.CreatedByUserID,
	).Scan(&id); err != nil {
		return Expense{}, err
	}

	if err := insertSplits(ctx, tx, id, in.Splits); err != nil {
		return Expense{}, err
	}

	e, err := loadExpense(ctx, tx, in.TripID, id)
	if err != nil {
		return Expense{}, err
	}
	if err := tx.Commit(); err != nil {
		return Expense{}, err
	}
	return e, nil
}

// UpdateExpenseRow is the repo-shaped patch payload. Scalar pointers are
// tri-state (nil = leave column alone). SetExpenseDate gates the nullable
// date column. SplitsSet gates split replacement: when false the existing
// splits are kept.
type UpdateExpenseRow struct {
	Title        *string
	AmountCents  *int64
	Currency     *string
	Category     *string
	PaidByUserID *uuid.UUID
	Notes        *string

	SetExpenseDate bool
	ExpenseDate    *time.Time

	SplitsSet bool
	Splits    []SplitInput
}

// UpdateExpense applies a partial update inside a single transaction.
//
//  1. SELECT ... FOR UPDATE locks the expense row and reads its current
//     amount. Missing row -> ErrExpenseNotFound.
//  2. The final amount is the new amount (if the request changes it) or
//     the locked current amount. The split-sum invariant is then checked
//     under the row lock so two concurrent PATCHes cannot both pass a
//     stale-row check and commit a broken (sum != amount) state. When the
//     request provides splits, the provided shares must sum to the final
//     amount and the old splits are deleted and replaced; when it omits
//     them, the existing splits must still sum to the final amount — this
//     is what rejects an amount change the untouched splits no longer
//     cover.
//  3. A dynamic UPDATE applies the set scalar fields; updated_at = now()
//     is always written.
//  4. The joined expense is re-read inside the tx and returned.
func (r *Repo) UpdateExpense(ctx context.Context, tripID, expenseID uuid.UUID, in UpdateExpenseRow) (Expense, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Expense{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var currentAmount int64
	err = tx.QueryRowxContext(ctx, `
		SELECT amount_cents FROM trip_expenses
		WHERE trip_id = $1 AND id = $2
		FOR UPDATE
	`, tripID, expenseID).Scan(&currentAmount)
	if errors.Is(err, sql.ErrNoRows) {
		return Expense{}, ErrExpenseNotFound
	}
	if err != nil {
		return Expense{}, err
	}

	finalAmount := currentAmount
	if in.AmountCents != nil {
		finalAmount = *in.AmountCents
	}

	if in.SplitsSet {
		var sum int64
		for _, s := range in.Splits {
			sum += s.ShareCents
		}
		if sum != finalAmount {
			return Expense{}, ErrSplitSumMismatch
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM trip_expense_splits WHERE expense_id = $1`, expenseID,
		); err != nil {
			return Expense{}, err
		}
		if err := insertSplits(ctx, tx, expenseID, in.Splits); err != nil {
			return Expense{}, err
		}
	} else {
		var sum sql.NullInt64
		if err := tx.GetContext(ctx, &sum, `
			SELECT SUM(share_cents) FROM trip_expense_splits
			WHERE expense_id = $1
		`, expenseID); err != nil {
			return Expense{}, err
		}
		current := int64(0)
		if sum.Valid {
			current = sum.Int64
		}
		if current != finalAmount {
			return Expense{}, ErrSplitSumMismatch
		}
	}

	parts := []string{"updated_at = now()"}
	args := []any{tripID, expenseID}
	next := 3
	add := func(col string, val any) {
		parts = append(parts, fmt.Sprintf("%s = $%d", col, next))
		args = append(args, val)
		next++
	}
	if in.Title != nil {
		add("title", *in.Title)
	}
	if in.AmountCents != nil {
		add("amount_cents", *in.AmountCents)
	}
	if in.Currency != nil {
		add("currency", *in.Currency)
	}
	if in.Category != nil {
		add("category", *in.Category)
	}
	if in.PaidByUserID != nil {
		add("paid_by_user_id", *in.PaidByUserID)
	}
	if in.Notes != nil {
		add("notes", *in.Notes)
	}
	if in.SetExpenseDate {
		add("expense_date", in.ExpenseDate) // *time.Time; nil -> NULL
	}

	query := fmt.Sprintf(`
		UPDATE trip_expenses SET %s
		WHERE trip_id = $1 AND id = $2
	`, strings.Join(parts, ", "))

	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return Expense{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Expense{}, ErrExpenseNotFound
	}

	e, err := loadExpense(ctx, tx, tripID, expenseID)
	if err != nil {
		return Expense{}, err
	}
	if err := tx.Commit(); err != nil {
		return Expense{}, err
	}
	return e, nil
}

// DeleteExpense hard-deletes an expense scoped to its trip. Its splits are
// removed by the ON DELETE CASCADE on trip_expense_splits.expense_id.
func (r *Repo) DeleteExpense(ctx context.Context, tripID, expenseID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM trip_expenses WHERE trip_id = $1 AND id = $2
	`, tripID, expenseID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrExpenseNotFound
	}
	return nil
}
