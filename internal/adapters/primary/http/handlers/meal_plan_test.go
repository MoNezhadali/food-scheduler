package handlers_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MoNezhadali/foodscheduler/internal/adapters/primary/http/handlers"
	"github.com/MoNezhadali/foodscheduler/internal/adapters/primary/http/middleware"
	sqliteadapter "github.com/MoNezhadali/foodscheduler/internal/adapters/secondary/sqlite"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/auth"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/database"
	"github.com/MoNezhadali/foodscheduler/migrations"
)

func setupMealPlanServer(t *testing.T) (http.Handler, auth.Service) {
	t.Helper()

	db, err := database.OpenSQLite(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.RunMigrations(db, migrations.SQLiteFS, "sqlite"))
	t.Cleanup(func() { db.Close() })

	tokenSvc := auth.NewJWTService("test-secret")
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	foodRepo := sqliteadapter.NewFoodRepo(db)

	ingHandler := handlers.NewIngredientHandler(ingRepo)
	foodHandler := handlers.NewFoodHandler(foodRepo, ingRepo)
	mpHandler := handlers.NewMealPlanHandler(foodRepo)

	r := chi.NewRouter()
	r.Use(middleware.Auth(tokenSvc))
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		r.Post("/v1/ingredients", ingHandler.Create)
		r.Post("/v1/foods", foodHandler.Create)
	})
	r.Post("/v1/meal-plan", mpHandler.Generate)
	return r, tokenSvc
}

// seedMealPlanData creates one ingredient per protein type and one food per
// type, each labelled accordingly. Returns the food IDs.
func seedMealPlanData(t *testing.T, r http.Handler, svc auth.Service) map[string]string {
	t.Helper()
	admin := adminToken(t, svc)

	proteins := []struct{ label, ingName, foodName string }{
		{"beef", "beef-ing", "beef-stew"},
		{"chicken", "chicken-ing", "chicken-soup"},
		{"fish", "fish-ing", "fish-curry"},
		{"pork", "pork-ing", "pork-ribs"},
	}

	foodIDs := make(map[string]string)
	for _, p := range proteins {
		// Create ingredient
		rr := do(t, r, http.MethodPost, "/v1/ingredients", fmt.Sprintf(
			`{"name":%q,"display_name":%q,"base_unit":"grams","food_group":"meats-and-proteins","allergens":[],"unit_map":{"grams":1}}`,
			p.ingName, p.ingName), admin)
		require.Equal(t, http.StatusCreated, rr.Code)
		var ingM map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&ingM))

		// Create food with that label
		rr = do(t, r, http.MethodPost, "/v1/foods", fmt.Sprintf(
			`{"name":%q,"display_name":%q,"labels":[%q],"ingredients":[{"ingredient_id":%q,"amount":200,"unit":"grams"}]}`,
			p.foodName, p.foodName, p.label, ingM["id"]), admin)
		require.Equal(t, http.StatusCreated, rr.Code)
		var foodM map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&foodM))
		foodIDs[p.label] = foodM["id"].(string)
	}
	return foodIDs
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestMealPlan_BasicCount(t *testing.T) {
	r, svc := setupMealPlanServer(t)
	seedMealPlanData(t, r, svc)
	user := userToken(t, svc)

	rr := do(t, r, http.MethodPost, "/v1/meal-plan", `{"count":3}`, user)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, float64(3), resp["count"])
	assert.Len(t, resp["foods"].([]any), 3)
}

func TestMealPlan_NoRepeats(t *testing.T) {
	r, svc := setupMealPlanServer(t)
	seedMealPlanData(t, r, svc)
	user := userToken(t, svc)

	rr := do(t, r, http.MethodPost, "/v1/meal-plan", `{"count":4}`, user)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	foods := resp["foods"].([]any)
	seen := make(map[any]bool)
	for _, f := range foods {
		id := f.(map[string]any)["id"]
		assert.False(t, seen[id], "food %v repeated", id)
		seen[id] = true
	}
}

