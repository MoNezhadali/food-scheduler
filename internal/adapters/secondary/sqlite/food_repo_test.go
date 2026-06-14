package sqliteadapter_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqliteadapter "github.com/MoNezhadali/foodscheduler/internal/adapters/secondary/sqlite"
	"github.com/MoNezhadali/foodscheduler/internal/domain"
	"github.com/MoNezhadali/foodscheduler/internal/domain/food"
	"github.com/MoNezhadali/foodscheduler/internal/domain/ingredient"
)

// seedIngredients creates chicken and rice in the DB and returns their IDs.
func seedIngredients(t *testing.T, repo *sqliteadapter.IngredientRepo) (chickenID, riceID string) {
	t.Helper()
	ctx := context.Background()
	c, err := repo.Create(ctx, newChicken())
	require.NoError(t, err)
	r, err := repo.Create(ctx, newRice())
	require.NoError(t, err)
	return c.ID, r.ID
}

func newChineseFood(chickenID, riceID string) food.Food {
	return food.Food{
		Name:        "chinese-meal",
		DisplayName: "Chinese Meal",
		Description: "A quick stir-fry",
		Portions:    4,
		Recipe:      []string{"Cook rice", "Stir-fry chicken"},
		Labels:      []string{"main-course", "poultry"},
		Ingredients: []food.FoodIngredient{
			{IngredientID: chickenID, Amount: 500, Unit: "grams"},
			{IngredientID: riceID, Amount: 2, Unit: "cups"},
		},
	}
}

func TestFoodRepo_CreateAndGetByID(t *testing.T) {
	db := setupTestDB(t)
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	foodRepo := sqliteadapter.NewFoodRepo(db)
	ctx := context.Background()

	chickenID, riceID := seedIngredients(t, ingRepo)
	created, err := foodRepo.Create(ctx, newChineseFood(chickenID, riceID))
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "chinese-meal", created.Name)
	assert.Equal(t, 4, created.Portions)

	got, err := foodRepo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, []string{"main-course", "poultry"}, got.Labels)
	assert.Len(t, got.Ingredients, 2)

	var chickenFI food.FoodIngredient
	for _, fi := range got.Ingredients {
		if fi.IngredientID == chickenID {
			chickenFI = fi
		}
	}
	assert.InDelta(t, 500.0, chickenFI.Amount, 0.001)
	assert.Equal(t, "grams", chickenFI.Unit)
}

