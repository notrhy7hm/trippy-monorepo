package users

import (
	"net/http"

	"github.com/trippyai/trippy/backend/internal/api"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	u, err := h.svc.ByID(r.Context(), api.UserID(r.Context()))
	if err != nil {
		api.Err(w, http.StatusNotFound, "not_found", "user not found")
		return
	}
	api.JSON(w, http.StatusOK, u)
}

type updateMeReq struct {
	DisplayName *string `json:"displayName,omitempty"`
	Bio         *string `json:"bio,omitempty"`
	AvatarURL   *string `json:"avatarUrl,omitempty"`
}

func (h *Handler) UpdateMe(w http.ResponseWriter, r *http.Request) {
	var in updateMeReq
	if err := api.DecodeJSON(r, &in); err != nil {
		api.Err(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	u, err := h.svc.UpdateProfile(r.Context(), api.UserID(r.Context()), UpdateInput{
		DisplayName: in.DisplayName,
		Bio:         in.Bio,
		AvatarURL:   in.AvatarURL,
	})
	if err != nil {
		api.Err(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	api.JSON(w, http.StatusOK, u)
}
