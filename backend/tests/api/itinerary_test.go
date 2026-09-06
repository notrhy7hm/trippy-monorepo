package api

import (
	"encoding/json"
	"strconv"
	"sync"
	"testing"
)

// TestItineraryTitleOnlyCreateDefaultsDayZero
//
// Title-only create returns 201 with dayIndex=0 (server default).
// Also protects against the pgx "cannot find encode plan" advisory-
// lock bug from earlier in M2 — a regression of that bug would 500
// here instead of returning a clean item.
func TestItineraryTitleOnlyCreateDefaultsDayZero(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	trip := createTrip(t, alice, "Italy")

	item := createItinerary(t, alice, trip.Slug, map[string]any{
		"title": "Visit Colosseum",
	})
	if item.Title != "Visit Colosseum" {
		t.Fatalf("title: got %q want %q", item.Title, "Visit Colosseum")
	}
	if item.DayIndex != 0 {
		t.Fatalf("dayIndex: got %d want 0", item.DayIndex)
	}
	if item.Position <= 0 {
		t.Fatalf("position: got %d want > 0", item.Position)
	}
}

// TestItineraryConcurrentCreatePositions
//
// Race-tests the itinerary bucket lock: N parallel title-only POSTs
// (all land in Day 0) must produce distinct positive positions.
func TestItineraryConcurrentCreatePositions(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	trip := createTrip(t, alice, "Italy")

	const N = 10
	var wg sync.WaitGroup
	results := make([]itineraryResponse, N)
	errs := make([]string, N)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			code, raw, err := rawDo("POST", "/api/v1/trips/"+trip.Slug+"/itinerary", alice.Token,
				map[string]any{"title": "item " + strconv.Itoa(idx)})
			if err != nil {
				errs[idx] = err.Error()
				return
			}
			if code != 201 {
				errs[idx] = "status " + strconv.Itoa(code) + ": " + string(raw)
				return
			}
			if err := json.Unmarshal(raw, &results[idx]); err != nil {
				errs[idx] = "decode: " + err.Error()
			}
		}(i)
	}
	wg.Wait()

	seen := make(map[int]int, N)
	for i, item := range results {
		if errs[i] != "" {
			t.Errorf("item %d: %s", i, errs[i])
			continue
		}
		if item.DayIndex != 0 {
			t.Errorf("item %d: dayIndex %d (want 0)", i, item.DayIndex)
		}
		if item.Position <= 0 {
			t.Errorf("item %d: non-positive position %d", i, item.Position)
		}
		if other, dup := seen[item.Position]; dup {
			t.Errorf("duplicate position %d on items %d and %d", item.Position, other, i)
		}
		seen[item.Position] = i
	}
	if t.Failed() {
		return
	}

	// All N concurrent creates must also be visible via the list
	// endpoint — no item silently lost to the bucket-lock path.
	all := listItinerary(t, alice, trip.Slug)
	if len(all) != N {
		t.Fatalf("list size: got %d want %d", len(all), N)
	}
}

// TestItineraryDayRebucket
//
// Moving an item to a different dayIndex without an explicit position
// reassigns position to the end of the destination day.
func TestItineraryDayRebucket(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	trip := createTrip(t, alice, "Italy")

	day0A := createItinerary(t, alice, trip.Slug, map[string]any{"title": "D0 A"})
	_ = createItinerary(t, alice, trip.Slug, map[string]any{"title": "D0 B"})
	day1C := createItinerary(t, alice, trip.Slug, map[string]any{
		"title":    "D1 C",
		"dayIndex": 1,
	})

	var moved itineraryResponse
	mustDo(t, "PATCH", "/api/v1/trips/"+trip.Slug+"/itinerary/"+day0A.ID, alice.Token,
		map[string]any{"dayIndex": 1}, 200, &moved,
	)
	if moved.DayIndex != 1 {
		t.Fatalf("dayIndex: got %d want 1", moved.DayIndex)
	}
	if moved.Position <= day1C.Position {
		t.Fatalf("moved.Position=%d should be > existing day1 max=%d",
			moved.Position, day1C.Position)
	}
}

