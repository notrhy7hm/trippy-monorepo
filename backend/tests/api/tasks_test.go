package api

import (
	"encoding/json"
	"strconv"
	"sync"
	"testing"
)

// TestPlanningTaskTitleAndMemberAccess
//
// Member can create / list tasks with a minimal payload (title only,
// since the API now defaults status/priority server-side). A
// non-member of a private trip gets 403 on both list and create.
func TestPlanningTaskTitleAndMemberAccess(t *testing.T) {
	cleanDB(t)

	alice := registerUser(t, "alice")
	_ = registerUser(t, "bob") // unused here but keeps actor naming consistent across tests
	carol := registerUser(t, "carol")

	trip := createTrip(t, alice, "Italy")

	// Alice (owner = member) creates a task with just a title.
	task := createTask(t, alice, trip.Slug, map[string]any{
		"title": "Book hotel",
	})
	if task.Title != "Book hotel" {
		t.Fatalf("title: got %q want %q", task.Title, "Book hotel")
	}
	if task.Status != "todo" {
		t.Fatalf("default status: got %q want todo", task.Status)
	}
	if task.Priority != "normal" {
		t.Fatalf("default priority: got %q want normal", task.Priority)
	}
	if task.CreatedBy.Username != alice.Username {
		t.Fatalf("createdBy: got %q want %q", task.CreatedBy.Username, alice.Username)
	}

	// Non-member Carol gets forbidden on list and create.
	if code, body := doRequest(t, "GET", "/api/v1/trips/"+trip.Slug+"/tasks", carol.Token, nil); code != 403 {
		t.Fatalf("non-member list: status %d (want 403): %s", code, body)
	}
	if code, body := doRequest(t, "POST", "/api/v1/trips/"+trip.Slug+"/tasks", carol.Token,
		map[string]any{"title": "hack"}); code != 403 {
		t.Fatalf("non-member create: status %d (want 403): %s", code, body)
	}
}

// TestPlanningTaskConcurrentCreatePositions
//
// Race-tests the advisory-lock + max(position) path. N parallel POSTs
// in the same (trip, status) bucket must all succeed and produce
// distinct positive positions.
func TestPlanningTaskConcurrentCreatePositions(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	trip := createTrip(t, alice, "Italy")

	const N = 10
	var wg sync.WaitGroup
	results := make([]taskResponse, N)
	errs := make([]string, N)

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			code, raw, err := rawDo("POST", "/api/v1/trips/"+trip.Slug+"/tasks", alice.Token,
				map[string]any{"title": "task " + strconv.Itoa(idx)})
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
	for i, task := range results {
		if errs[i] != "" {
			t.Errorf("task %d: %s", i, errs[i])
			continue
		}
		if task.Position <= 0 {
			t.Errorf("task %d: non-positive position %d", i, task.Position)
		}
		if other, dup := seen[task.Position]; dup {
			t.Errorf("duplicate position %d on tasks %d and %d", task.Position, other, i)
		}
		seen[task.Position] = i
	}
	if t.Failed() {
		return
	}

	// Sanity: list endpoint returns all N and order is deterministic
	// (the comparator's secondary keys handle ties even if positions
	// happened to repeat in a different scenario — they shouldn't here).
	all := listTasks(t, alice, trip.Slug)
	if len(all) != N {
		t.Fatalf("list size: got %d want %d", len(all), N)
	}
	for i := 1; i < len(all); i++ {
		if all[i].Position < all[i-1].Position {
			t.Fatalf("list not ordered by position: %v", all)
		}
	}
}

// TestPlanningTaskStatusRebucket
//
// Moving a task to a new status without an explicit position must
// reassign position to the end of the destination column. Verified by
// asserting the moved task's new position is greater than every other
// position currently in the destination bucket.
func TestPlanningTaskStatusRebucket(t *testing.T) {
	cleanDB(t)
	alice := registerUser(t, "alice")
	trip := createTrip(t, alice, "Italy")

	todoA := createTask(t, alice, trip.Slug, map[string]any{"title": "Todo A"})
	_ = createTask(t, alice, trip.Slug, map[string]any{"title": "Todo B"})
	inProgC := createTask(t, alice, trip.Slug, map[string]any{
		"title":  "InProgress C",
		"status": "in_progress",
	})

	var moved taskResponse
	mustDo(t, "PATCH", "/api/v1/trips/"+trip.Slug+"/tasks/"+todoA.ID, alice.Token,
		map[string]any{"status": "in_progress"}, 200, &moved,
	)
	if moved.Status != "in_progress" {
		t.Fatalf("status: got %q want in_progress", moved.Status)
	}
	if moved.Position <= inProgC.Position {
		t.Fatalf("moved.Position=%d should be > existing in_progress max=%d",
			moved.Position, inProgC.Position)
	}

	// And that the destination bucket's order is consistent on list.
	all := listTasks(t, alice, trip.Slug)
	var lastInProgressPos int
	for _, task := range all {
		if task.Status == "in_progress" && task.Position > lastInProgressPos {
			lastInProgressPos = task.Position
		}
	}
	if lastInProgressPos != moved.Position {
		t.Fatalf("moved should be at end of in_progress: lastPos=%d moved=%d",
			lastInProgressPos, moved.Position)
	}
}
