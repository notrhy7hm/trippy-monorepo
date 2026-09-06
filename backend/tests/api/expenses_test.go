package api

import "testing"

// ---------------------------------------------------------------------------
// budget API response shapes (subsets of internal/budget models)
// ---------------------------------------------------------------------------

type splitResponse struct {
	ID         string `json:"id"`
	ExpenseID  string `json:"expenseId"`
	UserID     string `json:"userId"`
	ShareCents int64  `json:"shareCents"`
}

type expenseResponse struct {
	ID           string          `json:"id"`
	TripID       string          `json:"tripId"`
	Title        string          `json:"title"`
	AmountCents  int64           `json:"amountCents"`
	Currency     string          `json:"currency"`
	Category     string          `json:"category"`
	PaidByUserID string          `json:"paidByUserId"`
	ExpenseDate  *string         `json:"expenseDate,omitempty"`
	Notes        string          `json:"notes"`
	Splits       []splitResponse `json:"splits"`
}

type memberBalanceResponse struct {
	UserID       string `json:"userId"`
	Username     string `json:"username"`
	PaidCents    int64  `json:"paidCents"`
	ShareCents   int64  `json:"shareCents"`
	BalanceCents int64  `json:"balanceCents"`
}

type settlementResponse struct {
	DebtorUserID   string `json:"debtorUserId"`
	CreditorUserID string `json:"creditorUserId"`
	AmountCents    int64  `json:"amountCents"`
}

type currencySummaryResponse struct {
	Currency    string                  `json:"currency"`
	TotalCents  int64                   `json:"totalCents"`
	Members     []memberBalanceResponse `json:"members"`
	Settlements []settlementResponse    `json:"settlements"`
}

type summaryResponse struct {
	Currencies []currencySummaryResponse `json:"currencies"`
}

// ---------------------------------------------------------------------------
// budget helpers
// ---------------------------------------------------------------------------

func createExpense(t *testing.T, a actor, slug string, body any) expenseResponse {
	t.Helper()
	var r expenseResponse
	mustDo(t, "POST", "/api/v1/trips/"+slug+"/expenses", a.Token, body, 201, &r)
	return r
}

func listExpenses(t *testing.T, a actor, slug string) []expenseResponse {
	t.Helper()
	var r []expenseResponse
	mustDo(t, "GET", "/api/v1/trips/"+slug+"/expenses", a.Token, nil, 200, &r)
	return r
}

func budgetSummary(t *testing.T, a actor, slug string) summaryResponse {
	t.Helper()
	var r summaryResponse
	mustDo(t, "GET", "/api/v1/trips/"+slug+"/budget/summary", a.Token, nil, 200, &r)
	return r
}

// findBalance returns the summary row for userID, failing the test if the
// member is missing.
func findBalance(t *testing.T, members []memberBalanceResponse, userID string) memberBalanceResponse {
	t.Helper()
	for _, m := range members {
		if m.UserID == userID {
			return m
		}
	}
	t.Fatalf("no budget summary row for user %s", userID)
	return memberBalanceResponse{}
}

// splitShare returns the shareCents recorded for userID on the expense.
func splitShare(t *testing.T, e expenseResponse, userID string) int64 {
	t.Helper()
	for _, s := range e.Splits {
		if s.UserID == userID {
			return s.ShareCents
		}
	}
	t.Fatalf("no split for user %s on expense %s", userID, e.ID)
	return 0
}

// ---------------------------------------------------------------------------
// tests
// ---------------------------------------------------------------------------

// TestBudgetExpenseCreateWithSplits
//
// A trip member creates an expense with an explicit two-way split.
// currency / category default server-side, the optional date round-trips,
// and the expense (with its splits) is visible to every trip member.
func TestBudgetExpenseCreateWithSplits(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	bob := registerUser(t, "bob")
	friendsBecome(t, alice, bob)
	trip := createTrip(t, alice, "Italy")
	inviteAndJoinAs(t, alice, bob, trip.Slug, "budget_manager")

	exp := createExpense(t, alice, trip.Slug, map[string]any{
		"title":        "Hotel",
		"amountCents":  60000,
		"paidByUserId": alice.ID.String(),
		"expenseDate":  "2026-06-15",
		"splits": []map[string]any{
			{"userId": alice.ID.String(), "shareCents": 30000},
			{"userId": bob.ID.String(), "shareCents": 30000},
		},
	})

	if exp.Title != "Hotel" {
		t.Fatalf("title: got %q want Hotel", exp.Title)
	}
	if exp.AmountCents != 60000 {
		t.Fatalf("amountCents: got %d want 60000", exp.AmountCents)
	}
	if exp.Currency != "EUR" {
		t.Fatalf("currency default: got %q want EUR", exp.Currency)
	}
	if exp.Category != "other" {
		t.Fatalf("category default: got %q want other", exp.Category)
	}
	if exp.PaidByUserID != alice.ID.String() {
		t.Fatalf("paidByUserId: got %q want %q", exp.PaidByUserID, alice.ID)
	}
	if exp.ExpenseDate == nil || *exp.ExpenseDate != "2026-06-15" {
		t.Fatalf("expenseDate: got %v want 2026-06-15", exp.ExpenseDate)
	}
	if len(exp.Splits) != 2 {
		t.Fatalf("splits: got %d want 2", len(exp.Splits))
	}
	if got := splitShare(t, exp, alice.ID.String()) + splitShare(t, exp, bob.ID.String()); got != 60000 {
		t.Fatalf("split shares sum: got %d want 60000", got)
	}

	// Visible to the owner and to the other member.
	if all := listExpenses(t, alice, trip.Slug); len(all) != 1 {
		t.Fatalf("alice list size: got %d want 1", len(all))
	}
	if all := listExpenses(t, bob, trip.Slug); len(all) != 1 {
		t.Fatalf("bob list size: got %d want 1", len(all))
	}
}