// TestItineraryPartialTimeRangeUpdateValidation
//
// Final startsAt/endsAt range validation lives inside the repo's
// SELECT ... FOR UPDATE transaction. A PATCH that only updates one of
// the two fields must reject when the resulting (current, new) pair is
// inverted, even though the request payload alone looks fine.
func TestItineraryPartialTimeRangeUpdateValidation(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	trip := createTrip(t, alice, "Italy")

	a := createItinerary(t, alice, trip.Slug, map[string]any{
		"title":    "morning",
		"startsAt": "10:00",
		"endsAt":   "12:00",
	})

	// Patch only startsAt to 13:00 -> final (13:00, 12:00) is inverted.
	code, body := doRequest(t, "PATCH",
		"/api/v1/trips/"+trip.Slug+"/itinerary/"+a.ID, alice.Token,
		map[string]any{"startsAt": "13:00"},
	)
	if code != 400 {
		t.Fatalf("startsAt only: status %d (want 400): %s", code, body)
	}
	if !containsCode(body, "invalid_time_range") {
		t.Fatalf("startsAt only: want invalid_time_range, got %s", body)
	}

	b := createItinerary(t, alice, trip.Slug, map[string]any{
		"title":    "evening",
		"startsAt": "18:00",
		"endsAt":   "20:00",
	})

	// Patch only endsAt earlier than the locked startsAt.
	code, body = doRequest(t, "PATCH",
		"/api/v1/trips/"+trip.Slug+"/itinerary/"+b.ID, alice.Token,
		map[string]any{"endsAt": "17:00"},
	)
	if code != 400 {
		t.Fatalf("endsAt only: status %d (want 400): %s", code, body)
	}
	if !containsCode(body, "invalid_time_range") {
		t.Fatalf("endsAt only: want invalid_time_range, got %s", body)
	}
}

// TestItineraryNonMemberAccess
//
// A non-member of a private trip cannot list / create / update /
// delete itinerary items. Verified end-to-end through the HTTP layer.
func TestItineraryNonMemberAccess(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	carol := registerUser(t, "carol")
	trip := createTrip(t, alice, "Italy")

	// Alice plants one item so Carol has a real id to try PATCH/DELETE
	// against.
	item := createItinerary(t, alice, trip.Slug, map[string]any{"title": "Arrive"})

	if code, body := doRequest(t, "GET", "/api/v1/trips/"+trip.Slug+"/itinerary", carol.Token, nil); code != 403 {
		t.Fatalf("non-member list: status %d (want 403): %s", code, body)
	}
	if code, body := doRequest(t, "POST", "/api/v1/trips/"+trip.Slug+"/itinerary", carol.Token,
		map[string]any{"title": "hack"}); code != 403 {
		t.Fatalf("non-member create: status %d (want 403): %s", code, body)
	}
	if code, body := doRequest(t, "PATCH",
		"/api/v1/trips/"+trip.Slug+"/itinerary/"+item.ID, carol.Token,
		map[string]any{"title": "tampered"},
	); code != 403 {
		t.Fatalf("non-member update: status %d (want 403): %s", code, body)
	}
	if code, body := doRequest(t, "DELETE",
		"/api/v1/trips/"+trip.Slug+"/itinerary/"+item.ID, carol.Token, nil,
	); code != 403 {
		t.Fatalf("non-member delete: status %d (want 403): %s", code, body)
	}
}

