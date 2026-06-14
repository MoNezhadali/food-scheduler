package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/MoNezhadali/foodscheduler/internal/domain"
	"github.com/MoNezhadali/foodscheduler/internal/domain/food"
	"github.com/MoNezhadali/foodscheduler/internal/domain/ingredient"
)

type foodStore interface {
	List(ctx context.Context, filter food.Filter) ([]food.Food, error)
	GetByID(ctx context.Context, id string) (food.Food, error)
	Create(ctx context.Context, f food.Food) (food.Food, error)
	Update(ctx context.Context, f food.Food) (food.Food, error)
	Delete(ctx context.Context, id string) error
}

type ingredientFetcher interface {
	GetByIDs(ctx context.Context, ids []string) ([]ingredient.Ingredient, error)
}

type FoodHandler struct {
	foods       foodStore
	ingredients ingredientFetcher
}

func NewFoodHandler(foods foodStore, ingredients ingredientFetcher) *FoodHandler {
	return &FoodHandler{foods: foods, ingredients: ingredients}
}

// ── request / response types ──────────────────────────────────────────────────

type foodIngredientBody struct {
	IngredientID string  `json:"ingredient_id"`
	Amount       float64 `json:"amount"`
	Unit         string  `json:"unit"`
}

type foodNutritionBody struct {
	CaloriesTotal      float64 `json:"calories_total"`
	CaloriesPerPortion float64 `json:"calories_per_portion"`
	ProteinTotal       float64 `json:"protein_total"`
	ProteinPerPortion  float64 `json:"protein_per_portion"`
	CarbsTotal         float64 `json:"carbs_total"`
	CarbsPerPortion    float64 `json:"carbs_per_portion"`
	FatTotal           float64 `json:"fat_total"`
	FatPerPortion      float64 `json:"fat_per_portion"`
}

type foodRequest struct {
	Name        string               `json:"name"`
	DisplayName string               `json:"display_name"`
	Description string               `json:"description"`
	Portions    int                  `json:"portions"`
	Ingredients []foodIngredientBody `json:"ingredients"`
	Recipe      []string             `json:"recipe"`
	Labels      []string             `json:"labels"`
}

type foodResponse struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	DisplayName string               `json:"display_name"`
	Description string               `json:"description"`
	Portions    int                  `json:"portions"`
	Ingredients []foodIngredientBody `json:"ingredients"`
	Recipe      []string             `json:"recipe"`
	Labels      []string             `json:"labels"`
	Nutrition   foodNutritionBody    `json:"nutrition"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

func toFoodResponse(f food.Food) foodResponse {
	ings := make([]foodIngredientBody, len(f.Ingredients))
	for i, fi := range f.Ingredients {
		ings[i] = foodIngredientBody{
			IngredientID: fi.IngredientID,
			Amount:       fi.Amount,
			Unit:         fi.Unit,
		}
	}
	return foodResponse{
		ID:          f.ID,
		Name:        f.Name,
		DisplayName: f.DisplayName,
		Description: f.Description,
		Portions:    f.Portions,
		Ingredients: ings,
		Recipe:      f.Recipe,
		Labels:      f.Labels,
		Nutrition: foodNutritionBody{
			CaloriesTotal:      f.Nutrition.CaloriesTotal,
			CaloriesPerPortion: f.Nutrition.CaloriesPerPortion,
			ProteinTotal:       f.Nutrition.ProteinTotal,
			ProteinPerPortion:  f.Nutrition.ProteinPerPortion,
			CarbsTotal:         f.Nutrition.CarbsTotal,
			CarbsPerPortion:    f.Nutrition.CarbsPerPortion,
			FatTotal:           f.Nutrition.FatTotal,
			FatPerPortion:      f.Nutrition.FatPerPortion,
		},
		CreatedAt: f.CreatedAt,
		UpdatedAt: f.UpdatedAt,
	}
}

func (req foodRequest) toDomain() food.Food {
	portions := req.Portions
	if portions <= 0 {
		portions = 4
	}
	ings := make([]food.FoodIngredient, len(req.Ingredients))
	for i, fi := range req.Ingredients {
		ings[i] = food.FoodIngredient{
			IngredientID: fi.IngredientID,
			Amount:       fi.Amount,
			Unit:         fi.Unit,
		}
	}
	return food.Food{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Portions:    portions,
		Ingredients: ings,
		Recipe:      req.Recipe,
		Labels:      req.Labels,
	}
}

// ── handlers ──────────────────────────────────────────────────────────────────

func (h *FoodHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := food.Filter{
		Labels:           q["label"],
		ExcludeAllergens: q["allergen_free"],
	}
	if s := q.Get("search"); s != "" {
		filter.Search = &s
	}

	foods, err := h.foods.List(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := make([]foodResponse, len(foods))
	for i, f := range foods {
		resp[i] = toFoodResponse(f)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *FoodHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	f, err := h.foods.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toFoodResponse(f))
}

func (h *FoodHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req foodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "INVALID_JSON", Message: "invalid request body"})
		return
	}
	if err := validateFoodRequest(req); err != nil {
		writeError(w, err)
		return
	}
	f := req.toDomain()
	if err := h.computeAndSetNutrition(r.Context(), &f); err != nil {
		writeError(w, err)
		return
	}
	created, err := h.foods.Create(r.Context(), f)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toFoodResponse(created))
}

func (h *FoodHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req foodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "INVALID_JSON", Message: "invalid request body"})
		return
	}
	if err := validateFoodRequest(req); err != nil {
		writeError(w, err)
		return
	}
	f := req.toDomain()
	f.ID = id
	if err := h.computeAndSetNutrition(r.Context(), &f); err != nil {
		writeError(w, err)
		return
	}
	updated, err := h.foods.Update(r.Context(), f)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toFoodResponse(updated))
}

func (h *FoodHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.foods.Delete(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func (h *FoodHandler) computeAndSetNutrition(ctx context.Context, f *food.Food) error {
	ids := make([]string, len(f.Ingredients))
	for i, fi := range f.Ingredients {
		ids[i] = fi.IngredientID
	}
	ings, err := h.ingredients.GetByIDs(ctx, ids)
	if err != nil {
		return err
	}
	ingMap := make(map[string]ingredient.Ingredient, len(ings))
	for _, ing := range ings {
		ingMap[ing.ID] = ing
	}
	f.Nutrition = food.ComputeNutrition(*f, ingMap)
	return nil
}

func validateFoodRequest(req foodRequest) error {
	if req.Name == "" || req.DisplayName == "" {
		return domain.ErrInvalidInput
	}
	if len(req.Ingredients) == 0 {
		return domain.ErrInvalidInput
	}
	for _, fi := range req.Ingredients {
		if fi.IngredientID == "" || fi.Amount <= 0 || fi.Unit == "" {
			return domain.ErrInvalidInput
		}
	}
	return nil
}
