package api

import (
	"net/url"
	"testing"

	"github.com/google/uuid"
)

// actor represents an authenticated user during a test.
type actor struct {
	ID       uuid.UUID
	Username string
	Email    string
	Token    string
}

// authResponse mirrors the auth handler's success body.
type authResponse struct {
	Token string `json:"token"`
	User  struct {
		Username string `json:"username"`
	} `json:"user"`
}

// tripResponse — subset of trips.Trip that tests assert against.
type tripResponse struct {
	Slug       string  `json:"slug"`
	Title      string  `json:"title"`
	Visibility string  `json:"visibility"`
	StartsOn   *string `json:"startsOn,omitempty"`
	EndsOn     *string `json:"endsOn,omitempty"`
}

// inviteResponse — subset of trips.Invite returned by POST /trips/{slug}/invites.
type inviteResponse struct {
	Token string `json:"token"`
	Role  string `json:"role"`
}

// taskResponse — subset of planning.Task.
type taskResponse struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Priority  string `json:"priority"`
	Position  int    `json:"position"`
	CreatedBy struct {
		Username string `json:"username"`
	} `json:"createdBy"`
}

// itineraryResponse — subset of planning.ItineraryItem.
type itineraryResponse struct {
	ID       string  `json:"id"`
	DayIndex int     `json:"dayIndex"`
	Date     *string `json:"date,omitempty"`
	Title    string  `json:"title"`
	Notes    string  `json:"notes"`
	StartsAt *string `json:"startsAt,omitempty"`
	EndsAt   *string `json:"endsAt,omitempty"`
	Position int     `json:"position"`
}

// ---------------------------------------------------------------------------
// auth + actors
// ---------------------------------------------------------------------------

// registerUser creates a fresh account and returns an authenticated
// actor. Tests should call cleanDB first; usernames are arbitrary but
// must be unique across the test (the cleanDB call guarantees that).
func registerUser(t *testing.T, username string) actor {
	t.Helper()
	email := username + "@test.dev"
	var resp authResponse
	mustDo(t, "POST", "/api/v1/auth/register", "", map[string]string{
		"email":    email,
		"username": username,
		"password": "hunter2!!",
	}, 201, &resp)
	if resp.Token == "" {
		t.Fatalf("register %s: empty token", username)
	}
	// The API never exposes user UUIDs, but budget expenses reference
	// users by id, so tests resolve it straight from the test DB.
	var id uuid.UUID
	if err := testDB.Get(&id, "SELECT id FROM users WHERE username = $1", username); err != nil {
		t.Fatalf("register %s: look up user id: %v", username, err)
	}
	return actor{ID: id, Username: username, Email: email, Token: resp.Token}
}

// ---------------------------------------------------------------------------
// trips + friends + invites
// ---------------------------------------------------------------------------

// createTrip creates a private trip with no dates set.
func createTrip(t *testing.T, owner actor, title string) tripResponse {
	t.Helper()
	return createTripWithVisibility(t, owner, title, "private")
}

func createTripWithVisibility(t *testing.T, owner actor, title, visibility string) tripResponse {
	t.Helper()
	var trip tripResponse
	mustDo(t, "POST", "/api/v1/trips", owner.Token, map[string]any{
		"title":      title,
		"visibility": visibility,
	}, 201, &trip)
	return trip
}

// createTripWithDates is the date-aware variant used by the trip-range
// validation test.
func createTripWithDates(t *testing.T, owner actor, title, startsOn, endsOn string) tripResponse {
	t.Helper()
	var trip tripResponse
	mustDo(t, "POST", "/api/v1/trips", owner.Token, map[string]any{
		"title":      title,
		"visibility": "private",
		"startsOn":   startsOn,
		"endsOn":     endsOn,
	}, 201, &trip)
	return trip
}

// friendsBecome wires Alice <-> Bob as friends.
func friendsBecome(t *testing.T, a, b actor) {
	t.Helper()
	mustDo(t, "POST", "/api/v1/friend-requests", a.Token,
		map[string]string{"username": b.Username},
		201, nil,
	)
	mustDo(t, "POST",
		"/api/v1/friend-requests/from/"+url.PathEscape(a.Username)+"/accept",
		b.Token, nil, 200, nil,
	)
}

// inviteAndJoin issues a trip invite from owner to invitee and accepts
// it on the invitee's side. Both parties must already be friends.
func inviteAndJoin(t *testing.T, owner, invitee actor, slug string) {
	t.Helper()
	inviteAndJoinAs(t, owner, invitee, slug, "member")
}

func inviteAndJoinAs(t *testing.T, owner, invitee actor, slug, role string) {
	t.Helper()
	var inv inviteResponse
	mustDo(t, "POST", "/api/v1/trips/"+slug+"/invites", owner.Token, map[string]string{
		"identifier": invitee.Username,
		"role":       role,
	}, 201, &inv)
	mustDo(t, "POST", "/api/v1/invites/"+inv.Token+"/accept", invitee.Token,
		nil, 200, nil,
	)
}

// ---------------------------------------------------------------------------
// tasks
// ---------------------------------------------------------------------------

// createTask POSTs and asserts 201; returns the new task.
func createTask(t *testing.T, a actor, slug string, body any) taskResponse {
	t.Helper()
	var r taskResponse
	mustDo(t, "POST", "/api/v1/trips/"+slug+"/tasks", a.Token, body, 201, &r)
	return r
}

// listTasks does a GET and asserts 200.
func listTasks(t *testing.T, a actor, slug string) []taskResponse {
	t.Helper()
	var r []taskResponse
	mustDo(t, "GET", "/api/v1/trips/"+slug+"/tasks", a.Token, nil, 200, &r)
	return r
}

// ---------------------------------------------------------------------------
// itinerary
// ---------------------------------------------------------------------------

// createItinerary POSTs and asserts 201; returns the new item.
func createItinerary(t *testing.T, a actor, slug string, body any) itineraryResponse {
	t.Helper()
	var r itineraryResponse
	mustDo(t, "POST", "/api/v1/trips/"+slug+"/itinerary", a.Token, body, 201, &r)
	return r
}

// listItinerary does a GET and asserts 200.
func listItinerary(t *testing.T, a actor, slug string) []itineraryResponse {
	t.Helper()
	var r []itineraryResponse
	mustDo(t, "GET", "/api/v1/trips/"+slug+"/itinerary", a.Token, nil, 200, &r)
	return r
}
