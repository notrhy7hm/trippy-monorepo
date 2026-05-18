package budget

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/trippyai/trippy/backend/internal/api"
	"github.com/trippyai/trippy/backend/internal/trips"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

// splitReq is one split as sent on the wire. userId is a string so the
// handler can return a clean 400 on a malformed UUID rather than failing
// JSON decode.
type splitReq struct {
	UserID     string `json:"userId"`
	ShareCents int64  `json:"shareCents"`
}

// ---- list -----------------------------------------------------------------

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tripSlug")
	expenses, err := h.svc.ListExpenses(r.Context(), api.UserID(r.Context()), slug)
	if err != nil {
		writeError(w, err)
		return
	}
	if expenses == nil {
		expenses = []Expense{}
	}
	api.JSON(w, http.StatusOK, expenses)
}

// ---- create ---------------------------------------------------------------

type createReq struct {
	Title        string     `json:"title"`
	AmountCents  int64      `json:"amountCents"`
	Currency     string     `json:"currency"`
	Category     string     `json:"category"`
	PaidByUserID string     `json:"paidByUserId"`
	ExpenseDate  string     `json:"expenseDate"`
	Notes        string     `json:"notes"`
	Splits       []splitReq `json:"splits"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in createReq
	if err := api.DecodeJSON(r, &in); err != nil {
		api.Err(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	slug := chi.URLParam(r, "tripSlug")

	paidBy, err := uuid.Parse(strings.TrimSpace(in.PaidByUserID))
	if err != nil {
		api.Err(w, http.StatusBadRequest, "invalid_paid_by",
			"paidByUserId must be a valid user id")
		return
	}
	splits, ok := parseSplits(in.Splits)
	if !ok {
		api.Err(w, http.StatusBadRequest, "invalid_split_user",
			"each split userId must be a valid user id")
		return
	}

	create := CreateInput{
		Title:        in.Title,
		AmountCents:  in.AmountCents,
		Currency:     in.Currency,
		Category:     in.Category,
		PaidByUserID: paidBy,
		Notes:        in.Notes,
		Splits:       splits,
	}
	if d := strings.TrimSpace(in.ExpenseDate); d != "" {
		parsed, err := parseDate(d)
		if err != nil {
			api.Err(w, http.StatusBadRequest, "invalid_expense_date",
				"expenseDate must be in YYYY-MM-DD format")
			return
		}
		create.ExpenseDate = &parsed
	}

	e, err := h.svc.CreateExpense(r.Context(), api.UserID(r.Context()), slug, create)
	if err != nil {
		writeError(w, err)
		return
	}
	api.JSON(w, http.StatusCreated, e)
}

// ---- update ---------------------------------------------------------------

// updateReq uses pointers so the handler can tell field-omitted from
// field-present. A present-but-empty expenseDate clears the column; an
// omitted splits array keeps the existing splits, a present one replaces
// them.
type updateReq struct {
	Title        *string     `json:"title,omitempty"`
	AmountCents  *int64      `json:"amountCents,omitempty"`
	Currency     *string     `json:"currency,omitempty"`
	Category     *string     `json:"category,omitempty"`
	PaidByUserID *string     `json:"paidByUserId,omitempty"`
	ExpenseDate  *string     `json:"expenseDate,omitempty"`
	Notes        *string     `json:"notes,omitempty"`
	Splits       *[]splitReq `json:"splits,omitempty"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var in updateReq
	if err := api.DecodeJSON(r, &in); err != nil {
		api.Err(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	slug := chi.URLParam(r, "tripSlug")
	expenseID, err := uuid.Parse(chi.URLParam(r, "expenseID"))
	if err != nil {
		api.Err(w, http.StatusNotFound, "expense_not_found", "expense not found")
		return
	}

	upd := UpdateInput{
		Title:       in.Title,
		AmountCents: in.AmountCents,
		Currency:    in.Currency,
		Category:    in.Category,
		Notes:       in.Notes,
	}
	if in.PaidByUserID != nil {
		paidBy, err := uuid.Parse(strings.TrimSpace(*in.PaidByUserID))
		if err != nil {
			api.Err(w, http.StatusBadRequest, "invalid_paid_by",
				"paidByUserId must be a valid user id")
			return
		}
		upd.PaidByUserID = &paidBy
	}
	if in.ExpenseDate != nil {
		upd.SetExpenseDate = true
		if d := strings.TrimSpace(*in.ExpenseDate); d != "" {
			parsed, err := parseDate(d)
			if err != nil {
				api.Err(w, http.StatusBadRequest, "invalid_expense_date",
					"expenseDate must be in YYYY-MM-DD format")
				return
			}
			upd.ExpenseDate = &parsed
		}
		// present-but-empty -> ExpenseDate stays nil -> column cleared.
	}
	if in.Splits != nil {
		splits, ok := parseSplits(*in.Splits)
		if !ok {
			api.Err(w, http.StatusBadRequest, "invalid_split_user",
				"each split userId must be a valid user id")
			return
		}
		upd.SplitsSet = true
		upd.Splits = splits
	}

	e, err := h.svc.UpdateExpense(r.Context(), api.UserID(r.Context()), slug, expenseID, upd)
	if err != nil {
		writeError(w, err)
		return
	}
	api.JSON(w, http.StatusOK, e)
}

// ---- delete ---------------------------------------------------------------

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tripSlug")
	expenseID, err := uuid.Parse(chi.URLParam(r, "expenseID"))
	if err != nil {
		api.Err(w, http.StatusNotFound, "expense_not_found", "expense not found")
		return
	}
	if err := h.svc.DeleteExpense(r.Context(), api.UserID(r.Context()), slug, expenseID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- summary --------------------------------------------------------------

func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tripSlug")
	summary, err := h.svc.Summary(r.Context(), api.UserID(r.Context()), slug)
	if err != nil {
		writeError(w, err)
		return
	}
	api.JSON(w, http.StatusOK, summary)
}

// ---- helpers --------------------------------------------------------------

func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// parseSplits converts the wire splits into SplitInput, returning ok=false
// if any userId is not a valid UUID. An empty input is valid here (the
// "at least one split" rule is enforced in the service).
func parseSplits(reqs []splitReq) ([]SplitInput, bool) {
	out := make([]SplitInput, 0, len(reqs))
	for _, s := range reqs {
		uid, err := uuid.Parse(strings.TrimSpace(s.UserID))
		if err != nil {
			return nil, false
		}
		out = append(out, SplitInput{UserID: uid, ShareCents: s.ShareCents})
	}
	return out, true
}

// writeError centralizes typed-error -> HTTP mapping. Sibling-package
// errors (trips.ErrNotFound / trips.ErrForbidden) are translated here so
// the budget handler never returns a raw DB error or another package's
// message verbatim.
func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidTitle):
		api.Err(w, http.StatusBadRequest, "invalid_title", "title must not be empty")
	case errors.Is(err, ErrInvalidAmount):
		api.Err(w, http.StatusBadRequest, "invalid_amount",
			"amountCents must be a positive integer")
	case errors.Is(err, ErrInvalidCurrency):
		api.Err(w, http.StatusBadRequest, "invalid_currency",
			"currency must be a 3-letter uppercase code")
	case errors.Is(err, ErrInvalidShare):
		api.Err(w, http.StatusBadRequest, "invalid_share",
			"split shareCents must be non-negative")
	case errors.Is(err, ErrSplitsRequired):
		api.Err(w, http.StatusBadRequest, "splits_required",
			"at least one split is required")
	case errors.Is(err, ErrDuplicateSplitUser):
		api.Err(w, http.StatusBadRequest, "duplicate_split_user",
			"a user appears in more than one split")
	case errors.Is(err, ErrSplitSumMismatch):
		api.Err(w, http.StatusBadRequest, "split_sum_mismatch",
			"the split shares must sum to the expense amount")
	case errors.Is(err, ErrPaidByNotMember):
		api.Err(w, http.StatusUnprocessableEntity, "paid_by_not_member",
			"the payer must be a trip member")
	case errors.Is(err, ErrSplitUserNotMember):
		api.Err(w, http.StatusUnprocessableEntity, "split_user_not_member",
			"every split user must be a trip member")
	case errors.Is(err, ErrExpenseNotFound):
		api.Err(w, http.StatusNotFound, "expense_not_found", "expense not found")
	case errors.Is(err, trips.ErrNotFound):
		api.Err(w, http.StatusNotFound, "trip_not_found", "trip not found")
	case errors.Is(err, trips.ErrForbidden):
		api.Err(w, http.StatusForbidden, "forbidden",
			"you do not have access to this trip")
	default:
		slog.Error("budget internal error", "err", err)
		api.Err(w, http.StatusInternalServerError, "internal", "something went wrong")
	}
}