// TestBudgetRoleMutationAccess
//
// Plain members can read expenses and budget summaries but cannot create,
// update, or delete expenses. The budget_manager role can mutate budget data.
func TestBudgetRoleMutationAccess(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	bob := registerUser(t, "bob")
	bea := registerUser(t, "bea")
	friendsBecome(t, alice, bob)
	friendsBecome(t, alice, bea)
	trip := createTrip(t, alice, "Italy")
	inviteAndJoin(t, alice, bob, trip.Slug)
	inviteAndJoinAs(t, alice, bea, trip.Slug, "budget_manager")

	exp := createExpense(t, alice, trip.Slug, map[string]any{
		"title":        "Hotel",
		"amountCents":  10000,
		"paidByUserId": alice.ID.String(),
		"splits": []map[string]any{
			{"userId": alice.ID.String(), "shareCents": 10000},
		},
	})

	if all := listExpenses(t, bob, trip.Slug); len(all) != 1 {
		t.Fatalf("member list size: got %d want 1", len(all))
	}
	if summary := budgetSummary(t, bob, trip.Slug); len(summary.Currencies) != 1 {
		t.Fatalf("member summary currencies: got %d want 1", len(summary.Currencies))
	}

	base := "/api/v1/trips/" + trip.Slug + "/expenses"
	if code, body := doRequest(t, "POST", base, bob.Token, map[string]any{
		"title":        "member edit",
		"amountCents":  100,
		"paidByUserId": bob.ID.String(),
		"splits":       []map[string]any{{"userId": bob.ID.String(), "shareCents": 100}},
	}); code != 403 {
		t.Fatalf("member create: status %d (want 403): %s", code, body)
	}
	if code, body := doRequest(t, "PATCH", base+"/"+exp.ID, bob.Token,
		map[string]any{"title": "tampered"}); code != 403 {
		t.Fatalf("member update: status %d (want 403): %s", code, body)
	}
	if code, body := doRequest(t, "DELETE", base+"/"+exp.ID, bob.Token, nil); code != 403 {
		t.Fatalf("member delete: status %d (want 403): %s", code, body)
	}

	managed := createExpense(t, bea, trip.Slug, map[string]any{
		"title":        "Taxi",
		"amountCents":  4000,
		"paidByUserId": bea.ID.String(),
		"splits": []map[string]any{
			{"userId": bea.ID.String(), "shareCents": 4000},
		},
	})
	var updated expenseResponse
	mustDo(t, "PATCH", base+"/"+managed.ID, bea.Token,
		map[string]any{"title": "Airport taxi"}, 200, &updated)
	if updated.Title != "Airport taxi" {
		t.Fatalf("budget manager update title: got %q", updated.Title)
	}
	mustDo(t, "DELETE", base+"/"+managed.ID, bea.Token, nil, 204, nil)
}

