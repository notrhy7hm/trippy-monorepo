package friends

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/trippyai/trippy/backend/internal/api"
	"github.com/trippyai/trippy/backend/internal/users"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

// ---- friend requests ------------------------------------------------------

type sendReq struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

func (h *Handler) Send(w http.ResponseWriter, r *http.Request) {
	var in sendReq
	if err := api.DecodeJSON(r, &in); err != nil {
		api.Err(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	req, err := h.svc.SendRequest(r.Context(), api.UserID(r.Context()), in.Username, in.Message)
	if err != nil {
		writeError(w, err)
		return
	}
	api.JSON(w, http.StatusCreated, req)
}

func (h *Handler) Accept(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	fr, err := h.svc.AcceptRequest(r.Context(), api.UserID(r.Context()), username)
	if err != nil {
		writeError(w, err)
		return
	}
	api.JSON(w, http.StatusOK, fr)
}

func (h *Handler) Decline(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if err := h.svc.DeclineRequest(r.Context(), api.UserID(r.Context()), username); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if err := h.svc.CancelRequest(r.Context(), api.UserID(r.Context()), username); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListIncoming(w http.ResponseWriter, r *http.Request) {
	rs, err := h.svc.ListIncomingRequests(r.Context(), api.UserID(r.Context()))
	if err != nil {
		slog.Error("friends incoming", "err", err)
		api.Err(w, http.StatusInternalServerError, "list_failed", "could not list incoming requests")
		return
	}
	if rs == nil {
		rs = []Request{}
	}
	api.JSON(w, http.StatusOK, rs)
}

func (h *Handler) ListOutgoing(w http.ResponseWriter, r *http.Request) {
	rs, err := h.svc.ListOutgoingRequests(r.Context(), api.UserID(r.Context()))
	if err != nil {
		slog.Error("friends outgoing", "err", err)
		api.Err(w, http.StatusInternalServerError, "list_failed", "could not list outgoing requests")
		return
	}
	if rs == nil {
		rs = []Request{}
	}
	api.JSON(w, http.StatusOK, rs)
}

// ---- friendships ----------------------------------------------------------

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	fs, err := h.svc.ListFriends(r.Context(), api.UserID(r.Context()))
	if err != nil {
		slog.Error("friends list", "err", err)
		api.Err(w, http.StatusInternalServerError, "list_failed", "could not list friends")
		return
	}
	if fs == nil {
		fs = []Friend{}
	}
	api.JSON(w, http.StatusOK, fs)
}

func (h *Handler) Remove(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if err := h.svc.RemoveFriend(r.Context(), api.UserID(r.Context()), username); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- user search ----------------------------------------------------------

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	results, err := h.svc.SearchPeople(r.Context(), api.UserID(r.Context()), q)
	if err != nil {
		slog.Error("user search", "err", err, "q", q)
		api.Err(w, http.StatusInternalServerError, "search_failed", "could not search users")
		return
	}
	if results == nil {
		results = []SearchResult{}
	}
	api.JSON(w, http.StatusOK, results)
}

// ---- shared error translation --------------------------------------------

func writeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrSelf):
		api.Err(w, http.StatusBadRequest, "self_friend", "you cannot friend yourself")
	case errors.Is(err, ErrAlreadyFriends):
		api.Err(w, http.StatusConflict, "already_friends", "you are already friends with this user")
	case errors.Is(err, ErrAlreadyPending):
		api.Err(w, http.StatusConflict, "already_pending", "a friend request is already pending between you")
	case errors.Is(err, ErrRequestNotFound):
		api.Err(w, http.StatusNotFound, "request_not_found", "friend request not found")
	case errors.Is(err, ErrNotFriends):
		api.Err(w, http.StatusNotFound, "not_friends", "you are not friends with this user")
	case errors.Is(err, ErrMessageTooLong):
		api.Err(w, http.StatusBadRequest, "message_too_long", "message is too long")
	case errors.Is(err, users.ErrNotFound):
		api.Err(w, http.StatusNotFound, "user_not_found", "user not found")
	default:
		slog.Error("friends internal error", "err", err)
		api.Err(w, http.StatusInternalServerError, "internal", "something went wrong")
	}
}
