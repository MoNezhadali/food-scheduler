package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/MoNezhadali/foodscheduler/internal/domain"
	"github.com/MoNezhadali/foodscheduler/internal/domain/food"
	"github.com/MoNezhadali/foodscheduler/internal/domain/mealplan"
)

type MealPlanHandler struct {
	foods foodLister
}

func NewMealPlanHandler(foods foodLister) *MealPlanHandler {
	return &MealPlanHandler{foods: foods}
}

// ── request / response types ──────────────────────────────────────────────────

type mealPlanPreferences struct {
	MinBeef    int `json:"min_beef"`
	MinChicken int `json:"min_chicken"`
	MinFish    int `json:"min_fish"`
	MinPork    int `json:"min_pork"`
}

type mealPlanRequest struct {
	Count        int                 `json:"count"`
	AllergenFree []string            `json:"allergen_free"`
	Labels       []string            `json:"labels"`
	Preferences  mealPlanPreferences `json:"preferences"`
}

type mealPlanResponse struct {
	Count int            `json:"count"`
	Foods []foodResponse `json:"foods"`
}

// ── handler ───────────────────────────────────────────────────────────────────

func (h *MealPlanHandler) Generate(w http.ResponseWriter, r *http.Request) {
	var req mealPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "INVALID_JSON", Message: "invalid request body"})
		return
	}
	if req.Count < 1 {
		writeError(w, domain.ErrInvalidInput)
		return
	}

	// Fetch filtered pool from the database
	filter := food.Filter{
		Labels:           req.Labels,
		ExcludeAllergens: req.AllergenFree,
	}
	pool, err := h.foods.List(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}

	plan, err := mealplan.Plan(pool, mealplan.Request{
		Count: req.Count,
		Preferences: mealplan.Preferences{
			MinBeef:    req.Preferences.MinBeef,
			MinChicken: req.Preferences.MinChicken,
			MinFish:    req.Preferences.MinFish,
			MinPork:    req.Preferences.MinPork,
		},
	})
	if err != nil {
		if err == mealplan.ErrInsufficientFoods {
			writeJSON(w, http.StatusUnprocessableEntity, errorResponse{
				Code:    "INSUFFICIENT_FOODS",
				Message: err.Error(),
			})
			return
		}
		writeError(w, err)
		return
	}

	resp := mealPlanResponse{Count: len(plan.Foods)}
	resp.Foods = make([]foodResponse, len(plan.Foods))
	for i, f := range plan.Foods {
		resp.Foods[i] = toFoodResponse(f)
	}
	writeJSON(w, http.StatusOK, resp)
}
