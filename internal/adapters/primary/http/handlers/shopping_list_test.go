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

// setupShoppingListServer returns a router that includes ingredient, food, and
// shopping-list routes so we can seed data through the API.
func setupShoppingListServer(t *testing.T) (http.Handler, auth.Service) {
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
	slHandler := handlers.NewShoppingListHandler(foodRepo, ingRepo)

	r := chi.NewRouter()
	r.Use(middleware.Auth(tokenSvc))
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		r.Post("/v1/ingredients", ingHandler.Create)
		r.Post("/v1/foods", foodHandler.Create)
	})
	r.Post("/v1/shopping-list", slHandler.Generate)
	return r, tokenSvc
}

// seedTestData creates two ingredients and two foods, returns their IDs.
func seedTestData(t *testing.T, r http.Handler, svc auth.Service) (riceID, chickenID, food1ID, food2ID string) {
	t.Helper()
	admin := adminToken(t, svc)

	rr := do(t, r, http.MethodPost, "/v1/ingredients",
		`{"name":"rice-sl","display_name":"Rice","base_unit":"grams","food_group":"grains-and-starches","allergens":[],"unit_map":{"grams":1},"nutrition":{"calories_per_base":3.5}}`,
		admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var rm map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&rm))
	riceID = rm["id"].(string)

	rr = do(t, r, http.MethodPost, "/v1/ingredients",
		`{"name":"chicken-sl","display_name":"Chicken","base_unit":"grams","food_group":"meats-and-proteins","allergens":["poultry"],"unit_map":{"grams":1},"nutrition":{"calories_per_base":1.65}}`,
		admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var cm map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&cm))
	chickenID = cm["id"].(string)

	// Food 1: rice + chicken
	rr = do(t, r, http.MethodPost, "/v1/foods", fmt.Sprintf(
		`{"name":"rice-chicken","display_name":"Rice Chicken","ingredients":[{"ingredient_id":%q,"amount":200,"unit":"grams"},{"ingredient_id":%q,"amount":300,"unit":"grams"}]}`,
		riceID, chickenID), admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var f1 map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&f1))
	food1ID = f1["id"].(string)

	// Food 2: just rice
	rr = do(t, r, http.MethodPost, "/v1/foods", fmt.Sprintf(
		`{"name":"plain-rice","display_name":"Plain Rice","ingredients":[{"ingredient_id":%q,"amount":150,"unit":"grams"}]}`,
		riceID), admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var f2 map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&f2))
	food2ID = f2["id"].(string)

	return
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestShoppingList_Generate(t *testing.T) {
	r, svc := setupShoppingListServer(t)
	riceID, chickenID, food1ID, food2ID := seedTestData(t, r, svc)
	user := userToken(t, svc)

	// Request shopping list for both foods
	body := fmt.Sprintf(`{"food_ids":[%q,%q]}`, food1ID, food2ID)
	rr := do(t, r, http.MethodPost, "/v1/shopping-list", body, user)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))

	// total_items: rice + chicken = 2 unique ingredients
	assert.Equal(t, float64(2), resp["total_items"])

	cats := resp["categories"].(map[string]any)

	// grains-and-starches: rice appears in both foods → 200+150 = 350g
	grains := cats["grains-and-starches"].([]any)
	assert.Len(t, grains, 1)
	grain := grains[0].(map[string]any)
	assert.Equal(t, riceID, grain["ingredient_id"])
	assert.InDelta(t, 350.0, grain["total_amount"], 0.001)
	assert.Equal(t, "grams", grain["unit"])

	// meats-and-proteins: chicken only in food1 → 300g
	meats := cats["meats-and-proteins"].([]any)
	assert.Len(t, meats, 1)
	meat := meats[0].(map[string]any)
	assert.Equal(t, chickenID, meat["ingredient_id"])
	assert.InDelta(t, 300.0, meat["total_amount"], 0.001)
}

func TestShoppingList_SingleFood(t *testing.T) {
	r, svc := setupShoppingListServer(t)
	_, _, food1ID, _ := seedTestData(t, r, svc)
	user := userToken(t, svc)

	body := fmt.Sprintf(`{"food_ids":[%q]}`, food1ID)
	rr := do(t, r, http.MethodPost, "/v1/shopping-list", body, user)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, float64(2), resp["total_items"]) // rice + chicken
}

func TestShoppingList_EmptyFoodIDs(t *testing.T) {
	r, svc := setupShoppingListServer(t)
	user := userToken(t, svc)

	rr := do(t, r, http.MethodPost, "/v1/shopping-list", `{"food_ids":[]}`, user)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestShoppingList_RequiresAuth(t *testing.T) {
	r, _ := setupShoppingListServer(t)
	rr := do(t, r, http.MethodPost, "/v1/shopping-list", `{"food_ids":["x"]}`, "")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestShoppingList_UnknownFoodIDsReturnEmptyList(t *testing.T) {
	r, svc := setupShoppingListServer(t)
	user := userToken(t, svc)

	rr := do(t, r, http.MethodPost, "/v1/shopping-list", `{"food_ids":["no-such-id"]}`, user)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, float64(0), resp["total_items"])
}
