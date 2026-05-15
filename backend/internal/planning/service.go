package planning

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/trippyai/trippy/backend/internal/users"
)

var (
	ErrInvalidTitle      = errors.New("title must not be empty")
	ErrInvalidStatus     = errors.New("invalid status")
	ErrInvalidPriority   = errors.New("invalid priority")
	ErrInvalidDueDate    = errors.New("invalid due date")
	ErrInvalidPosition   = errors.New("position must be non-negative")
	ErrAssigneeNotFound  = errors.New("assignee user not found")
	ErrAssigneeNotMember = errors.New("assignee is not a trip member")
)

// TripAccess is the minimum surface planning needs from the trips package.
// trips.Service satisfies it. Keeping it small avoids any import cycle and
// keeps the planning module testable in isolation.
type TripAccess interface {
	// AssertMember returns the trip's UUID if userID belongs to the trip
	// addressed by slug. Implementations return the trips package's
	// ErrForbidden / ErrNotFound sentinels; the service handler maps them
	// onto safe HTTP codes via writeError below.
	AssertMember(ctx context.Context, slug string, userID uuid.UUID) (uuid.UUID, error)
	// IsTripMember reports whether userID is a member of tripID.
	IsTripMember(ctx context.Context, tripID, userID uuid.UUID) (bool, error)
}

type Service struct {
	repo  *Repo
	users *users.Service
	trips TripAccess
}

func NewService(r *Repo, u *users.Service, t TripAccess) *Service {
	return &Service{repo: r, users: u, trips: t}
}

// ListTasks returns the trip's tasks if the caller is a trip member.
func (s *Service) ListTasks(ctx context.Context, callerID uuid.UUID, slug string) ([]Task, error) {
	tripID, err := s.trips.AssertMember(ctx, slug, callerID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListTasks(ctx, tripID)
}

// CreateTask inserts a new task. Title is required; status/priority default
// to "todo"/"normal" when omitted. Assignee is resolved by username and
// must already be a trip member.
func (s *Service) CreateTask(ctx context.Context, callerID uuid.UUID, slug string, in CreateInput) (Task, error) {
	tripID, err := s.trips.AssertMember(ctx, slug, callerID)
	if err != nil {
		return Task{}, err
	}

	title := strings.TrimSpace(in.Title)
	if title == "" {
		return Task{}, ErrInvalidTitle
	}

	status := in.Status
	if status == "" {
		status = StatusTodo
	}
	if !status.IsValid() {
		return Task{}, ErrInvalidStatus
	}

	priority := in.Priority
	if priority == "" {
		priority = PriorityNormal
	}
	if !priority.IsValid() {
		return Task{}, ErrInvalidPriority
	}

	assigneeID, err := s.resolveAssignee(ctx, tripID, in.AssigneeUsername)
	if err != nil {
		return Task{}, err
	}

	return s.repo.CreateTask(ctx, CreateRow{
		TripID:          tripID,
		Title:           title,
		Description:     in.Description,
		Status:          status,
		Priority:        priority,
		AssigneeUserID:  assigneeID,
		CreatedByUserID: callerID,
		DueDate:         in.DueDate,
	})
}

// UpdateTask applies a partial update to a single task. Membership is
// checked once at the top; per-field validation runs only on fields that
// were sent (pointer non-nil).
func (s *Service) UpdateTask(ctx context.Context, callerID uuid.UUID, slug string, taskID uuid.UUID, in UpdateInput) (Task, error) {
	tripID, err := s.trips.AssertMember(ctx, slug, callerID)
	if err != nil {
		return Task{}, err
	}

	// Ensure the task exists before doing field-by-field validation, so
	// stale-task errors win over field errors (better UX, no leakage).
	if _, err := s.repo.TaskByIDForTrip(ctx, tripID, taskID); err != nil {
		return Task{}, err
	}

	row := UpdateRow{}

	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if t == "" {
			return Task{}, ErrInvalidTitle
		}
		row.Title = &t
	}
	if in.Description != nil {
		row.Description = in.Description
	}
	if in.Status != nil {
		if !in.Status.IsValid() {
			return Task{}, ErrInvalidStatus
		}
		row.Status = in.Status
	}
	if in.Priority != nil {
		if !in.Priority.IsValid() {
			return Task{}, ErrInvalidPriority
		}
		row.Priority = in.Priority
	}
	if in.Position != nil {
		if *in.Position < 0 {
			return Task{}, ErrInvalidPosition
		}
		row.Position = in.Position
	}

	if in.AssigneeUsername != nil {
		row.SetAssignee = true
		if strings.TrimSpace(*in.AssigneeUsername) == "" {
			row.AssigneeUserID = nil // clear
		} else {
			assigneeID, err := s.resolveAssignee(ctx, tripID, *in.AssigneeUsername)
			if err != nil {
				return Task{}, err
			}
			row.AssigneeUserID = assigneeID
		}
	}

	if in.DueDateRaw != nil {
		row.SetDueDate = true
		raw := strings.TrimSpace(*in.DueDateRaw)
		if raw == "" {
			row.DueDate = nil // clear
		} else {
			parsed, err := time.Parse("2006-01-02", raw)
			if err != nil {
				return Task{}, ErrInvalidDueDate
			}
			row.DueDate = &parsed
		}
	}

	return s.repo.UpdateTask(ctx, tripID, taskID, row)
}

// DeleteTask hard-deletes a task. Any trip member can call this in M2.
func (s *Service) DeleteTask(ctx context.Context, callerID uuid.UUID, slug string, taskID uuid.UUID) error {
	tripID, err := s.trips.AssertMember(ctx, slug, callerID)
	if err != nil {
		return err
	}
	return s.repo.DeleteTask(ctx, tripID, taskID)
}

// resolveAssignee turns a (possibly empty) username into an optional user
// UUID. Empty / whitespace -> (nil, nil). Unknown username ->
// ErrAssigneeNotFound. Known username but not a trip member ->
// ErrAssigneeNotMember.
func (s *Service) resolveAssignee(ctx context.Context, tripID uuid.UUID, username string) (*uuid.UUID, error) {
	uname := strings.ToLower(strings.TrimSpace(username))
	if uname == "" {
		return nil, nil
	}
	u, err := s.users.ByUsername(ctx, uname)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			return nil, ErrAssigneeNotFound
		}
		return nil, err
	}
	ok, err := s.trips.IsTripMember(ctx, tripID, u.ID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrAssigneeNotMember
	}
	id := u.ID
	return &id, nil
}