// TestBudgetNonMemberCannotAccess
//
// A non-member of a private trip is forbidden from every budget endpoint:
// list, create, update, delete, and summary.
func TestBudgetNonMemberCannotAccess(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	carol := registerUser(t, "carol")
	trip := createTrip(t, alice, "Italy")

	exp := createExpense(t, alice, trip.Slug, map[string]any{
		"title":        "Hotel",
		"amountCents":  10000,
		"paidByUserId": alice.ID.String(),
		"splits": []map[string]any{
			{"userId": alice.ID.String(), "shareCents": 10000},
		},
	})

	base := "/api/v1/trips/" + trip.Slug
	if code, body := doRequest(t, "GET", base+"/expenses", carol.Token, nil); code != 403 {
		t.Fatalf("non-member list: status %d (want 403): %s", code, body)
	}
	if code, body := doRequest(t, "POST", base+"/expenses", carol.Token, map[string]any{
		"title":        "hack",
		"amountCents":  100,
		"paidByUserId": carol.ID.String(),
		"splits":       []map[string]any{{"userId": carol.ID.String(), "shareCents": 100}},
	}); code != 403 {
		t.Fatalf("non-member create: status %d (want 403): %s", code, body)
	}
	if code, body := doRequest(t, "PATCH", base+"/expenses/"+exp.ID, carol.Token,
		map[string]any{"title": "tampered"}); code != 403 {
		t.Fatalf("non-member update: status %d (want 403): %s", code, body)
	}
	if code, body := doRequest(t, "DELETE", base+"/expenses/"+exp.ID, carol.Token, nil); code != 403 {
		t.Fatalf("non-member delete: status %d (want 403): %s", code, body)
	}
	if code, body := doRequest(t, "GET", base+"/budget/summary", carol.Token, nil); code != 403 {
		t.Fatalf("non-member summary: status %d (want 403): %s", code, body)
	}
}

// TestBudgetPaidByMustBeMember
//
// The payer must be a current trip member: an expense whose paidByUserId
// is a registered user outside the trip is rejected 422.
func TestBudgetPaidByMustBeMember(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	carol := registerUser(t, "carol") // registered, not on the trip
	trip := createTrip(t, alice, "Italy")

	code, body := doRequest(t, "POST", "/api/v1/trips/"+trip.Slug+"/expenses", alice.Token,
		map[string]any{
			"title":        "Taxi",
			"amountCents":  4000,
			"paidByUserId": carol.ID.String(),
			"splits":       []map[string]any{{"userId": alice.ID.String(), "shareCents": 4000}},
		})
	if code != 422 || !containsCode(body, "paid_by_not_member") {
		t.Fatalf("paid-by non-member: status %d body %s", code, body)
	}
}

// TestBudgetSplitUserMustBeMember
//
// Every split user must be a current trip member: a split addressed to a
// non-member is rejected 422.
func TestBudgetSplitUserMustBeMember(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	carol := registerUser(t, "carol") // registered, not on the trip
	trip := createTrip(t, alice, "Italy")

	code, body := doRequest(t, "POST", "/api/v1/trips/"+trip.Slug+"/expenses", alice.Token,
		map[string]any{
			"title":        "Museum",
			"amountCents":  12000,
			"paidByUserId": alice.ID.String(),
			"splits": []map[string]any{
				{"userId": alice.ID.String(), "shareCents": 6000},
				{"userId": carol.ID.String(), "shareCents": 6000},
			},
		})
	if code != 422 || !containsCode(body, "split_user_not_member") {
		t.Fatalf("split user non-member: status %d body %s", code, body)
	}
}

// TestBudgetSplitSumMustEqualAmount
//
// The split shares must sum exactly to the expense amount; a short sum is
// rejected 400.
func TestBudgetSplitSumMustEqualAmount(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	bob := registerUser(t, "bob")
	friendsBecome(t, alice, bob)
	trip := createTrip(t, alice, "Italy")
	inviteAndJoin(t, alice, bob, trip.Slug)

	code, body := doRequest(t, "POST", "/api/v1/trips/"+trip.Slug+"/expenses", alice.Token,
		map[string]any{
			"title":        "Dinner",
			"amountCents":  10000,
			"paidByUserId": alice.ID.String(),
			"splits": []map[string]any{
				{"userId": alice.ID.String(), "shareCents": 5000},
				{"userId": bob.ID.String(), "shareCents": 4000}, // 9000 != 10000
			},
		})
	if code != 400 || !containsCode(body, "split_sum_mismatch") {
		t.Fatalf("split sum mismatch: status %d body %s", code, body)
	}
}

