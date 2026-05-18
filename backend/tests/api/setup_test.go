// Package api hosts the DB-backed integration tests for the HTTP API
// surface. Tests live here (rather than in each internal package)
// because they exercise routing, middleware, and cross-module flows that
// only make sense end-to-end.
//
// Each test is sequential against a single test database; do not call
// t.Parallel.
package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/trippyai/trippy/backend/internal/auth"
	"github.com/trippyai/trippy/backend/internal/db"
	"github.com/trippyai/trippy/backend/internal/friends"
	"github.com/trippyai/trippy/backend/internal/httpx"
	"github.com/trippyai/trippy/backend/internal/planning"
	"github.com/trippyai/trippy/backend/internal/trips"
	"github.com/trippyai/trippy/backend/internal/users"
)

var (
	testDB     *sqlx.DB
	testServer *httptest.Server
)

// TestMain refuses to run unless TEST_DATABASE_URL is set AND its
// database name contains "test". Both guards are necessary: env can
// point at a non-test DB by mistake. The schema is expected to be
// migrated separately (see README in this package); cleanDB only
// truncates known tables.
func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		// Don't fail — let the rest of the test suite run elsewhere.
		// Print so CI / a developer notices and wires it up.
		log.Println("TEST_DATABASE_URL not set; skipping DB integration tests")
		os.Exit(0)
	}
	if !looksLikeTestDB(dsn) {
		log.Fatalf("TEST_DATABASE_URL does not look like a test database "+
			"(database name must contain 'test'); got %q", redactDSN(dsn))
	}

	pool, err := db.Open(dsn)
	if err != nil {
		log.Fatalf("open test db: %v", err)
	}
	defer pool.Close()
	testDB = pool

	if err := verifySchema(testDB); err != nil {
		log.Fatalf("test DB schema check failed: %v\n"+
			"Did you run `goose up` against the test DB? See README in this package.", err)
	}

	srv := newTestServer(testDB)
	defer srv.Close()
	testServer = srv

	os.Exit(m.Run())
}

// looksLikeTestDB returns true when the DSN's database-name component
// contains "test" (case-insensitive). Anything else is treated as
// possibly-dev/possibly-prod and refused.
func looksLikeTestDB(dsn string) bool {
	u, err := url.Parse(dsn)
	if err != nil {
		// Fall back to substring match on the full DSN so KV-style DSNs
		// also work; either form must contain "test".
		return strings.Contains(strings.ToLower(dsn), "test")
	}
	name := strings.TrimPrefix(u.Path, "/")
	return strings.Contains(strings.ToLower(name), "test")
}

// redactDSN strips the password from a Postgres URL before logging.
func redactDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil {
		return "(unparseable DSN)"
	}
	if u.User != nil {
		u.User = url.User(u.User.Username())
	}
	return u.String()
}

// verifySchema does a trivial SELECT on the most recently added M2
// table. ErrNoRows means the table exists and is empty (fine); any
// other error means the schema isn't up to date.
func verifySchema(d *sqlx.DB) error {
	var n int
	err := d.Get(&n, "SELECT 1 FROM trip_itinerary_items LIMIT 1")
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return nil
}

// newTestServer constructs the same wiring main.go does, but with a
// fixed JWT secret + TTL suitable for tests. The httptest server is
// bound to a local port and torn down in TestMain.
func newTestServer(pool *sqlx.DB) *httptest.Server {
	userSvc := users.NewService(users.NewRepo(pool))
	// 32-byte secret satisfies the >=16 guard from config.Load (we
	// bypass config.Load entirely for tests).
	authSvc := auth.NewService(
		userSvc,
		[]byte("test-jwt-secret-please-do-not-use!"),
		24*time.Hour,
	)
	friendsSvc := friends.NewService(friends.NewRepo(pool), userSvc)
	tripSvc := trips.NewService(trips.NewRepo(pool), userSvc, friendsSvc)
	planningSvc := planning.NewService(
		planning.NewRepo(pool),
		planning.NewItineraryRepo(pool),
		userSvc,
		tripSvc,
	)
	handler := httpx.NewRouter(httpx.Deps{
		Auth:     authSvc,
		Users:    userSvc,
		Friends:  friendsSvc,
		Trips:    tripSvc,
		Planning: planningSvc,
	})
	return httptest.NewServer(handler)
}

// cleanDB truncates every user-data table the API can touch. Called at
// the start of each test for isolation. RESTART IDENTITY CASCADE is
// safe because all FKs point inward (no external references).
func cleanDB(t *testing.T) {
	t.Helper()
	const stmt = `
		TRUNCATE TABLE
			agent_action_proposals,
			agent_messages,
			trip_agents,
			trip_itinerary_items,
			trip_tasks,
			trip_favorites,
			trip_invites,
			trip_members,
			audit_log,
			friend_requests,
			friendships,
			user_profiles,
			trips,
			users
		RESTART IDENTITY CASCADE
	`
	if _, err := testDB.Exec(stmt); err != nil {
		t.Fatalf("clean db: %v", err)
	}
}

// ---------------------------------------------------------------------------
// low-level HTTP helpers — t-aware (must run on the test goroutine) and
// raw (safe inside goroutines because they never call t.Fatalf).
// ---------------------------------------------------------------------------

// rawDo is safe to call from any goroutine. It performs the HTTP round
// trip and returns (status, body, error) without touching *testing.T.
// Use this in concurrent tests; use doRequest elsewhere.
func rawDo(method, path, token string, body any) (int, []byte, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, testServer.URL+path, reqBody)
	if err != nil {
		return 0, nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read body: %w", err)
	}
	return resp.StatusCode, raw, nil
}

// doRequest is the test-goroutine version of rawDo: it fails the test
// on transport errors.
func doRequest(t *testing.T, method, path, token string, body any) (int, []byte) {
	t.Helper()
	code, raw, err := rawDo(method, path, token, body)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return code, raw
}

// mustDo asserts a specific status code and decodes the body into out
// when non-nil. Use this in straight-line happy paths.
func mustDo(t *testing.T, method, path, token string, body any, wantStatus int, out any) {
	t.Helper()
	code, raw := doRequest(t, method, path, token, body)
	if code != wantStatus {
		t.Fatalf("%s %s: status %d (want %d): %s", method, path, code, wantStatus, raw)
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("decode response: %v: %s", err, raw)
		}
	}
}
