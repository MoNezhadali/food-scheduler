package sqliteadapter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MoNezhadali/foodscheduler/internal/adapters/secondary/sqlite"
	"github.com/MoNezhadali/foodscheduler/internal/domain"
	"github.com/MoNezhadali/foodscheduler/internal/domain/ingredient"
)

func newChicken() ingredient.Ingredient {
	return ingredient.Ingredient{
		Name:        "chicken-breast",
		DisplayName: "Chicken Breast",
		FoodGroup:   ingredient.FoodGroupMeatsAndProteins,
		Allergens:   []ingredient.Allergen{ingredient.AllergenPoultry},
		BaseUnit:    "grams",
		UnitMap:     ingredient.UnitMap{"grams": 1},
	}
}

func newRice() ingredient.Ingredient {
	return ingredient.Ingredient{
		Name:        "rice",
		DisplayName: "Rice",
		FoodGroup:   ingredient.FoodGroupGrainsAndStarches,
		Allergens:   nil,
		BaseUnit:    "grams",
		UnitMap:     ingredient.UnitMap{"grams": 1, "cups": 200},
	}
}

func TestIngredientRepo_CreateAndGetByID(t *testing.T) {
	repo := sqliteadapter.NewIngredientRepo(setupTestDB(t))
	ctx := context.Background()

	created, err := repo.Create(ctx, newChicken())
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "chicken-breast", created.Name)
	assert.False(t, created.CreatedAt.IsZero())

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, ingredient.FoodGroupMeatsAndProteins, got.FoodGroup)
	assert.Equal(t, []ingredient.Allergen{ingredient.AllergenPoultry}, got.Allergens)
	assert.InDelta(t, float64(1), got.UnitMap["grams"], 0.001)
}

func TestIngredientRepo_GetByName(t *testing.T) {
	repo := sqliteadapter.NewIngredientRepo(setupTestDB(t))
	ctx := context.Background()

	_, err := repo.Create(ctx, newRice())
	require.NoError(t, err)

	got, err := repo.GetByName(ctx, "rice")
	require.NoError(t, err)
	assert.Equal(t, "rice", got.Name)
}

func TestIngredientRepo_GetByID_NotFound(t *testing.T) {
	repo := sqliteadapter.NewIngredientRepo(setupTestDB(t))
	_, err := repo.GetByID(context.Background(), "nonexistent-id")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestIngredientRepo_CreateDuplicate(t *testing.T) {
	repo := sqliteadapter.NewIngredientRepo(setupTestDB(t))
	ctx := context.Background()

	_, err := repo.Create(ctx, newChicken())
	require.NoError(t, err)

	_, err = repo.Create(ctx, newChicken())
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}

func TestIngredientRepo_List_NoFilter(t *testing.T) {
	repo := sqliteadapter.NewIngredientRepo(setupTestDB(t))
	ctx := context.Background()

	_, _ = repo.Create(ctx, newChicken())
	_, _ = repo.Create(ctx, newRice())

	all, err := repo.List(ctx, ingredient.Filter{})
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestIngredientRepo_List_FoodGroupFilter(t *testing.T) {
	repo := sqliteadapter.NewIngredientRepo(setupTestDB(t))
	ctx := context.Background()

	_, _ = repo.Create(ctx, newChicken())
	_, _ = repo.Create(ctx, newRice())

	fg := ingredient.FoodGroupGrainsAndStarches
	result, err := repo.List(ctx, ingredient.Filter{FoodGroup: &fg})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "rice", result[0].Name)
}

func TestIngredientRepo_List_AllergenFreeFilter(t *testing.T) {
	repo := sqliteadapter.NewIngredientRepo(setupTestDB(t))
	ctx := context.Background()

	_, _ = repo.Create(ctx, newChicken()) // has poultry allergen
	_, _ = repo.Create(ctx, newRice())    // no allergens

	result, err := repo.List(ctx, ingredient.Filter{AllergenFree: []ingredient.Allergen{ingredient.AllergenPoultry}})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "rice", result[0].Name)
}

func TestIngredientRepo_Update(t *testing.T) {
	repo := sqliteadapter.NewIngredientRepo(setupTestDB(t))
	ctx := context.Background()

	created, err := repo.Create(ctx, newChicken())
	require.NoError(t, err)

	created.DisplayName = "Chicken Breast (Updated)"
	updated, err := repo.Update(ctx, created)
	require.NoError(t, err)
	assert.Equal(t, "Chicken Breast (Updated)", updated.DisplayName)
	assert.True(t, updated.UpdatedAt.After(updated.CreatedAt) || updated.UpdatedAt.Equal(updated.CreatedAt))
}

func TestIngredientRepo_Delete(t *testing.T) {
	repo := sqliteadapter.NewIngredientRepo(setupTestDB(t))
	ctx := context.Background()

	created, err := repo.Create(ctx, newChicken())
	require.NoError(t, err)

	err = repo.Delete(ctx, created.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, created.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestIngredientRepo_Delete_NotFound(t *testing.T) {
	repo := sqliteadapter.NewIngredientRepo(setupTestDB(t))
	err := repo.Delete(context.Background(), "nonexistent-id")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestIngredientRepo_UpdateNutrition(t *testing.T) {
	repo := sqliteadapter.NewIngredientRepo(setupTestDB(t))
	ctx := context.Background()

	created, err := repo.Create(ctx, newChicken())
	require.NoError(t, err)
	assert.Nil(t, created.Nutrition.CaloriesPerBase)

	cal := 1.65
	err = repo.UpdateNutrition(ctx, created.ID, ingredient.NutritionInfo{
		CaloriesPerBase: &cal,
	})
	require.NoError(t, err)

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, got.Nutrition.CaloriesPerBase)
	assert.InDelta(t, 1.65, *got.Nutrition.CaloriesPerBase, 0.001)
}

func TestIngredientRepo_ListMissingNutrition(t *testing.T) {
	repo := sqliteadapter.NewIngredientRepo(setupTestDB(t))
	ctx := context.Background()

	chicken, _ := repo.Create(ctx, newChicken())
	_, _ = repo.Create(ctx, newRice())

	// enrich chicken only
	cal := 1.65
	_ = repo.UpdateNutrition(ctx, chicken.ID, ingredient.NutritionInfo{CaloriesPerBase: &cal})

	missing, err := repo.ListMissingNutrition(ctx)
	require.NoError(t, err)
	assert.Len(t, missing, 1)
	assert.Equal(t, "rice", missing[0].Name)
}
