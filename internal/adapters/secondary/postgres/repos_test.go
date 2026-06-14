package pgadapter_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pgadapter "github.com/MoNezhadali/foodscheduler/internal/adapters/secondary/postgres"
	"github.com/MoNezhadali/foodscheduler/internal/domain"
	"github.com/MoNezhadali/foodscheduler/internal/domain/food"
	"github.com/MoNezhadali/foodscheduler/internal/domain/ingredient"
	"github.com/MoNezhadali/foodscheduler/internal/domain/user"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/database"
	"github.com/MoNezhadali/foodscheduler/migrations"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_PG_URL")
	if url == "" {
		t.Skip("TEST_PG_URL not set; skipping PostgreSQL integration tests")
	}
	db, err := database.OpenPostgres(url)
	require.NoError(t, err)
	require.NoError(t, database.RunMigrations(db, migrations.PostgresFS, "postgres"))
	t.Cleanup(func() {
		// Tear down in dependency order so FK constraints are satisfied.
		db.Exec("DELETE FROM food_ingredients") //nolint:errcheck
		db.Exec("DELETE FROM foods")            //nolint:errcheck
		db.Exec("DELETE FROM user_preferences") //nolint:errcheck
		db.Exec("DELETE FROM users")            //nolint:errcheck
		db.Exec("DELETE FROM ingredients")      //nolint:errcheck
		db.Close()
	})
	return db
}

// ── IngredientRepo ────────────────────────────────────────────────────────────

func TestIngredientRepo_CRUD(t *testing.T) {
	db := setupTestDB(t)
	repo := pgadapter.NewIngredientRepo(db)
	ctx := context.Background()

	cal := 350.0
	ing := ingredient.Ingredient{
		Name:        "pg-rice",
		DisplayName: "PG Rice",
		FoodGroup:   ingredient.FoodGroupGrainsAndStarches,
		BaseUnit:    "grams",
		UnitMap:     ingredient.UnitMap{"cups": 200},
		Nutrition:   ingredient.NutritionInfo{CaloriesPerBase: &cal},
	}

	created, err := repo.Create(ctx, ing)
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "pg-rice", got.Name)
	assert.InDelta(t, 350.0, *got.Nutrition.CaloriesPerBase, 0.01)

	got.DisplayName = "PG Rice Updated"
	updated, err := repo.Update(ctx, got)
	require.NoError(t, err)
	assert.Equal(t, "PG Rice Updated", updated.DisplayName)

	err = repo.Delete(ctx, created.ID)
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, created.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestIngredientRepo_DuplicateName(t *testing.T) {
	db := setupTestDB(t)
	repo := pgadapter.NewIngredientRepo(db)
	ctx := context.Background()

	ing := ingredient.Ingredient{
		Name: "pg-unique-ing", DisplayName: "D",
		FoodGroup: ingredient.FoodGroupDairy, BaseUnit: "ml",
		UnitMap: ingredient.UnitMap{"ml": 1},
	}
	_, err := repo.Create(ctx, ing)
	require.NoError(t, err)

	_, err = repo.Create(ctx, ing)
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}

func TestIngredientRepo_Filter(t *testing.T) {
	db := setupTestDB(t)
	repo := pgadapter.NewIngredientRepo(db)
	ctx := context.Background()

	_, err := repo.Create(ctx, ingredient.Ingredient{
		Name: "pg-wheat", DisplayName: "Wheat", FoodGroup: ingredient.FoodGroupGrainsAndStarches,
		BaseUnit: "grams", UnitMap: ingredient.UnitMap{"g": 1},
		Allergens: []ingredient.Allergen{ingredient.AllergenGluten},
	})
	require.NoError(t, err)

	fg := ingredient.FoodGroupGrainsAndStarches
	all, err := repo.List(ctx, ingredient.Filter{FoodGroup: &fg})
	require.NoError(t, err)
	assert.NotEmpty(t, all)

	allergenFree, err := repo.List(ctx, ingredient.Filter{AllergenFree: []ingredient.Allergen{ingredient.AllergenGluten}})
	require.NoError(t, err)
	for _, ing := range allergenFree {
		for _, a := range ing.Allergens {
			assert.NotEqual(t, ingredient.AllergenGluten, a)
		}
	}
}

// ── FoodRepo ──────────────────────────────────────────────────────────────────

func TestFoodRepo_CRUD(t *testing.T) {
	db := setupTestDB(t)
	ingRepo := pgadapter.NewIngredientRepo(db)
	foodRepo := pgadapter.NewFoodRepo(db)
	ctx := context.Background()

	rice, err := ingRepo.Create(ctx, ingredient.Ingredient{
		Name: "pg-food-rice", DisplayName: "Rice", FoodGroup: ingredient.FoodGroupGrainsAndStarches,
		BaseUnit: "grams", UnitMap: ingredient.UnitMap{"g": 1},
	})
	require.NoError(t, err)

	f := food.Food{
		Name: "pg-fried-rice", DisplayName: "Fried Rice", Portions: 2,
		Recipe: []string{"Cook rice"}, Labels: []string{"quick"},
		Ingredients: []food.FoodIngredient{{IngredientID: rice.ID, Amount: 200, Unit: "g"}},
	}

	created, err := foodRepo.Create(ctx, f)
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Len(t, created.Ingredients, 1)

	got, err := foodRepo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "pg-fried-rice", got.Name)

	got.DisplayName = "Fried Rice Updated"
	updated, err := foodRepo.Update(ctx, got)
	require.NoError(t, err)
	assert.Equal(t, "Fried Rice Updated", updated.DisplayName)

	err = foodRepo.Delete(ctx, created.ID)
	require.NoError(t, err)

	_, err = foodRepo.GetByID(ctx, created.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ── UserRepo ──────────────────────────────────────────────────────────────────

func TestUserRepo_CRUD(t *testing.T) {
	db := setupTestDB(t)
	repo := pgadapter.NewUserRepo(db)
	ctx := context.Background()

	u := user.User{Email: "pg@example.com", PasswordHash: "hashed", Role: "user"}
	created, err := repo.Create(ctx, u)
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)

	got, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "pg@example.com", got.Email)
	assert.Empty(t, got.Preferences.ExcludedAllergens)

	err = repo.UpdatePreferences(ctx, created.ID, user.Preferences{
		ExcludedAllergens:   []string{"gluten"},
		DietaryRestrictions: []string{"vegan"},
	})
	require.NoError(t, err)

	updated, err := repo.GetByEmail(ctx, "pg@example.com")
	require.NoError(t, err)
	assert.Equal(t, []string{"gluten"}, updated.Preferences.ExcludedAllergens)
}

func TestUserRepo_DuplicateEmail(t *testing.T) {
	db := setupTestDB(t)
	repo := pgadapter.NewUserRepo(db)
	ctx := context.Background()

	u := user.User{Email: "pg-dup@example.com", PasswordHash: "hash", Role: "user"}
	_, err := repo.Create(ctx, u)
	require.NoError(t, err)

	_, err = repo.Create(ctx, u)
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}
