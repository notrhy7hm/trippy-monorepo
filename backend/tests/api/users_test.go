package api

import (
	"strings"
	"testing"
)

func TestRegisterValidation(t *testing.T) {
	cleanDB(t)

	tests := []struct {
		name string
		body map[string]any
	}{
		{
			name: "bad email",
			body: map[string]any{
				"email":    "not-an-email",
				"username": "alice",
				"password": "hunter2!!",
			},
		},
		{
			name: "short username",
			body: map[string]any{
				"email":    "alice@test.dev",
				"username": "ab",
				"password": "hunter2!!",
			},
		},
		{
			name: "unsafe username",
			body: map[string]any{
				"email":    "alice@test.dev",
				"username": "bad/name",
				"password": "hunter2!!",
			},
		},
		{
			name: "short password",
			body: map[string]any{
				"email":    "alice@test.dev",
				"username": "alice",
				"password": "short",
			},
		},
	}

	for _, tc := range tests {
		code, body := doRequest(t, "POST", "/api/v1/auth/register", "", tc.body)
		if code != 400 || !containsCode(body, "invalid_input") {
			t.Fatalf("%s: status %d body %s want invalid_input", tc.name, code, body)
		}
	}
}

func TestProfileValidation(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")

	tests := []struct {
		name string
		body map[string]any
	}{
		{
			name: "display name too long",
			body: map[string]any{"displayName": strings.Repeat("a", 81)},
		},
		{
			name: "bio too long",
			body: map[string]any{"bio": strings.Repeat("b", 281)},
		},
		{
			name: "invalid avatar URL",
			body: map[string]any{"avatarUrl": "ftp://example.test/avatar.jpg"},
		},
	}

	for _, tc := range tests {
		code, body := doRequest(t, "PATCH", "/api/v1/users/me", alice.Token, tc.body)
		if code != 400 || !containsCode(body, "invalid_profile") {
			t.Fatalf("%s: status %d body %s want invalid_profile", tc.name, code, body)
		}
	}

	var updated struct {
		DisplayName string `json:"displayName"`
		Bio         string `json:"bio"`
		AvatarURL   string `json:"avatarUrl"`
	}
	mustDo(t, "PATCH", "/api/v1/users/me", alice.Token, map[string]any{
		"displayName": " Alice ",
		"bio":         " Trip planner ",
		"avatarUrl":   "https://example.test/avatar.jpg",
	}, 200, &updated)
	if updated.DisplayName != "Alice" || updated.Bio != "Trip planner" || updated.AvatarURL != "https://example.test/avatar.jpg" {
		t.Fatalf("profile normalization: %+v", updated)
	}
}
