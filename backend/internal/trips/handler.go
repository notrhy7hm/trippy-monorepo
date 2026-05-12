package trips

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/trippyai/trippy/backend/internal/api"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

type createReq struct {
	Title       string     `json:"title"`
	Description string     `json:"description"`
	StartsOn    *time.Time `json:"startsOn,omitempty"`
	EndsOn      *time.Time `json:"endsOn,omitempty"`
	Visibility  Visibility `json:"visibility,omitempty"`
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in createReq
	if err := api.DecodeJSON(r, &in); err != nil {
		api.Err(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	t, err := h.svc.Create(r.Context(), api.UserID(r.Context()), CreateInput{
		Title:       in.Title,
		Description: in.Description,
		StartsOn:    in.StartsOn,
		EndsOn:      in.EndsOn,
		Visibility:  in.Visibility,
	})
	if err != nil {
		api.Err(w, http.StatusBadRequest, "create_failed", err.Error())
		return
	}
	api.JSON(w, http.StatusCreated, t)
}

func (h *Handler) ListMine(w http.ResponseWriter, r *http.Request) {
	ts, err := h.svc.ListForUser(r.Context(), api.UserID(r.Context()))
	if err != nil {
		slog.Error("trips list", "err", err, "userId", api.UserID(r.Context()))
		api.Err(w, http.StatusInternalServerError, "list_failed", "could not list trips")
		return
	}
	if ts == nil {
		ts = []Trip{}
	}
	api.JSON(w, http.StatusOK, ts)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tripSlug")
	t, err := h.svc.BySlugForUser(r.Context(), slug, api.UserID(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	api.JSON(w, http.StatusOK, t)
}

type updateReq struct {
	Title       *string     `json:"title,omitempty"`
	Description *string     `json:"description,omitempty"`
	StartsOn    *time.Time  `json:"startsOn,omitempty"`
	EndsOn      *time.Time  `json:"endsOn,omitempty"`
	Visibility  *Visibility `json:"visibility,omitempty"`
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var in updateReq
	if err := api.DecodeJSON(r, &in); err != nil {
		api.Err(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	slug := chi.URLParam(r, "tripSlug")
	t, err := h.svc.Update(r.Context(), slug, api.UserID(r.Context()), UpdateInput{
		Title:       in.Title,
		Description: in.Description,
		StartsOn:    in.StartsOn,
		EndsOn:      in.EndsOn,
		Visibility:  in.Visibility,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	api.JSON(w, http.StatusOK, t)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tripSlug")
	if err := h.svc.Delete(r.Context(), slug, api.UserID(r.Context())); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tripSlug")
	ms, err := h.svc.ListMembers(r.Context(), slug, api.UserID(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	if ms == nil {
		ms = []Member{}
	}
	api.JSON(w, http.StatusOK, ms)
}

// CreateInvitePlaceholder is intentionally a stub for M0. M1 replaces it with
// real invite tokens, email lookup, and notifications.
func (h *Handler) CreateInvitePlaceholder(w http.ResponseWriter, r *http.Request) {
	api.JSON(w, http.StatusAccepted, map[string]string{
		"status":  "not_implemented",
		"message": "trip invites land in M1",
	})
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		api.Err(w, http.StatusNotFound, "not_found", "trip not found")
	case errors.Is(err, ErrForbidden):
		api.Err(w, http.StatusForbidden, "forbidden", "you do not have access to this trip")
	default:
		slog.Error("trips internal error", "err", err)
		api.Err(w, http.StatusInternalServerError, "internal", "something went wrong")
	}
}
