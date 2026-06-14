package food

import "github.com/MoNezhadali/foodscheduler/internal/domain/ingredient"

// ComputeNutrition calculates total and per-portion nutrition for a food
// from its ingredients and their amounts. Ingredients with unknown nutrition
// are skipped; the totals reflect only what could be computed.
func ComputeNutrition(f Food, ingredients map[string]ingredient.Ingredient) NutritionInfo {
	var info NutritionInfo
	for _, fi := range f.Ingredients {
		ing, ok := ingredients[fi.IngredientID]
		if !ok {
			continue
		}
		base := ing.ToBaseAmount(fi.Amount, fi.Unit)
		if v := ing.Nutrition.CaloriesPerBase; v != nil {
			info.CaloriesTotal += *v * base
		}
		if v := ing.Nutrition.ProteinPerBase; v != nil {
			info.ProteinTotal += *v * base
		}
		if v := ing.Nutrition.CarbsPerBase; v != nil {
			info.CarbsTotal += *v * base
		}
		if v := ing.Nutrition.FatPerBase; v != nil {
			info.FatTotal += *v * base
		}
	}
	if f.Portions > 0 {
		p := float64(f.Portions)
		info.CaloriesPerPortion = info.CaloriesTotal / p
		info.ProteinPerPortion = info.ProteinTotal / p
		info.CarbsPerPortion = info.CarbsTotal / p
		info.FatPerPortion = info.FatTotal / p
	}
	return info
}
