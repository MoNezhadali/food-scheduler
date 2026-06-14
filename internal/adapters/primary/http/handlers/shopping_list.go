package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/MoNezhadali/foodscheduler/internal/domain"
	"github.com/MoNezhadali/foodscheduler/internal/domain/ingredient"
	"github.com/MoNezhadali/foodscheduler/internal/domain/shoppinglist"
)

type ShoppingListHandler struct {
	foods       foodFetcher
	ingredients ingredientFetcher
}

func NewShoppingListHandler(foods foodFetcher, ingredients ingredientFetcher) *ShoppingListHandler {
	return &ShoppingListHandler{foods: foods, ingredients: ingredients}
}

// ── request / response types ──────────────────────────────────────────────────

type shoppingListRequest struct {
	FoodIDs []string `json:"food_ids"`
}

type shoppingItemResponse struct {
	IngredientID string  `json:"ingredient_id"`
	Name         string  `json:"name"`
	DisplayName  string  `json:"display_name"`
	TotalAmount  float64 `json:"total_amount"`
	Unit         string  `json:"unit"`
}

type shoppingListResponse struct {
	TotalItems int                              `json:"total_items"`
	Categories map[string][]shoppingItemResponse `json:"categories"`
}

func toShoppingListResponse(sl shoppinglist.ShoppingList) shoppingListResponse {
	cats := make(map[string][]shoppingItemResponse, len(sl.Categories))
	for group, items := range sl.Categories {
		resp := make([]shoppingItemResponse, len(items))
		for i, item := range items {
			resp[i] = shoppingItemResponse{
				IngredientID: item.IngredientID,
				Name:         item.Name,
				DisplayName:  item.DisplayName,
				TotalAmount:  item.TotalAmount,
				Unit:         item.Unit,
			}
		}
		cats[group] = resp
	}
	return shoppingListResponse{TotalItems: sl.TotalItems, Categories: cats}
}

// ── handler ───────────────────────────────────────────────────────────────────

func (h *ShoppingListHandler) Generate(w http.ResponseWriter, r *http.Request) {
	var req shoppingListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Code: "INVALID_JSON", Message: "invalid request body"})
		return
	}
	if len(req.FoodIDs) == 0 {
		writeError(w, domain.ErrInvalidInput)
		return
	}

	foods, err := h.foods.GetByIDs(r.Context(), req.FoodIDs)
	if err != nil {
		writeError(w, err)
		return
	}

	// Collect unique ingredient IDs across all foods
	seen := make(map[string]bool)
	var ingIDs []string
	for _, f := range foods {
		for _, fi := range f.Ingredients {
			if !seen[fi.IngredientID] {
				seen[fi.IngredientID] = true
				ingIDs = append(ingIDs, fi.IngredientID)
			}
		}
	}

	ings, err := h.ingredients.GetByIDs(r.Context(), ingIDs)
	if err != nil {
		writeError(w, err)
		return
	}
	ingMap := make(map[string]ingredient.Ingredient, len(ings))
	for _, ing := range ings {
		ingMap[ing.ID] = ing
	}

	sl := shoppinglist.Generate(foods, ingMap)
	writeJSON(w, http.StatusOK, toShoppingListResponse(sl))
}
