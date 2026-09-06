package api

import "testing"

type memberResponse struct {
	UserID      string   `json:"userId"`
	Username    string   `json:"username"`
	DisplayName string   `json:"displayName"`
	Role        string   `json:"role"`
	Tags        []string `json:"tags"`
}

func TestPublicTripReadEndpoint(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	publicTrip := createTripWithVisibility(t, alice, "Public Italy", "public")
	privateTrip := createTrip(t, alice, "Private Italy")

	var got tripResponse
	mustDo(t, "GET", "/api/v1/public/trips/"+publicTrip.Slug, "", nil, 200, &got)
	if got.Slug != publicTrip.Slug || got.Visibility != "public" {
		t.Fatalf("public trip: got %+v want slug=%s visibility=public", got, publicTrip.Slug)
	}

	if code, body := doRequest(t, "GET", "/api/v1/public/trips/"+privateTrip.Slug, "", nil); code != 404 {
		t.Fatalf("private public-read: status %d (want 404): %s", code, body)
	}
}

func TestTripMembersExposeUserIDs(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	trip := createTrip(t, alice, "Italy")

	var members []memberResponse
	mustDo(t, "GET", "/api/v1/trips/"+trip.Slug+"/members", alice.Token, nil, 200, &members)
	if len(members) != 1 {
		t.Fatalf("members: got %d want 1", len(members))
	}
	if members[0].UserID != alice.ID.String() {
		t.Fatalf("member userId: got %q want %q", members[0].UserID, alice.ID)
	}
}

func TestTripCreateValidation(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")

	tests := []struct {
		name string
		body map[string]any
		code string
	}{
		{
			name: "empty title",
			body: map[string]any{"title": "   ", "visibility": "private"},
			code: "invalid_title",
		},
		{
			name: "invalid visibility",
			body: map[string]any{"title": "Italy", "visibility": "secret"},
			code: "invalid_visibility",
		},
		{
			name: "inverted dates",
			body: map[string]any{
				"title":      "Italy",
				"visibility": "private",
				"startsOn":   "2026-06-20T00:00:00Z",
				"endsOn":     "2026-06-15T00:00:00Z",
			},
			code: "invalid_date_range",
		},
	}

	for _, tc := range tests {
		code, body := doRequest(t, "POST", "/api/v1/trips", alice.Token, tc.body)
		if code != 400 || !containsCode(body, tc.code) {
			t.Fatalf("%s: status %d body %s want code %s", tc.name, code, body, tc.code)
		}
	}
}

func TestTripUpdateValidation(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	trip := createTripWithDates(t, alice, "Italy",
		"2026-06-15T00:00:00Z", "2026-06-20T00:00:00Z")

	tests := []struct {
		name string
		body map[string]any
		code string
	}{
		{
			name: "empty title",
			body: map[string]any{"title": ""},
			code: "invalid_title",
		},
		{
			name: "invalid visibility",
			body: map[string]any{"visibility": "secret"},
			code: "invalid_visibility",
		},
		{
			name: "inverted final dates",
			body: map[string]any{"endsOn": "2026-06-10T00:00:00Z"},
			code: "invalid_date_range",
		},
	}

	for _, tc := range tests {
		code, body := doRequest(t, "PATCH", "/api/v1/trips/"+trip.Slug, alice.Token, tc.body)
		if code != 400 || !containsCode(body, tc.code) {
			t.Fatalf("%s: status %d body %s want code %s", tc.name, code, body, tc.code)
		}
	}
}

func TestFriendsTripVisibility(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	bob := registerUser(t, "bob")
	carol := registerUser(t, "carol")
	friendsBecome(t, alice, bob)
	trip := createTripWithVisibility(t, alice, "Friends Italy", "friends")

	var got tripResponse
	mustDo(t, "GET", "/api/v1/trips/"+trip.Slug, bob.Token, nil, 200, &got)
	if got.Slug != trip.Slug {
		t.Fatalf("friend visible slug: got %q want %q", got.Slug, trip.Slug)
	}
	if code, body := doRequest(t, "GET", "/api/v1/trips/"+trip.Slug+"/members", bob.Token, nil); code != 200 {
		t.Fatalf("friend members: status %d (want 200): %s", code, body)
	}
	if code, body := doRequest(t, "GET", "/api/v1/trips/"+trip.Slug, carol.Token, nil); code != 403 {
		t.Fatalf("non-friend visible: status %d (want 403): %s", code, body)
	}
}