// TestItineraryRoleMutationAccess
//
// Plain members can read itinerary items but cannot create, update, or delete
// them. The planner role can mutate itinerary data.
func TestItineraryRoleMutationAccess(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	bob := registerUser(t, "bob")
	pat := registerUser(t, "pat")
	friendsBecome(t, alice, bob)
	friendsBecome(t, alice, pat)
	trip := createTrip(t, alice, "Italy")
	inviteAndJoin(t, alice, bob, trip.Slug)
	inviteAndJoinAs(t, alice, pat, trip.Slug, "planner")

	item := createItinerary(t, alice, trip.Slug, map[string]any{"title": "Arrive"})

	if all := listItinerary(t, bob, trip.Slug); len(all) != 1 {
		t.Fatalf("member list size: got %d want 1", len(all))
	}
	if code, body := doRequest(t, "POST", "/api/v1/trips/"+trip.Slug+"/itinerary", bob.Token,
		map[string]any{"title": "member edit"}); code != 403 {
		t.Fatalf("member create: status %d (want 403): %s", code, body)
	}
	if code, body := doRequest(t, "PATCH", "/api/v1/trips/"+trip.Slug+"/itinerary/"+item.ID, bob.Token,
		map[string]any{"title": "tampered"}); code != 403 {
		t.Fatalf("member update: status %d (want 403): %s", code, body)
	}
	if code, body := doRequest(t, "DELETE", "/api/v1/trips/"+trip.Slug+"/itinerary/"+item.ID, bob.Token, nil); code != 403 {
		t.Fatalf("member delete: status %d (want 403): %s", code, body)
	}

	planned := createItinerary(t, pat, trip.Slug, map[string]any{"title": "Planner item"})
	var updated itineraryResponse
	mustDo(t, "PATCH", "/api/v1/trips/"+trip.Slug+"/itinerary/"+planned.ID, pat.Token,
		map[string]any{"title": "Updated planner item"}, 200, &updated)
	if updated.Title != "Updated planner item" {
		t.Fatalf("planner update title: got %q", updated.Title)
	}
	mustDo(t, "DELETE", "/api/v1/trips/"+trip.Slug+"/itinerary/"+planned.ID, pat.Token, nil, 204, nil)
}

// TestItineraryTripDateRangeValidation
//
// Backend enforces date/day vs trip range (added in the M2
// stabilization sweep). Frontend hiding controls is not the boundary.
func TestItineraryTripDateRangeValidation(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	trip := createTripWithDates(t, alice, "Italy",
		"2026-06-15T00:00:00Z", "2026-06-20T00:00:00Z")

	// Day 2 == 2026-06-17 — consistent, in range.
	ok := createItinerary(t, alice, trip.Slug, map[string]any{
		"title":    "Visit Colosseum",
		"dayIndex": 2,
		"date":     "2026-06-17",
	})
	if ok.DayIndex != 2 {
		t.Fatalf("expected dayIndex=2 got %d", ok.DayIndex)
	}

	// Date before startsOn.
	code, body := doRequest(t, "POST", "/api/v1/trips/"+trip.Slug+"/itinerary", alice.Token,
		map[string]any{"title": "early", "dayIndex": 0, "date": "2026-06-10"})
	if code != 400 || !containsCode(body, "itinerary_date_out_of_range") {
		t.Fatalf("date before startsOn: status %d body %s", code, body)
	}

	// Date after endsOn.
	code, body = doRequest(t, "POST", "/api/v1/trips/"+trip.Slug+"/itinerary", alice.Token,
		map[string]any{"title": "late", "dayIndex": 0, "date": "2026-06-25"})
	if code != 400 || !containsCode(body, "itinerary_date_out_of_range") {
		t.Fatalf("date after endsOn: status %d body %s", code, body)
	}

	// dayIndex > trip span (trip is 6 days -> max dayIndex is 5).
	code, body = doRequest(t, "POST", "/api/v1/trips/"+trip.Slug+"/itinerary", alice.Token,
		map[string]any{"title": "too far", "dayIndex": 6})
	if code != 400 || !containsCode(body, "itinerary_day_out_of_range") {
		t.Fatalf("dayIndex out of range: status %d body %s", code, body)
	}

	// Date / dayIndex disagree.
	code, body = doRequest(t, "POST", "/api/v1/trips/"+trip.Slug+"/itinerary", alice.Token,
		map[string]any{"title": "mismatch", "dayIndex": 5, "date": "2026-06-17"})
	if code != 400 || !containsCode(body, "itinerary_date_day_mismatch") {
		t.Fatalf("date/day mismatch: status %d body %s", code, body)
	}
}

// containsCode scans a JSON error body for the api.ErrorBody "code"
// field. Avoids pulling the api package in (cyclic with test wiring).
func containsCode(body []byte, want string) bool {
	var env struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return false
	}
	return env.Code == want
}
