package handlers

import (
	"context"

	"github.com/MoNezhadali/foodscheduler/internal/domain/food"
	"github.com/MoNezhadali/foodscheduler/internal/domain/ingredient"
)

// ingredientFetcher is satisfied by IngredientRepo.
type ingredientFetcher interface {
	GetByIDs(ctx context.Context, ids []string) ([]ingredient.Ingredient, error)
}

// foodFetcher is satisfied by FoodRepo.
type foodFetcher interface {
	GetByIDs(ctx context.Context, ids []string) ([]food.Food, error)
}
