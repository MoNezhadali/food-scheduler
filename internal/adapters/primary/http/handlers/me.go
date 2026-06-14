package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/MoNezhadali/foodscheduler/internal/adapters/primary/http/middleware"
	"github.com/MoNezhadali/foodscheduler/internal/domain"
	domuser "github.com/MoNezhadali/foodscheduler/internal/domain/user"
)

type userProfileStore interface {
	GetByID(ctx context.Context, id string) (domuser.User, error)
	UpdatePreferences(ctx context.Context, id string, prefs domuser.Preferences) error
}

type MeHandler struct {
	users userProfileStore
}

func NewMeHandler(users userProfileStore) *MeHandler {
	return &MeHandler{users: users}
}

// ── request / response types ──────────────────────────────────────────────────

type preferencesBody struct {
	ExcludedAllergens   []string `json:"excluded_allergens"`
	DietaryRestrictions []string `json:"dietary_restrictions"`
}

type meResponse struct {
	ID          string          `json:"id"`
	Email       string          `json:"email"`
	Role        string          `json:"role"`
	Preferences preferencesBody `json:"preferences"`
}

func toMeResponse(u domuser.User) meResponse {
	allergens := u.Preferences.ExcludedAllergens
	if allergens == nil {
		allergens = []string{}
	}
	restrictions := u.Preferences.DietaryRestrictions
	if restrictions == nil {
		restrictions = []string{}
	}
	return meResponse{
		ID:    u.ID,
		Email: u.Email,
		Role:  u.Role,
		Preferences: preferencesBody{
			ExcludedAllergens:   allergens,
			DietaryRestrictions: restrictions,
		},
	}
}

// ── handlers ──────────────────────────────────────────────────────────────────

// GetProfile handles GET /v1/me
func (h *MeHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, domain.ErrUnauthorized)
		return
	}
	u, err := h.users.GetByID(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toMeResponse(u))
}

// GetPreferences handles GET /v1/me/preferences
func (h *MeHandler) GetPreferences(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, domain.ErrUnauthorized)
		return
	}
	u, err := h.users.GetByID(r.Context(), claims.UserID)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := toMeResponse(u)
	writeJSON(w, http.StatusOK, resp.Preferences)
}

// UpdatePreferences handles PUT /v1/me/preferences
func (h *MeHandler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		writeError(w, domain.ErrUnauthorized)
		return
	}

	var body preferencesBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "INVALID_JSON", Message: "invalid request body"})
		return
	}

	prefs := domuser.Preferences{
		ExcludedAllergens:   body.ExcludedAllergens,
		DietaryRestrictions: body.DietaryRestrictions,
	}
	if prefs.ExcludedAllergens == nil {
		prefs.ExcludedAllergens = []string{}
	}
	if prefs.DietaryRestrictions == nil {
		prefs.DietaryRestrictions = []string{}
	}

	if err := h.users.UpdatePreferences(r.Context(), claims.UserID, prefs); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, body)
}
