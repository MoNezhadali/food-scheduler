package food_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/MoNezhadali/foodscheduler/internal/domain/food"
	"github.com/MoNezhadali/foodscheduler/internal/domain/ingredient"
)

func fptr(f float64) *float64 { return &f }

// makeIngredient builds a test ingredient with full nutrition.
func makeIngredient(id, baseUnit string, unitMap ingredient.UnitMap, cal, prot, carb, fat float64) ingredient.Ingredient {
	return ingredient.Ingredient{
		ID:       id,
		BaseUnit: baseUnit,
		UnitMap:  unitMap,
		Nutrition: ingredient.NutritionInfo{
			CaloriesPerBase: fptr(cal),
			ProteinPerBase:  fptr(prot),
			CarbsPerBase:    fptr(carb),
			FatPerBase:      fptr(fat),
		},
	}
}

func TestComputeNutrition_AllKnown(t *testing.T) {
	// chicken breast: 1.65 kcal/g, 0.31g protein/g, 0g carbs, 0.036g fat/g
	// rice: 1.30 kcal/g, 0.027g protein/g, 0.28g carbs/g, 0.003g fat/g
	chicken := makeIngredient("chicken-id", "grams",
		ingredient.UnitMap{"grams": 1},
		1.65, 0.31, 0.0, 0.036)

	rice := makeIngredient("rice-id", "grams",
		ingredient.UnitMap{"grams": 1, "cup": 200, "cups": 200},
		1.30, 0.027, 0.28, 0.003)

	f := food.Food{
		Portions: 4,
		Ingredients: []food.FoodIngredient{
			{IngredientID: "chicken-id", Amount: 500, Unit: "grams"}, // 500g
			{IngredientID: "rice-id", Amount: 2, Unit: "cups"},       // 2 cups = 400g
		},
	}
	ingMap := map[string]ingredient.Ingredient{
		"chicken-id": chicken,
		"rice-id":    rice,
	}

	info := food.ComputeNutrition(f, ingMap)

	// calories: 500*1.65 + 400*1.30 = 825 + 520 = 1345
	assert.InDelta(t, 1345.0, info.CaloriesTotal, 0.01)
	assert.InDelta(t, 1345.0/4, info.CaloriesPerPortion, 0.01)

	// protein: 500*0.31 + 400*0.027 = 155 + 10.8 = 165.8
	assert.InDelta(t, 165.8, info.ProteinTotal, 0.01)
	assert.InDelta(t, 165.8/4, info.ProteinPerPortion, 0.01)

	// carbs: 500*0 + 400*0.28 = 112
	assert.InDelta(t, 112.0, info.CarbsTotal, 0.01)

	// fat: 500*0.036 + 400*0.003 = 18 + 1.2 = 19.2
	assert.InDelta(t, 19.2, info.FatTotal, 0.01)
}

func TestComputeNutrition_PartialNutrition(t *testing.T) {
	// rice has full nutrition; spice has no nutrition data (all nil)
	rice := makeIngredient("rice-id", "grams",
		ingredient.UnitMap{"grams": 1},
		1.30, 0.027, 0.28, 0.003)

	spice := ingredient.Ingredient{
		ID:       "salt-id",
		BaseUnit: "grams",
		UnitMap:  ingredient.UnitMap{"grams": 1},
		Nutrition: ingredient.NutritionInfo{
			// all nil — not yet enriched
		},
	}

	f := food.Food{
		Portions: 2,
		Ingredients: []food.FoodIngredient{
			{IngredientID: "rice-id", Amount: 300, Unit: "grams"},
			{IngredientID: "salt-id", Amount: 5, Unit: "grams"},
		},
	}
	ingMap := map[string]ingredient.Ingredient{
		"rice-id": rice,
		"salt-id": spice,
	}

	info := food.ComputeNutrition(f, ingMap)

	// only rice contributes: 300*1.30 = 390 kcal
	assert.InDelta(t, 390.0, info.CaloriesTotal, 0.01)
	assert.InDelta(t, 195.0, info.CaloriesPerPortion, 0.01)
	// rice fat: 300*0.003 = 0.9; salt's nil fat field does not add anything
	assert.InDelta(t, 0.9, info.FatTotal, 0.01)
}

func TestComputeNutrition_UnknownIngredientSkipped(t *testing.T) {
	rice := makeIngredient("rice-id", "grams",
		ingredient.UnitMap{"grams": 1},
		1.30, 0.027, 0.28, 0.003)

	f := food.Food{
		Portions: 2,
		Ingredients: []food.FoodIngredient{
			{IngredientID: "rice-id", Amount: 200, Unit: "grams"},
			{IngredientID: "unknown-id", Amount: 100, Unit: "grams"}, // not in map
		},
	}
	ingMap := map[string]ingredient.Ingredient{"rice-id": rice}

	info := food.ComputeNutrition(f, ingMap)

	// only rice: 200*1.30 = 260 kcal
	assert.InDelta(t, 260.0, info.CaloriesTotal, 0.01)
}

func TestComputeNutrition_ZeroPortionsNoPanic(t *testing.T) {
	rice := makeIngredient("rice-id", "grams",
		ingredient.UnitMap{"grams": 1},
		1.30, 0.027, 0.28, 0.003)

	f := food.Food{
		Portions:    0, // degenerate case: no div-by-zero
		Ingredients: []food.FoodIngredient{{IngredientID: "rice-id", Amount: 200, Unit: "grams"}},
	}

	info := food.ComputeNutrition(f, map[string]ingredient.Ingredient{"rice-id": rice})

	assert.InDelta(t, 260.0, info.CaloriesTotal, 0.01)
	assert.Equal(t, 0.0, info.CaloriesPerPortion) // stays zero, no panic
}

func TestComputeNutrition_UnitConversion(t *testing.T) {
	// olive oil: base unit grams, 1 tablespoon = 14g
	oil := makeIngredient("oil-id", "grams",
		ingredient.UnitMap{"grams": 1, "tablespoon": 14, "tablespoons": 14},
		8.84, 0.0, 0.0, 1.0)

	f := food.Food{
		Portions:    1,
		Ingredients: []food.FoodIngredient{{IngredientID: "oil-id", Amount: 3, Unit: "tablespoons"}},
	}

	info := food.ComputeNutrition(f, map[string]ingredient.Ingredient{"oil-id": oil})

	// 3 tbsp = 42g; 42 * 8.84 = 371.28 kcal
	assert.InDelta(t, 371.28, info.CaloriesTotal, 0.01)
}
