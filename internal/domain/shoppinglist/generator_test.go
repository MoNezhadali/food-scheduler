package shoppinglist_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MoNezhadali/foodscheduler/internal/domain/food"
	"github.com/MoNezhadali/foodscheduler/internal/domain/ingredient"
	"github.com/MoNezhadali/foodscheduler/internal/domain/shoppinglist"
)

var (
	chicken = ingredient.Ingredient{
		ID: "chicken-id", Name: "chicken-breast", DisplayName: "Chicken Breast",
		FoodGroup: ingredient.FoodGroupMeatsAndProteins,
		BaseUnit:  "grams",
		UnitMap:   ingredient.UnitMap{"grams": 1},
	}
	rice = ingredient.Ingredient{
		ID: "rice-id", Name: "rice", DisplayName: "Rice",
		FoodGroup: ingredient.FoodGroupGrainsAndStarches,
		BaseUnit:  "grams",
		UnitMap:   ingredient.UnitMap{"grams": 1, "cup": 200, "cups": 200},
	}
	oil = ingredient.Ingredient{
		ID: "oil-id", Name: "olive-oil", DisplayName: "Olive Oil",
		FoodGroup: ingredient.FoodGroupOilsAndFats,
		BaseUnit:  "grams",
		UnitMap:   ingredient.UnitMap{"grams": 1, "tablespoon": 14, "tablespoons": 14},
	}
)

func ingMap(ings ...ingredient.Ingredient) map[string]ingredient.Ingredient {
	m := make(map[string]ingredient.Ingredient, len(ings))
	for _, i := range ings {
		m[i.ID] = i
	}
	return m
}

func TestGenerate_SingleFood(t *testing.T) {
	foods := []food.Food{
		{
			Ingredients: []food.FoodIngredient{
				{IngredientID: "chicken-id", Amount: 500, Unit: "grams"},
				{IngredientID: "rice-id", Amount: 2, Unit: "cups"}, // 400g
			},
		},
	}

	list := shoppinglist.Generate(foods, ingMap(chicken, rice))

	assert.Equal(t, 2, list.TotalItems)

	meats := list.Categories[string(ingredient.FoodGroupMeatsAndProteins)]
	require.Len(t, meats, 1)
	assert.InDelta(t, 500.0, meats[0].TotalAmount, 0.01)
	assert.Equal(t, "grams", meats[0].Unit)

	grains := list.Categories[string(ingredient.FoodGroupGrainsAndStarches)]
	require.Len(t, grains, 1)
	assert.InDelta(t, 400.0, grains[0].TotalAmount, 0.01) // 2 cups × 200g
}

func TestGenerate_SharedIngredientAcrossFoods(t *testing.T) {
	// two foods both use rice — amounts should be summed
	foods := []food.Food{
		{Ingredients: []food.FoodIngredient{{IngredientID: "rice-id", Amount: 300, Unit: "grams"}}},
		{Ingredients: []food.FoodIngredient{{IngredientID: "rice-id", Amount: 1, Unit: "cup"}}}, // 200g
	}

	list := shoppinglist.Generate(foods, ingMap(rice))

	grains := list.Categories[string(ingredient.FoodGroupGrainsAndStarches)]
	require.Len(t, grains, 1)
	assert.InDelta(t, 500.0, grains[0].TotalAmount, 0.01) // 300 + 200
	assert.Equal(t, 1, list.TotalItems)
}

func TestGenerate_GroupedByFoodGroup(t *testing.T) {
	foods := []food.Food{
		{
			Ingredients: []food.FoodIngredient{
				{IngredientID: "chicken-id", Amount: 400, Unit: "grams"},
				{IngredientID: "rice-id", Amount: 200, Unit: "grams"},
				{IngredientID: "oil-id", Amount: 2, Unit: "tablespoons"}, // 28g
			},
		},
	}

	list := shoppinglist.Generate(foods, ingMap(chicken, rice, oil))

	assert.Equal(t, 3, list.TotalItems)
	assert.Len(t, list.Categories[string(ingredient.FoodGroupMeatsAndProteins)], 1)
	assert.Len(t, list.Categories[string(ingredient.FoodGroupGrainsAndStarches)], 1)
	assert.Len(t, list.Categories[string(ingredient.FoodGroupOilsAndFats)], 1)

	oilItem := list.Categories[string(ingredient.FoodGroupOilsAndFats)][0]
	assert.InDelta(t, 28.0, oilItem.TotalAmount, 0.01)
}

func TestGenerate_UnknownIngredientSkipped(t *testing.T) {
	foods := []food.Food{
		{
			Ingredients: []food.FoodIngredient{
				{IngredientID: "rice-id", Amount: 200, Unit: "grams"},
				{IngredientID: "unknown-id", Amount: 100, Unit: "grams"},
			},
		},
	}

	list := shoppinglist.Generate(foods, ingMap(rice)) // unknown not in map

	assert.Equal(t, 1, list.TotalItems)
}

func TestGenerate_EmptyFoods(t *testing.T) {
	list := shoppinglist.Generate([]food.Food{}, ingMap(chicken, rice))

	assert.Equal(t, 0, list.TotalItems)
	assert.Empty(t, list.Categories)
}
