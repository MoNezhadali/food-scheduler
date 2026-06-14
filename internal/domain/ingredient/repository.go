package ingredient

import "context"

type Filter struct {
	FoodGroup    *FoodGroup
	AllergenFree []Allergen
	Search       *string
}

type Repository interface {
	List(ctx context.Context, filter Filter) ([]Ingredient, error)
	GetByID(ctx context.Context, id string) (Ingredient, error)
	GetByName(ctx context.Context, name string) (Ingredient, error)
	GetByIDs(ctx context.Context, ids []string) ([]Ingredient, error)
	Create(ctx context.Context, i Ingredient) (Ingredient, error)
	Update(ctx context.Context, i Ingredient) (Ingredient, error)
	Delete(ctx context.Context, id string) error
	ListMissingNutrition(ctx context.Context) ([]Ingredient, error)
	UpdateNutrition(ctx context.Context, id string, n NutritionInfo) error
}
