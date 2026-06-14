package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/MoNezhadali/foodscheduler/internal/domain"
	"github.com/MoNezhadali/foodscheduler/internal/domain/ingredient"
)

type ingredientStore interface {
	List(ctx context.Context, filter ingredient.Filter) ([]ingredient.Ingredient, error)
	GetByID(ctx context.Context, id string) (ingredient.Ingredient, error)
	Create(ctx context.Context, i ingredient.Ingredient) (ingredient.Ingredient, error)
	Update(ctx context.Context, i ingredient.Ingredient) (ingredient.Ingredient, error)
	Delete(ctx context.Context, id string) error
}

type IngredientHandler struct {
	store ingredientStore
}

func NewIngredientHandler(store ingredientStore) *IngredientHandler {
	return &IngredientHandler{store: store}
}

// ── request / response types ──────────────────────────────────────────────────

type nutritionBody struct {
	CaloriesPerBase *float64 `json:"calories_per_base"`
	ProteinPerBase  *float64 `json:"protein_per_base"`
	CarbsPerBase    *float64 `json:"carbs_per_base"`
	FatPerBase      *float64 `json:"fat_per_base"`
}

type ingredientRequest struct {
	Name        string             `json:"name"`
	DisplayName string             `json:"display_name"`
	BaseUnit    string             `json:"base_unit"`
	FoodGroup   string             `json:"food_group"`
	Allergens   []string           `json:"allergens"`
	UnitMap     map[string]float64 `json:"unit_map"`
	Nutrition   nutritionBody      `json:"nutrition"`
}

type ingredientResponse struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	DisplayName string             `json:"display_name"`
	BaseUnit    string             `json:"base_unit"`
	FoodGroup   string             `json:"food_group"`
	Allergens   []string           `json:"allergens"`
	UnitMap     map[string]float64 `json:"unit_map"`
	Nutrition   nutritionBody      `json:"nutrition"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

func toIngredientResponse(i ingredient.Ingredient) ingredientResponse {
	allergens := make([]string, len(i.Allergens))
	for idx, a := range i.Allergens {
		allergens[idx] = string(a)
	}
	return ingredientResponse{
		ID:          i.ID,
		Name:        i.Name,
		DisplayName: i.DisplayName,
		BaseUnit:    i.BaseUnit,
		FoodGroup:   string(i.FoodGroup),
		Allergens:   allergens,
		UnitMap:     map[string]float64(i.UnitMap),
		Nutrition: nutritionBody{
			CaloriesPerBase: i.Nutrition.CaloriesPerBase,
			ProteinPerBase:  i.Nutrition.ProteinPerBase,
			CarbsPerBase:    i.Nutrition.CarbsPerBase,
			FatPerBase:      i.Nutrition.FatPerBase,
		},
		CreatedAt: i.CreatedAt,
		UpdatedAt: i.UpdatedAt,
	}
}

func (req ingredientRequest) toDomain() ingredient.Ingredient {
	allergens := make([]ingredient.Allergen, len(req.Allergens))
	for i, a := range req.Allergens {
		allergens[i] = ingredient.Allergen(a)
	}
	return ingredient.Ingredient{
		Name:        req.Name,
		DisplayName: req.DisplayName,
		BaseUnit:    req.BaseUnit,
		FoodGroup:   ingredient.FoodGroup(req.FoodGroup),
		Allergens:   allergens,
		UnitMap:     ingredient.UnitMap(req.UnitMap),
		Nutrition: ingredient.NutritionInfo{
			CaloriesPerBase: req.Nutrition.CaloriesPerBase,
			ProteinPerBase:  req.Nutrition.ProteinPerBase,
			CarbsPerBase:    req.Nutrition.CarbsPerBase,
			FatPerBase:      req.Nutrition.FatPerBase,
		},
	}
}

// ── handlers ──────────────────────────────────────────────────────────────────

func (h *IngredientHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := ingredient.Filter{}

	if fg := q.Get("food_group"); fg != "" {
		g := ingredient.FoodGroup(fg)
		filter.FoodGroup = &g
	}
	for _, a := range q["allergen_free"] {
		filter.AllergenFree = append(filter.AllergenFree, ingredient.Allergen(a))
	}
	if s := q.Get("search"); s != "" {
		filter.Search = &s
	}

	items, err := h.store.List(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := make([]ingredientResponse, len(items))
	for i, ing := range items {
		resp[i] = toIngredientResponse(ing)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *IngredientHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ing, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toIngredientResponse(ing))
}

func (h *IngredientHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req ingredientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "INVALID_JSON", Message: "invalid request body"})
		return
	}
	if err := validateIngredientRequest(req); err != nil {
		writeError(w, err)
		return
	}
	created, err := h.store.Create(r.Context(), req.toDomain())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toIngredientResponse(created))
}

func (h *IngredientHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req ingredientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "INVALID_JSON", Message: "invalid request body"})
		return
	}
	if err := validateIngredientRequest(req); err != nil {
		writeError(w, err)
		return
	}
	dom := req.toDomain()
	dom.ID = id
	updated, err := h.store.Update(r.Context(), dom)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toIngredientResponse(updated))
}

func (h *IngredientHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.Delete(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func validateIngredientRequest(req ingredientRequest) error {
	if req.Name == "" || req.DisplayName == "" || req.BaseUnit == "" || req.FoodGroup == "" {
		return domain.ErrInvalidInput
	}
	if len(req.UnitMap) == 0 {
		return domain.ErrInvalidInput
	}
	return nil
}