func TestFoodRepo_GetByID_NotFound(t *testing.T) {
	repo := sqliteadapter.NewFoodRepo(setupTestDB(t))
	_, err := repo.GetByID(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestFoodRepo_CreateDuplicate(t *testing.T) {
	db := setupTestDB(t)
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	foodRepo := sqliteadapter.NewFoodRepo(db)
	ctx := context.Background()

	chickenID, riceID := seedIngredients(t, ingRepo)
	_, err := foodRepo.Create(ctx, newChineseFood(chickenID, riceID))
	require.NoError(t, err)

	_, err = foodRepo.Create(ctx, newChineseFood(chickenID, riceID))
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}

func TestFoodRepo_List_NoFilter(t *testing.T) {
	db := setupTestDB(t)
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	foodRepo := sqliteadapter.NewFoodRepo(db)
	ctx := context.Background()

	chickenID, riceID := seedIngredients(t, ingRepo)
	_, _ = foodRepo.Create(ctx, newChineseFood(chickenID, riceID))

	vegFood := food.Food{
		Name: "salad", DisplayName: "Salad", Portions: 2,
		Recipe: []string{"Mix"}, Labels: []string{"vegetarian"},
		Ingredients: []food.FoodIngredient{{IngredientID: riceID, Amount: 100, Unit: "grams"}},
	}
	_, _ = foodRepo.Create(ctx, vegFood)

	all, err := foodRepo.List(ctx, food.Filter{})
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestFoodRepo_List_LabelFilter(t *testing.T) {
	db := setupTestDB(t)
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	foodRepo := sqliteadapter.NewFoodRepo(db)
	ctx := context.Background()

	chickenID, riceID := seedIngredients(t, ingRepo)
	_, _ = foodRepo.Create(ctx, newChineseFood(chickenID, riceID))

	vegFood := food.Food{
		Name: "salad", DisplayName: "Salad", Portions: 2,
		Recipe: []string{"Mix"}, Labels: []string{"vegetarian"},
		Ingredients: []food.FoodIngredient{{IngredientID: riceID, Amount: 100, Unit: "grams"}},
	}
	_, _ = foodRepo.Create(ctx, vegFood)

	result, err := foodRepo.List(ctx, food.Filter{Labels: []string{"vegetarian"}})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "salad", result[0].Name)
}

func TestFoodRepo_List_AllergenExclusion(t *testing.T) {
	db := setupTestDB(t)
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	foodRepo := sqliteadapter.NewFoodRepo(db)
	ctx := context.Background()

	chickenID, riceID := seedIngredients(t, ingRepo)
	_, _ = foodRepo.Create(ctx, newChineseFood(chickenID, riceID)) // has poultry

	vegFood := food.Food{
		Name: "rice-bowl", DisplayName: "Rice Bowl", Portions: 2,
		Recipe: []string{"Cook"}, Labels: []string{"vegetarian"},
		Ingredients: []food.FoodIngredient{{IngredientID: riceID, Amount: 300, Unit: "grams"}},
	}
	_, _ = foodRepo.Create(ctx, vegFood)

	result, err := foodRepo.List(ctx, food.Filter{ExcludeAllergens: []string{string(ingredient.AllergenPoultry)}})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "rice-bowl", result[0].Name)
}

func TestFoodRepo_Update(t *testing.T) {
	db := setupTestDB(t)
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	foodRepo := sqliteadapter.NewFoodRepo(db)
	ctx := context.Background()

	chickenID, riceID := seedIngredients(t, ingRepo)
	created, err := foodRepo.Create(ctx, newChineseFood(chickenID, riceID))
	require.NoError(t, err)

	created.DisplayName = "Chinese Stir-Fry"
	created.Portions = 6
	// Change ingredients: only chicken, remove rice
	created.Ingredients = []food.FoodIngredient{
		{IngredientID: chickenID, Amount: 800, Unit: "grams"},
	}

	updated, err := foodRepo.Update(ctx, created)
	require.NoError(t, err)
	assert.Equal(t, "Chinese Stir-Fry", updated.DisplayName)
	assert.Equal(t, 6, updated.Portions)

	got, err := foodRepo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Len(t, got.Ingredients, 1)
	assert.Equal(t, chickenID, got.Ingredients[0].IngredientID)
	assert.InDelta(t, 800.0, got.Ingredients[0].Amount, 0.001)
}

func TestFoodRepo_Delete(t *testing.T) {
	db := setupTestDB(t)
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	foodRepo := sqliteadapter.NewFoodRepo(db)
	ctx := context.Background()

	chickenID, riceID := seedIngredients(t, ingRepo)
	created, err := foodRepo.Create(ctx, newChineseFood(chickenID, riceID))
	require.NoError(t, err)

	err = foodRepo.Delete(ctx, created.ID)
	require.NoError(t, err)

	_, err = foodRepo.GetByID(ctx, created.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestFoodRepo_Delete_CascadesFoodIngredients(t *testing.T) {
	db := setupTestDB(t)
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	foodRepo := sqliteadapter.NewFoodRepo(db)
	ctx := context.Background()

	chickenID, riceID := seedIngredients(t, ingRepo)
	created, _ := foodRepo.Create(ctx, newChineseFood(chickenID, riceID))
	_ = foodRepo.Delete(ctx, created.ID)

	// Ingredients should still exist after food deletion
	_, err := ingRepo.GetByID(ctx, chickenID)
	assert.NoError(t, err)
}

func TestFoodRepo_GetByIDs(t *testing.T) {
	db := setupTestDB(t)
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	foodRepo := sqliteadapter.NewFoodRepo(db)
	ctx := context.Background()

	chickenID, riceID := seedIngredients(t, ingRepo)
	f1, _ := foodRepo.Create(ctx, newChineseFood(chickenID, riceID))
	f2, _ := foodRepo.Create(ctx, food.Food{
		Name: "plain-rice", DisplayName: "Plain Rice", Portions: 2,
		Recipe: []string{"Boil"}, Labels: []string{"vegetarian"},
		Ingredients: []food.FoodIngredient{{IngredientID: riceID, Amount: 300, Unit: "grams"}},
	})

	foods, err := foodRepo.GetByIDs(ctx, []string{f1.ID, f2.ID})
	require.NoError(t, err)
	assert.Len(t, foods, 2)
}
