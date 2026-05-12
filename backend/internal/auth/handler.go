package auth

import (
	"errors"
	"net/http"

	"github.com/trippyai/trippy/backend/internal/api"
)

type Handler struct{ svc *Service }

func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

type registerReq struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginReq struct {
	Identifier string `json:"identifier"` // email or username
	Password   string `json:"password"`
}

type authResp struct {
	Token string `json:"token"`
	User  any    `json:"user"`
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var in registerReq
	if err := api.DecodeJSON(r, &in); err != nil {
		api.Err(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	u, tok, err := h.svc.Register(r.Context(), RegisterInput{
		Email: in.Email, Username: in.Username, Password: in.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrUserExists):
			api.Err(w, http.StatusConflict, "user_exists", "email or username already taken")
		case errors.Is(err, ErrInvalidInput):
			api.Err(w, http.StatusBadRequest, "invalid_input", "email, username, and password (min 8 chars) required")
		default:
			api.Err(w, http.StatusInternalServerError, "register_failed", "could not register")
		}
		return
	}
	api.JSON(w, http.StatusCreated, authResp{Token: tok, User: u})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var in loginReq
	if err := api.DecodeJSON(r, &in); err != nil {
		api.Err(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	u, tok, err := h.svc.Login(r.Context(), in.Identifier, in.Password)
	if err != nil {
		api.Err(w, http.StatusUnauthorized, "invalid_credentials", "invalid credentials")
		return
	}
	api.JSON(w, http.StatusOK, authResp{Token: tok, User: u})
}