func TestMealPlan_WithPreferences(t *testing.T) {
	r, svc := setupMealPlanServer(t)
	seedMealPlanData(t, r, svc) // 4 foods: beef, chicken, fish, pork
	user := userToken(t, svc)

	body := `{"count":3,"preferences":{"min_beef":1,"min_chicken":1}}`
	for range 10 { // repeat to assert constraints hold across random runs
		rr := do(t, r, http.MethodPost, "/v1/meal-plan", body, user)
		require.Equal(t, http.StatusOK, rr.Code)

		var resp map[string]any
		require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
		foods := resp["foods"].([]any)
		require.Len(t, foods, 3)

		var beefCount, chickenCount int
		for _, f := range foods {
			labels := f.(map[string]any)["labels"].([]any)
			for _, l := range labels {
				switch l.(string) {
				case "beef":
					beefCount++
				case "chicken":
					chickenCount++
				}
			}
		}
		assert.GreaterOrEqual(t, beefCount, 1, "expected ≥1 beef")
		assert.GreaterOrEqual(t, chickenCount, 1, "expected ≥1 chicken")
	}
}

func TestMealPlan_AllergenFilter(t *testing.T) {
	r, svc := setupMealPlanServer(t)
	admin := adminToken(t, svc)
	user := userToken(t, svc)

	// Create a gluten ingredient and food
	rr := do(t, r, http.MethodPost, "/v1/ingredients",
		`{"name":"wheat","display_name":"Wheat","base_unit":"grams","food_group":"grains-and-starches","allergens":["gluten"],"unit_map":{"grams":1}}`,
		admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var ingM map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&ingM))

	rr = do(t, r, http.MethodPost, "/v1/foods", fmt.Sprintf(
		`{"name":"bread","display_name":"Bread","labels":["baked"],"ingredients":[{"ingredient_id":%q,"amount":100,"unit":"grams"}]}`,
		ingM["id"]), admin)
	require.Equal(t, http.StatusCreated, rr.Code)

	// Also seed one gluten-free food
	rr = do(t, r, http.MethodPost, "/v1/ingredients",
		`{"name":"rice-mp","display_name":"Rice","base_unit":"grams","food_group":"grains-and-starches","allergens":[],"unit_map":{"grams":1}}`,
		admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var riceM map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&riceM))

	rr = do(t, r, http.MethodPost, "/v1/foods", fmt.Sprintf(
		`{"name":"rice-dish-mp","display_name":"Rice Dish","labels":["healthy"],"ingredients":[{"ingredient_id":%q,"amount":200,"unit":"grams"}]}`,
		riceM["id"]), admin)
	require.Equal(t, http.StatusCreated, rr.Code)

	// Request gluten-free meal plan of 1 → only rice-dish should be eligible
	rr = do(t, r, http.MethodPost, "/v1/meal-plan",
		`{"count":1,"allergen_free":["gluten"]}`, user)
	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	foods := resp["foods"].([]any)
	require.Len(t, foods, 1)
	assert.Equal(t, "rice-dish-mp", foods[0].(map[string]any)["name"])
}

func TestMealPlan_InsufficientFoods(t *testing.T) {
	r, svc := setupMealPlanServer(t)
	seedMealPlanData(t, r, svc) // 4 foods total
	user := userToken(t, svc)

	// Request more than available
	rr := do(t, r, http.MethodPost, "/v1/meal-plan", `{"count":10}`, user)
	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)

	var errResp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&errResp))
	assert.Equal(t, "INSUFFICIENT_FOODS", errResp["code"])
}

func TestMealPlan_CountZero(t *testing.T) {
	r, svc := setupMealPlanServer(t)
	user := userToken(t, svc)
	rr := do(t, r, http.MethodPost, "/v1/meal-plan", `{"count":0}`, user)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestMealPlan_RequiresAuth(t *testing.T) {
	r, _ := setupMealPlanServer(t)
	rr := do(t, r, http.MethodPost, "/v1/meal-plan", `{"count":1}`, "")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
