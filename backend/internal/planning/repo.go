package planning

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

var ErrTaskNotFound = errors.New("task not found")

type Repo struct{ db *sqlx.DB }

func NewRepo(db *sqlx.DB) *Repo { return &Repo{db: db} }

// taskRow is the joined shape every read produces. The repo converts it
// to the API-facing Task; api/model never touches sql.Null* fields.
type taskRow struct {
	ID                   uuid.UUID      `db:"id"`
	Title                string         `db:"title"`
	Description          string         `db:"description"`
	Status               string         `db:"status"`
	Priority             string         `db:"priority"`
	Position             int            `db:"position"`
	DueDate              *time.Time     `db:"due_date"`
	CreatedAt            time.Time      `db:"created_at"`
	UpdatedAt            time.Time      `db:"updated_at"`
	AssigneeUsername     sql.NullString `db:"assignee_username"`
	AssigneeDisplayName  sql.NullString `db:"assignee_display_name"`
	CreatedByUsername    string         `db:"created_by_username"`
	CreatedByDisplayName string         `db:"created_by_display_name"`
}

const taskSelectColumns = `
	t.id, t.title, t.description, t.status, t.priority,
	t.position, t.due_date, t.created_at, t.updated_at,
	a.username                                AS assignee_username,
	COALESCE(ap.display_name, a.username)     AS assignee_display_name,
	cb.username                               AS created_by_username,
	COALESCE(cbp.display_name, cb.username)   AS created_by_display_name
`

const taskJoins = `
	FROM trip_tasks t
	LEFT JOIN users a            ON a.id  = t.assignee_user_id
	LEFT JOIN user_profiles ap   ON ap.user_id = a.id
	JOIN      users cb           ON cb.id = t.created_by_user_id
	LEFT JOIN user_profiles cbp  ON cbp.user_id = cb.id
`

// listOrder pins the order required by the spec:
//
//	status (todo, in_progress, done) -> position asc -> created_at asc -> id asc
const listOrder = `
	ORDER BY
		CASE t.status
			WHEN 'todo'        THEN 0
			WHEN 'in_progress' THEN 1
			WHEN 'done'        THEN 2
			ELSE 3
		END,
		t.position ASC,
		t.created_at ASC,
		t.id ASC
`

func rowToTask(r taskRow) Task {
	out := Task{
		ID:          r.ID,
		Title:       r.Title,
		Description: r.Description,
		Status:      Status(r.Status),
		Priority:    Priority(r.Priority),
		Position:    r.Position,
		DueDate:     r.DueDate,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
		CreatedBy: Party{
			Username:    r.CreatedByUsername,
			DisplayName: r.CreatedByDisplayName,
		},
	}
	if r.AssigneeUsername.Valid {
		out.Assignee = &Party{
			Username:    r.AssigneeUsername.String,
			DisplayName: r.AssigneeDisplayName.String,
		}
	}
	return out
}

func rowsToTasks(rows []taskRow) []Task {
	out := make([]Task, 0, len(rows))
	for _, r := range rows {
		out = append(out, rowToTask(r))
	}
	return out
}

// ListTasks returns every task on the trip in the spec's deterministic order.
func (r *Repo) ListTasks(ctx context.Context, tripID uuid.UUID) ([]Task, error) {
	var rows []taskRow
	err := r.db.SelectContext(ctx, &rows, `
		SELECT `+taskSelectColumns+taskJoins+`
		WHERE t.trip_id = $1
	`+listOrder, tripID)
	if err != nil {
		return nil, err
	}
	return rowsToTasks(rows), nil
}

// TaskByIDForTrip returns a single task scoped to its trip; cross-trip ids
// surface as ErrTaskNotFound rather than returning the wrong row.
func (r *Repo) TaskByIDForTrip(ctx context.Context, tripID, taskID uuid.UUID) (Task, error) {
	var row taskRow
	err := r.db.GetContext(ctx, &row, `
		SELECT `+taskSelectColumns+taskJoins+`
		WHERE t.trip_id = $1 AND t.id = $2
	`, tripID, taskID)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}
	if err != nil {
		return Task{}, err
	}
	return rowToTask(row), nil
}

// CreateRow is the repo-shaped insert payload. AssigneeUserID is nullable.
type CreateRow struct {
	TripID          uuid.UUID
	Title           string
	Description     string
	Status          Status
	Priority        Priority
	AssigneeUserID  *uuid.UUID
	CreatedByUserID uuid.UUID
	DueDate         *time.Time
}

// lockBucketSQL takes a Postgres transaction-scoped advisory lock keyed
// by (trip_id, status). hashtextextended produces a stable bigint hash so
// the same bucket maps to the same lock key across connections. The lock
// is released automatically on tx commit/rollback.
const lockBucketSQL = `
	SELECT pg_advisory_xact_lock(
		hashtextextended('trip_tasks_pos:' || $1::text || ':' || $2::text, 0)
	)
`

func lockTaskPositionBucket(ctx context.Context, tx *sqlx.Tx, tripID uuid.UUID, status Status) error {
	_, err := tx.ExecContext(ctx, lockBucketSQL, tripID, status)
	return err
}

func nextPositionInBucket(ctx context.Context, tx *sqlx.Tx, tripID uuid.UUID, status Status) (int, error) {
	var maxPos sql.NullInt64
	if err := tx.GetContext(ctx, &maxPos, `
		SELECT MAX(position) FROM trip_tasks
		WHERE trip_id = $1 AND status = $2
	`, tripID, status); err != nil {
		return 0, err
	}
	if maxPos.Valid {
		return int(maxPos.Int64) + 1000, nil
	}
	return 1000, nil
}