// TestBudgetUpdateReplacesSplits
//
// A PATCH that supplies splits replaces the existing set wholesale (the
// (expense_id, user_id) uniqueness means a non-atomic replace would error).
// Omitting splits keeps them — and an amount change that the kept splits no
// longer cover is rejected.
func TestBudgetUpdateReplacesSplits(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	bob := registerUser(t, "bob")
	friendsBecome(t, alice, bob)
	trip := createTrip(t, alice, "Italy")
	inviteAndJoin(t, alice, bob, trip.Slug)

	exp := createExpense(t, alice, trip.Slug, map[string]any{
		"title":        "Hotel",
		"amountCents":  60000,
		"paidByUserId": alice.ID.String(),
		"splits": []map[string]any{
			{"userId": alice.ID.String(), "shareCents": 30000},
			{"userId": bob.ID.String(), "shareCents": 30000},
		},
	})

	// Replace the splits (same amount, different shares).
	var updated expenseResponse
	mustDo(t, "PATCH", "/api/v1/trips/"+trip.Slug+"/expenses/"+exp.ID, alice.Token,
		map[string]any{
			"splits": []map[string]any{
				{"userId": alice.ID.String(), "shareCents": 40000},
				{"userId": bob.ID.String(), "shareCents": 20000},
			},
		}, 200, &updated)
	if len(updated.Splits) != 2 {
		t.Fatalf("after replace: %d splits want 2", len(updated.Splits))
	}
	if got := splitShare(t, updated, alice.ID.String()); got != 40000 {
		t.Fatalf("alice share after replace: got %d want 40000", got)
	}
	if got := splitShare(t, updated, bob.ID.String()); got != 20000 {
		t.Fatalf("bob share after replace: got %d want 20000", got)
	}

	// Change the amount with splits omitted: the kept splits still sum to
	// 60000, so the new 99999 amount is rejected.
	code, body := doRequest(t, "PATCH", "/api/v1/trips/"+trip.Slug+"/expenses/"+exp.ID,
		alice.Token, map[string]any{"amountCents": 99999})
	if code != 400 || !containsCode(body, "split_sum_mismatch") {
		t.Fatalf("amount change without splits: status %d body %s", code, body)
	}

	// Change the amount and supply matching splits in the same PATCH: ok.
	mustDo(t, "PATCH", "/api/v1/trips/"+trip.Slug+"/expenses/"+exp.ID, alice.Token,
		map[string]any{
			"amountCents": 80000,
			"splits": []map[string]any{
				{"userId": alice.ID.String(), "shareCents": 50000},
				{"userId": bob.ID.String(), "shareCents": 30000},
			},
		}, 200, &updated)
	if updated.AmountCents != 80000 {
		t.Fatalf("amount after matched update: got %d want 80000", updated.AmountCents)
	}
	if got := splitShare(t, updated, alice.ID.String()) + splitShare(t, updated, bob.ID.String()); got != 80000 {
		t.Fatalf("split shares after matched update: got %d want 80000", got)
	}
}

// TestBudgetSummaryBalancesAndSettlements
//
// The summary groups by currency and computes paid / share / balance per
// member, then a greedy settlement that nets every balance to zero.
func TestBudgetSummaryBalancesAndSettlements(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	bob := registerUser(t, "bob")
	friendsBecome(t, alice, bob)
	trip := createTrip(t, alice, "Italy")
	inviteAndJoin(t, alice, bob, trip.Slug)

	// Hotel 600.00 paid by alice, split 300/300.
	createExpense(t, alice, trip.Slug, map[string]any{
		"title":        "Hotel",
		"amountCents":  60000,
		"paidByUserId": alice.ID.String(),
		"splits": []map[string]any{
			{"userId": alice.ID.String(), "shareCents": 30000},
			{"userId": bob.ID.String(), "shareCents": 30000},
		},
	})
	// Taxi 40.00 paid by bob, split 20/20.
	createExpense(t, bob, trip.Slug, map[string]any{
		"title":        "Taxi",
		"amountCents":  4000,
		"paidByUserId": bob.ID.String(),
		"splits": []map[string]any{
			{"userId": alice.ID.String(), "shareCents": 2000},
			{"userId": bob.ID.String(), "shareCents": 2000},
		},
	})

	summary := budgetSummary(t, alice, trip.Slug)
	if len(summary.Currencies) != 1 {
		t.Fatalf("currencies: got %d want 1", len(summary.Currencies))
	}
	cur := summary.Currencies[0]
	if cur.Currency != "EUR" {
		t.Fatalf("currency: got %q want EUR", cur.Currency)
	}
	if cur.TotalCents != 64000 {
		t.Fatalf("totalCents: got %d want 64000", cur.TotalCents)
	}

	aliceBal := findBalance(t, cur.Members, alice.ID.String())
	bobBal := findBalance(t, cur.Members, bob.ID.String())
	if aliceBal.PaidCents != 60000 || aliceBal.ShareCents != 32000 || aliceBal.BalanceCents != 28000 {
		t.Fatalf("alice balance: %+v want paid=60000 share=32000 balance=28000", aliceBal)
	}
	if bobBal.PaidCents != 4000 || bobBal.ShareCents != 32000 || bobBal.BalanceCents != -28000 {
		t.Fatalf("bob balance: %+v want paid=4000 share=32000 balance=-28000", bobBal)
	}

	// One settlement: bob (debtor) pays alice (creditor) 280.00.
	if len(cur.Settlements) != 1 {
		t.Fatalf("settlements: got %d want 1", len(cur.Settlements))
	}
	s := cur.Settlements[0]
	if s.DebtorUserID != bob.ID.String() || s.CreditorUserID != alice.ID.String() || s.AmountCents != 28000 {
		t.Fatalf("settlement: %+v want bob->alice 28000", s)
	}
}
