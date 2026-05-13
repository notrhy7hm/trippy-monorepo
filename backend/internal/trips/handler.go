package trips

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/trippyai/trippy/backend/internal/api"
	"github.com/trippyai/trippy/backend/internal/users"
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

// ---------------------------------------------------------------------------
// invites
// ---------------------------------------------------------------------------

type createInviteReq struct {
	Identifier string `json:"identifier"`
	Role       Role   `json:"role,omitempty"`
}

func (h *Handler) CreateInvite(w http.ResponseWriter, r *http.Request) {
	var in createInviteReq
	if err := api.DecodeJSON(r, &in); err != nil {
		api.Err(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	slug := chi.URLParam(r, "tripSlug")
	inv, err := h.svc.CreateInvite(r.Context(), api.UserID(r.Context()), slug, CreateInviteInput{
		Identifier: in.Identifier,
		Role:       in.Role,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	api.JSON(w, http.StatusCreated, inv)
}

func (h *Handler) ListInvites(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tripSlug")
	invs, err := h.svc.ListInvites(r.Context(), api.UserID(r.Context()), slug)
	if err != nil {
		writeError(w, err)
		return
	}
	if invs == nil {
		invs = []Invite{}
	}
	api.JSON(w, http.StatusOK, invs)
}

func (h *Handler) RevokeInvite(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "tripSlug")
	token := chi.URLParam(r, "token")
	if err := h.svc.RevokeInvite(r.Context(), api.UserID(r.Context()), slug, token); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) PreviewInvite(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	inv, err := h.svc.PreviewInvite(r.Context(), api.UserID(r.Context()), token)
	if err != nil {
		writeError(w, err)
		return
	}
	api.JSON(w, http.StatusOK, inv)
}

func (h *Handler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	trip, err := h.svc.AcceptInvite(r.Context(), api.UserID(r.Context()), token)
	if err != nil {
		writeError(w, err)
		return
	}
	api.JSON(w, http.StatusOK, trip)
}

func (h *Handler) DeclineInvite(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if err := h.svc.DeclineInvite(r.Context(), api.UserID(r.Context()), token); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListMyInvites(w http.ResponseWriter, r *http.Request) {
	invs, err := h.svc.ListMyInvites(r.Context(), api.UserID(r.Context()))
	if err != nil {
		writeError(w, err)
		return
	}
	if invs == nil {
		invs = []Invite{}
	}
	api.JSON(w, http.StatusOK, invs)
}

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		api.Err(w, http.StatusNotFound, "not_found", "trip not found")
	case errors.Is(err, ErrForbidden):
		api.Err(w, http.StatusForbidden, "forbidden", "you do not have access to this trip")
	case errors.Is(err, users.ErrNotFound):
		api.Err(w, http.StatusNotFound, "user_not_found", "user not found")
	case errors.Is(err, ErrInvalidRole):
		api.Err(w, http.StatusBadRequest, "invalid_role", "role is not assignable")
	case errors.Is(err, ErrInviteBadIdentifier):
		api.Err(w, http.StatusBadRequest, "bad_request", "identifier is required")
	case errors.Is(err, ErrCannotInviteSelf):
		api.Err(w, http.StatusBadRequest, "self_invite", "you cannot invite yourself")
	case errors.Is(err, ErrAlreadyMember):
		api.Err(w, http.StatusConflict, "already_member", "user is already a trip member")
	case errors.Is(err, ErrNotFriends):
		api.Err(w, http.StatusForbidden, "not_friends", "only friends can be invited to a trip")
	case errors.Is(err, ErrInviteExists):
		api.Err(w, http.StatusConflict, "invite_exists", "a pending invite already exists")
	case errors.Is(err, ErrInviteNotFound):
		api.Err(w, http.StatusNotFound, "invite_not_found", "trip invite not found")
	case errors.Is(err, ErrInviteNotPending):
		api.Err(w, http.StatusConflict, "invite_not_pending", "this invite is no longer pending")
	case errors.Is(err, ErrInviteExpired):
		api.Err(w, http.StatusGone, "invite_expired", "this invite has expired")
	case errors.Is(err, ErrInviteNotYours):
		api.Err(w, http.StatusForbidden, "invite_not_yours", "this invite is addressed to someone else")
	default:
		slog.Error("trips internal error", "err", err)
		api.Err(w, http.StatusInternalServerError, "internal", "something went wrong")
	}
}
