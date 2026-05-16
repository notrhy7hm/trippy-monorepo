package planning

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/trippyai/trippy/backend/internal/api"
	"github.com/trippyai/trippy/backend/internal/trips"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

// ---- list -----------------------------------------------------------------

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tripSlug")
	tasks, err := h.svc.ListTasks(r.Context(), api.UserID(r.Context()), slug)
	if err != nil {
		writeError(w, err)
		return
	}
	if tasks == nil {
		tasks = []Task{}
	}
	api.JSON(w, http.StatusOK, tasks)
}

// ---- create ---------------------------------------------------------------

type createReq struct {
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Status           Status   `json:"status"`
	Priority         Priority `json:"priority"`
	AssigneeUsername string   `json:"assigneeUsername"`
	DueDate          string   `json:"dueDate"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in createReq
	if err := api.DecodeJSON(r, &in); err != nil {
		api.Err(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	slug := chi.URLParam(r, "tripSlug")

	create := CreateInput{
		Title:            in.Title,
		Description:      in.Description,
		Status:           in.Status,
		Priority:         in.Priority,
		AssigneeUsername: in.AssigneeUsername,
	}
	if d := in.DueDate; d != "" {
		// Service does the parsing path on update; for create we parse
		// here so the input contract is symmetrical with PATCH.
		parsed, err := parseDate(d)
		if err != nil {
			writeError(w, ErrInvalidDueDate)
			return
		}
		create.DueDate = &parsed
	}

	t, err := h.svc.CreateTask(r.Context(), api.UserID(r.Context()), slug, create)
	if err != nil {
		writeError(w, err)
		return
	}
	api.JSON(w, http.StatusCreated, t)
}

// ---- update ---------------------------------------------------------------

// updateReq uses pointers so the handler can distinguish field-omitted
// from field-present. For AssigneeUsername and DueDate the empty-string
// sentinel clears the column (service layer interprets).
type updateReq struct {
	Title            *string   `json:"title,omitempty"`
	Description      *string   `json:"description,omitempty"`
	Status           *Status   `json:"status,omitempty"`
	Priority         *Priority `json:"priority,omitempty"`
	AssigneeUsername *string   `json:"assigneeUsername,omitempty"`
	DueDate          *string   `json:"dueDate,omitempty"`
	Position         *int      `json:"position,omitempty"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var in updateReq
	if err := api.DecodeJSON(r, &in); err != nil {
		api.Err(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	slug := chi.URLParam(r, "tripSlug")
	taskID, err := uuid.Parse(chi.URLParam(r, "taskID"))
	if err != nil {
		api.Err(w, http.StatusNotFound, "task_not_found", "task not found")
		return
	}

	t, err := h.svc.UpdateTask(r.Context(), api.UserID(r.Context()), slug, taskID, UpdateInput{
		Title:            in.Title,
		Description:      in.Description,
		Status:           in.Status,
		Priority:         in.Priority,
		AssigneeUsername: in.AssigneeUsername,
		DueDateRaw:       in.DueDate,
		Position:         in.Position,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	api.JSON(w, http.StatusOK, t)
}

// ---- delete ---------------------------------------------------------------

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tripSlug")
	taskID, err := uuid.Parse(chi.URLParam(r, "taskID"))
	if err != nil {
		api.Err(w, http.StatusNotFound, "task_not_found", "task not found")
		return
	}
	if err := h.svc.DeleteTask(r.Context(), api.UserID(r.Context()), slug, taskID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- helpers --------------------------------------------------------------

func parseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// writeError centralizes typed-error -> HTTP mapping. Sibling-package
// errors (trips.ErrNotFound, trips.ErrForbidden, users.ErrNotFound) are
// translated here as well so the planning handler never returns raw DB
// errors and never returns trips's own message verbatim.
func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidTitle):
		api.Err(w, http.StatusBadRequest, "invalid_title", "title must not be empty")
	case errors.Is(err, ErrInvalidStatus):
		api.Err(w, http.StatusBadRequest, "invalid_status",
			"status must be one of todo, in_progress, done")
	case errors.Is(err, ErrInvalidPriority):
		api.Err(w, http.StatusBadRequest, "invalid_priority",
			"priority must be one of low, normal, high")
	case errors.Is(err, ErrInvalidDueDate):
		api.Err(w, http.StatusBadRequest, "invalid_due_date",
			"due date must be in YYYY-MM-DD format")
	case errors.Is(err, ErrInvalidPosition):
		api.Err(w, http.StatusBadRequest, "invalid_position",
			"position must be a non-negative integer")
	case errors.Is(err, ErrAssigneeNotFound):
		api.Err(w, http.StatusNotFound, "assignee_not_found",
			"assignee user not found")
	case errors.Is(err, ErrAssigneeNotMember):
		api.Err(w, http.StatusUnprocessableEntity, "assignee_not_member",
			"assignee must be a trip member")
	case errors.Is(err, ErrTaskNotFound):
		api.Err(w, http.StatusNotFound, "task_not_found", "task not found")
	case errors.Is(err, ErrItineraryItemNotFound):
		api.Err(w, http.StatusNotFound, "itinerary_item_not_found",
			"itinerary item not found")
	case errors.Is(err, ErrInvalidDayIndex):
		api.Err(w, http.StatusBadRequest, "invalid_day_index",
			"dayIndex must be a non-negative integer")
	case errors.Is(err, ErrInvalidDate):
		api.Err(w, http.StatusBadRequest, "invalid_date",
			"date must be in YYYY-MM-DD format")
	case errors.Is(err, ErrInvalidTime):
		api.Err(w, http.StatusBadRequest, "invalid_time",
			"time must be in HH:MM or HH:MM:SS format")
	case errors.Is(err, ErrInvalidTimeRange):
		api.Err(w, http.StatusBadRequest, "invalid_time_range",
			"startsAt must be earlier than or equal to endsAt")
	case errors.Is(err, trips.ErrNotFound):
		api.Err(w, http.StatusNotFound, "trip_not_found", "trip not found")
	case errors.Is(err, trips.ErrForbidden):
		api.Err(w, http.StatusForbidden, "forbidden",
			"you do not have access to this trip")
	default:
		slog.Error("planning internal error", "err", err)
		api.Err(w, http.StatusInternalServerError, "internal", "something went wrong")
	}
}
