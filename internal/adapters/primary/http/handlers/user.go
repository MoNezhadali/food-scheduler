package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	appuser "github.com/MoNezhadali/foodscheduler/internal/application/user"
	domuser "github.com/MoNezhadali/foodscheduler/internal/domain/user"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/auth"
)

type registerUC interface {
	Execute(ctx context.Context, cmd appuser.RegisterCmd) (domuser.User, error)
}

type loginUC interface {
	Execute(ctx context.Context, email, password string) (auth.TokenPair, error)
}

type refreshUC interface {
	Execute(ctx context.Context, refreshToken string) (auth.TokenPair, error)
}

type UserHandler struct {
	register registerUC
	login    loginUC
	refresh  refreshUC
}

func NewUserHandler(register registerUC, login loginUC, refresh refreshUC) *UserHandler {
	return &UserHandler{register: register, login: login, refresh: refresh}
}

func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "INVALID_JSON", Message: "invalid request body"})
		return
	}
	u, err := h.register.Execute(r.Context(), appuser.RegisterCmd{
		Email:    body.Email,
		Password: body.Password,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"id":    u.ID,
		"email": u.Email,
		"role":  u.Role,
	})
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "INVALID_JSON", Message: "invalid request body"})
		return
	}
	tokens, err := h.login.Execute(r.Context(), body.Email, body.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}

func (h *UserHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "INVALID_JSON", Message: "invalid request body"})
		return
	}
	tokens, err := h.refresh.Execute(r.Context(), body.RefreshToken)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tokens)
}