// CreateTask inserts a new task and returns it joined.
//
// Position assignment is concurrency-safe: the tx takes a per-bucket
// advisory lock before reading MAX(position), so two parallel CreateTask
// calls in the same (trip, status) bucket cannot read the same max and
// insert the same position. Tasks in different buckets do not contend.
func (r *Repo) CreateTask(ctx context.Context, in CreateRow) (Task, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Task{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockTaskPositionBucket(ctx, tx, in.TripID, in.Status); err != nil {
		return Task{}, err
	}
	nextPos, err := nextPositionInBucket(ctx, tx, in.TripID, in.Status)
	if err != nil {
		return Task{}, err
	}

	var id uuid.UUID
	if err := tx.QueryRowxContext(ctx, `
		INSERT INTO trip_tasks
			(trip_id, title, description, status, priority,
			 assignee_user_id, created_by_user_id, position, due_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`,
		in.TripID, in.Title, in.Description, in.Status, in.Priority,
		in.AssigneeUserID, in.CreatedByUserID, nextPos, in.DueDate,
	).Scan(&id); err != nil {
		return Task{}, err
	}

	var row taskRow
	if err := tx.GetContext(ctx, &row, `
		SELECT `+taskSelectColumns+taskJoins+`
		WHERE t.trip_id = $1 AND t.id = $2
	`, in.TripID, id); err != nil {
		return Task{}, err
	}

	if err := tx.Commit(); err != nil {
		return Task{}, err
	}
	return rowToTask(row), nil
}

// UpdateRow is the repo-shaped patch payload. All scalar pointer fields
// are tri-state (nil = leave column alone). AssigneeUserID + DueDate use
// an explicit boolean gate because nil also means "set to NULL" for them.
type UpdateRow struct {
	Title       *string
	Description *string
	Status      *Status
	Priority    *Priority
	Position    *int

	// When SetAssignee is true, assignee_user_id is set to AssigneeUserID
	// (which may itself be nil -> SQL NULL). When false, the column is
	// left alone.
	SetAssignee    bool
	AssigneeUserID *uuid.UUID

	// Same convention for due_date.
	SetDueDate bool
	DueDate    *time.Time
}

// UpdateTask applies a partial update inside a single transaction.
//
//  1. SELECT FOR UPDATE locks the task row, reading its current status.
//     Missing row -> ErrTaskNotFound.
//  2. If the request changes status AND does not include an explicit
//     position, the task is rebucketed: a per-bucket advisory lock is
//     taken on the new (trip, status) bucket and position is set to
//     max(position) + 1000 within that bucket. The same lock that
//     serializes CreateTask serializes rebucketing, so two concurrent
//     moves into the same bucket cannot tie.
//  3. Dynamic UPDATE applies all set fields; updated_at = now() is
//     always written.
//  4. Joined Task is re-read inside the tx and returned.
func (r *Repo) UpdateTask(ctx context.Context, tripID, taskID uuid.UUID, in UpdateRow) (Task, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return Task{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var currentStatus Status
	err = tx.QueryRowxContext(ctx, `
		SELECT status FROM trip_tasks
		WHERE trip_id = $1 AND id = $2
		FOR UPDATE
	`, tripID, taskID).Scan(&currentStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}
	if err != nil {
		return Task{}, err
	}

	// Decide whether to rebucket. Only fires when status changes *and*
	// the caller did not pin a position themselves.
	var rebucketedPos *int
	if in.Status != nil && *in.Status != currentStatus && in.Position == nil {
		if err := lockTaskPositionBucket(ctx, tx, tripID, *in.Status); err != nil {
			return Task{}, err
		}
		// The row is still in its old status bucket at this point, so the
		// MAX(position) for the new bucket already excludes it correctly.
		np, err := nextPositionInBucket(ctx, tx, tripID, *in.Status)
		if err != nil {
			return Task{}, err
		}
		rebucketedPos = &np
	}

	// Build SET clause dynamically. trip_id ($1) + task_id ($2) are
	// always the first two args; everything else follows.
	parts := []string{"updated_at = now()"}
	args := []any{tripID, taskID}
	next := 3
	add := func(col string, val any) {
		parts = append(parts, fmt.Sprintf("%s = $%d", col, next))
		args = append(args, val)
		next++
	}

	if in.Title != nil {
		add("title", *in.Title)
	}
	if in.Description != nil {
		add("description", *in.Description)
	}
	if in.Status != nil {
		add("status", *in.Status)
	}
	if in.Priority != nil {
		add("priority", *in.Priority)
	}
	switch {
	case in.Position != nil:
		add("position", *in.Position)
	case rebucketedPos != nil:
		add("position", *rebucketedPos)
	}
	if in.SetAssignee {
		// AssigneeUserID is *uuid.UUID, which scans as NULL when nil.
		add("assignee_user_id", in.AssigneeUserID)
	}
	if in.SetDueDate {
		add("due_date", in.DueDate)
	}

	query := fmt.Sprintf(`
		UPDATE trip_tasks SET %s
		WHERE trip_id = $1 AND id = $2
	`, strings.Join(parts, ", "))

	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return Task{}, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Task{}, ErrTaskNotFound
	}

	var row taskRow
	if err := tx.GetContext(ctx, &row, `
		SELECT `+taskSelectColumns+taskJoins+`
		WHERE t.trip_id = $1 AND t.id = $2
	`, tripID, taskID); err != nil {
		return Task{}, err
	}

	if err := tx.Commit(); err != nil {
		return Task{}, err
	}
	return rowToTask(row), nil
}

// DeleteTask hard-deletes a single task scoped to its trip.
func (r *Repo) DeleteTask(ctx context.Context, tripID, taskID uuid.UUID) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM trip_tasks WHERE trip_id = $1 AND id = $2
	`, tripID, taskID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrTaskNotFound
	}
	return nil
}
