// Package budget owns per-trip expense tracking (M3). It is a peer of the
// trips package, not a child: trip access (slug -> tripID, membership,
// member list) is injected via a small interface so budget never imports
// trips.Repo or trips internals.
//
// Money is always integer cents (int64). Splits, sums, and the balance /
// settlement math are therefore exact integer arithmetic — no float
// rounding drift.
package budget

import (
	"time"

	"github.com/google/uuid"
)

// DefaultCurrency is applied when an expense create omits currency.
const DefaultCurrency = "EUR"

// DefaultCategory is applied when an expense create omits (or blanks) the
// category field.
const DefaultCategory = "other"

// Split is one participant's share of an expense, in integer cents.
type Split struct {
	ID         uuid.UUID `json:"id"`
	ExpenseID  uuid.UUID `json:"expenseId"`
	UserID     uuid.UUID `json:"userId"`
	ShareCents int64     `json:"shareCents"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

// Expense is the API-facing representation of a trip expense together with
// its splits. expenseDate is emitted as a plain "YYYY-MM-DD" string (read
// via ::text) so the JSON shape never picks up a timezone the date column
// does not carry.
type Expense struct {
	ID              uuid.UUID `json:"id"`
	TripID          uuid.UUID `json:"tripId"`
	Title           string    `json:"title"`
	AmountCents     int64     `json:"amountCents"`
	Currency        string    `json:"currency"`
	Category        string    `json:"category"`
	PaidByUserID    uuid.UUID `json:"paidByUserId"`
	ExpenseDate     *string   `json:"expenseDate,omitempty"`
	Notes           string    `json:"notes"`
	CreatedByUserID uuid.UUID `json:"createdByUserId"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	Splits          []Split   `json:"splits"`
}

// SplitInput is one split as supplied by a create / update request, after
// the handler has decoded and UUID-parsed it.
type SplitInput struct {
	UserID     uuid.UUID
	ShareCents int64
}

// CreateInput is the service-facing create payload. The handler decodes
// the JSON, parses UUIDs / the optional date, and forwards values here;
// the service applies business validation.
type CreateInput struct {
	Title        string
	AmountCents  int64
	Currency     string // empty -> DefaultCurrency
	Category     string // empty -> DefaultCategory
	PaidByUserID uuid.UUID
	ExpenseDate  *time.Time // nil -> no date
	Notes        string
	Splits       []SplitInput
}

// UpdateInput is the service-facing partial-update payload.
//
// Scalar pointer fields are tri-state: nil = leave column alone, non-nil =
// set. expense_date is nullable, so it is gated by SetExpenseDate (the nil
// pointer with the gate set means "clear to NULL").
//
// Splits is gated by SplitsSet: when false the existing splits are kept;
// when true they are replaced wholesale by Splits (an empty Splits with
// SplitsSet true is rejected as "at least one split required").
type UpdateInput struct {
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

// ---------------------------------------------------------------------------
// budget summary
// ---------------------------------------------------------------------------

// MemberBalance is one participant's standing within a single currency:
// what they paid, what they owe (sum of their split shares), and the net.
// A positive balance means the trip owes them; negative means they owe.
type MemberBalance struct {
	UserID       uuid.UUID `json:"userId"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"displayName"`
	PaidCents    int64     `json:"paidCents"`
	ShareCents   int64     `json:"shareCents"`
	BalanceCents int64     `json:"balanceCents"`
}

// Settlement is a single suggested transfer that moves money from a debtor
// to a creditor. The set of settlements, applied in full, brings every
// member balance in the currency to zero.
type Settlement struct {
	DebtorUserID   uuid.UUID `json:"debtorUserId"`
	CreditorUserID uuid.UUID `json:"creditorUserId"`
	AmountCents    int64     `json:"amountCents"`
}

// CurrencySummary is the complete budget picture for one currency. M3 does
// no currency conversion, so each currency present on the trip gets its
// own self-contained summary.
type CurrencySummary struct {
	Currency    string          `json:"currency"`
	TotalCents  int64           `json:"totalCents"`
	Members     []MemberBalance `json:"members"`
	Settlements []Settlement    `json:"settlements"`
}

// Summary is the budget/summary response: one CurrencySummary per currency
// that appears on the trip. Empty when the trip has no expenses yet.
type Summary struct {
	Currencies []CurrencySummary `json:"currencies"`
}
