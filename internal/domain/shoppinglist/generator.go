package shoppinglist

import (
	"github.com/MoNezhadali/foodscheduler/internal/domain/food"
	"github.com/MoNezhadali/foodscheduler/internal/domain/ingredient"
)

// Generate aggregates ingredients across all foods, converts amounts to each
// ingredient's base unit, then groups the result by food group.
func Generate(foods []food.Food, ingredients map[string]ingredient.Ingredient) ShoppingList {
	totals := make(map[string]*Item)

	for _, f := range foods {
		for _, fi := range f.Ingredients {
			ing, ok := ingredients[fi.IngredientID]
			if !ok {
				continue
			}
			amountInBase := ing.ToBaseAmount(fi.Amount, fi.Unit)
			if item, exists := totals[fi.IngredientID]; exists {
				item.TotalAmount += amountInBase
			} else {
				totals[fi.IngredientID] = &Item{
					IngredientID: fi.IngredientID,
					Name:         ing.Name,
					DisplayName:  ing.DisplayName,
					TotalAmount:  amountInBase,
					Unit:         ing.BaseUnit,
					FoodGroup:    string(ing.FoodGroup),
				}
			}
		}
	}

	categories := make(map[string][]Item)
	for _, item := range totals {
		categories[item.FoodGroup] = append(categories[item.FoodGroup], *item)
	}

	total := 0
	for _, items := range categories {
		total += len(items)
	}

	return ShoppingList{Categories: categories, TotalItems: total}
}
