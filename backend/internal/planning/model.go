// Package planning owns the per-trip planning workspace (M2). It is a
// peer of the trips package, not a child: trip access (slug -> tripID,
// membership) is injected via a small interface so planning never imports
// trips.Repo or trips internals.
package planning

import (
	"time"

	"github.com/google/uuid"
)

// Status mirrors the task_status enum from migration 0004.
type Status string

const (
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

// IsValid reports whether s is one of the known enum values.
func (s Status) IsValid() bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusDone:
		return true
	}
	return false
}

// Priority mirrors the task_priority enum from migration 0004.
type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityNormal Priority = "normal"
	PriorityHigh   Priority = "high"
)

// IsValid reports whether p is one of the known enum values.
func (p Priority) IsValid() bool {
	switch p {
	case PriorityLow, PriorityNormal, PriorityHigh:
		return true
	}
	return false
}

// Party is the privacy-safe shape used wherever a user appears in a task
// payload (assignee, created_by). Matches the trip/friend party shape so
// the frontend can render it identically.
type Party struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
}

// Task is the API-facing representation of a trip task. Internal UUIDs
// stay out of every nested object except the task's own id (per the spec).
type Task struct {
	ID          uuid.UUID  `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      Status     `json:"status"`
	Priority    Priority   `json:"priority"`
	Assignee    *Party     `json:"assignee,omitempty"`
	CreatedBy   Party      `json:"createdBy"`
	Position    int        `json:"position"`
	DueDate     *time.Time `json:"dueDate,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// CreateInput is what the service receives after the handler decodes/
// normalizes the request. The handler validates shape; the service
// validates business rules.
type CreateInput struct {
	Title            string
	Description      string
	Status           Status
	Priority         Priority
	AssigneeUsername string // empty -> no assignee
	DueDate          *time.Time
}

// UpdateInput uses pointers to distinguish "field omitted" (nil, leave
// unchanged) from "field present" (non-nil, evaluate). For the two
// genuinely nullable fields the empty-string sentinel clears them:
//
//	AssigneeUsername == nil     -> no change
//	AssigneeUsername != nil &&  -> set or clear
//	               *== ""       -> clear
//	               *!= ""       -> set (must resolve + be a member)
//
//	DueDateRaw      == nil      -> no change
//	DueDateRaw      != nil && "" -> clear
//	DueDateRaw      != nil && _  -> parse YYYY-MM-DD
type UpdateInput struct {
	Title            *string
	Description      *string
	Status           *Status
	Priority         *Priority
	AssigneeUsername *string
	DueDateRaw       *string
	Position         *int
}
