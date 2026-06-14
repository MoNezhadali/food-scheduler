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

// ── test helpers ──────────────────────────────────────────────────────────────

func setupFoodServer(t *testing.T) (http.Handler, auth.Service, *sqliteadapter.IngredientRepo) {
	t.Helper()

	db, err := database.OpenSQLite(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.RunMigrations(db, migrations.SQLiteFS, "sqlite"))
	t.Cleanup(func() { db.Close() })

	tokenSvc := auth.NewJWTService("test-secret")
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	foodRepo := sqliteadapter.NewFoodRepo(db)
	foodHandler := handlers.NewFoodHandler(foodRepo, ingRepo)

	r := chi.NewRouter()
	r.Use(middleware.Auth(tokenSvc))
	r.Route("/v1/foods", func(r chi.Router) {
		r.Get("/", foodHandler.List)
		r.Get("/{id}", foodHandler.GetByID)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))
			r.Post("/", foodHandler.Create)
			r.Put("/{id}", foodHandler.Update)
			r.Delete("/{id}", foodHandler.Delete)
		})
	})
	return r, tokenSvc, ingRepo
}

// seedIngredient inserts an ingredient and returns its ID.
func seedIngredient(t *testing.T, handler http.Handler, svc auth.Service, body string) string {
	t.Helper()
	rr := do(t, handler, http.MethodPost, "/v1/ingredients", body, adminToken(t, svc))
	require.Equal(t, http.StatusCreated, rr.Code)
	var m map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&m))
	return m["id"].(string)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestFood_CreateAndGetByID(t *testing.T) {
	// Build a server that has both ingredient and food routes so we can seed
	db, err := database.OpenSQLite(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.RunMigrations(db, migrations.SQLiteFS, "sqlite"))
	t.Cleanup(func() { db.Close() })

	tokenSvc := auth.NewJWTService("test-secret")
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	foodRepo := sqliteadapter.NewFoodRepo(db)

	ingHandler := handlers.NewIngredientHandler(ingRepo)
	foodHandler := handlers.NewFoodHandler(foodRepo, ingRepo)

	r := chi.NewRouter()
	r.Use(middleware.Auth(tokenSvc))
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		r.Post("/v1/ingredients", ingHandler.Create)
		r.Post("/v1/foods", foodHandler.Create)
	})
	r.Get("/v1/foods/{id}", foodHandler.GetByID)
	r.Get("/v1/foods", foodHandler.List)

	admin := adminToken(t, tokenSvc)
	user := userToken(t, tokenSvc)

	// Seed two ingredients with known nutrition
	rr := do(t, r, http.MethodPost, "/v1/ingredients",
		`{"name":"rice","display_name":"Rice","base_unit":"grams","food_group":"grains-and-starches","allergens":[],"unit_map":{"grams":1},"nutrition":{"calories_per_base":3.5,"protein_per_base":0.07,"carbs_per_base":0.77,"fat_per_base":0.003}}`,
		admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var riceMap map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&riceMap))
	riceID := riceMap["id"].(string)

	rr = do(t, r, http.MethodPost, "/v1/ingredients",
		`{"name":"chicken","display_name":"Chicken","base_unit":"grams","food_group":"meats-and-proteins","allergens":["poultry"],"unit_map":{"grams":1},"nutrition":{"calories_per_base":1.65,"protein_per_base":0.31,"carbs_per_base":0.0,"fat_per_base":0.036}}`,
		admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var chickenMap map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&chickenMap))
	chickenID := chickenMap["id"].(string)

	// Create food with 200g rice and 300g chicken, 4 portions
	foodBody := fmt.Sprintf(`{
		"name":         "rice-and-chicken",
		"display_name": "Rice and Chicken",
		"description":  "Simple meal",
		"portions":     4,
		"labels":       ["quick","protein"],
		"recipe":       ["Cook rice","Grill chicken","Combine"],
		"ingredients": [
			{"ingredient_id": %q, "amount": 200, "unit": "grams"},
			{"ingredient_id": %q, "amount": 300, "unit": "grams"}
		]
	}`, riceID, chickenID)

	rr = do(t, r, http.MethodPost, "/v1/foods", foodBody, admin)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var created map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&created))
	id := created["id"].(string)

	// Verify fields
	assert.Equal(t, "rice-and-chicken", created["name"])
	assert.Equal(t, float64(4), created["portions"])

	// Verify nutrition: rice(200g * 3.5kcal) + chicken(300g * 1.65kcal) = 700 + 495 = 1195kcal total
	nutrition := created["nutrition"].(map[string]any)
	assert.InDelta(t, 1195.0, nutrition["calories_total"], 0.1)
	assert.InDelta(t, 1195.0/4, nutrition["calories_per_portion"], 0.1)

	// protein: rice(200*0.07) + chicken(300*0.31) = 14 + 93 = 107g
	assert.InDelta(t, 107.0, nutrition["protein_total"], 0.1)

	// Get by ID (as regular user)
	rr = do(t, r, http.MethodGet, "/v1/foods/"+id, "", user)
	require.Equal(t, http.StatusOK, rr.Code)
	var got map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.Equal(t, id, got["id"])
	assert.Equal(t, 2, len(got["ingredients"].([]any)))
}

func TestFood_PortionsDefaultTo4(t *testing.T) {
	db, err := database.OpenSQLite(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.RunMigrations(db, migrations.SQLiteFS, "sqlite"))
	t.Cleanup(func() { db.Close() })

	tokenSvc := auth.NewJWTService("test-secret")
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	foodRepo := sqliteadapter.NewFoodRepo(db)
	ingHandler := handlers.NewIngredientHandler(ingRepo)
	foodHandler := handlers.NewFoodHandler(foodRepo, ingRepo)

	r := chi.NewRouter()
	r.Use(middleware.Auth(tokenSvc))
	r.Use(middleware.RequireRole("admin"))
	r.Post("/v1/ingredients", ingHandler.Create)
	r.Post("/v1/foods", foodHandler.Create)

	admin := adminToken(t, tokenSvc)

	rr := do(t, r, http.MethodPost, "/v1/ingredients",
		`{"name":"salt","display_name":"Salt","base_unit":"grams","food_group":"spices-and-seasonings","allergens":[],"unit_map":{"grams":1}}`,
		admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var ing map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&ing))
	ingID := ing["id"].(string)

	// portions omitted → defaults to 4
	foodBody := fmt.Sprintf(`{
		"name":"simple","display_name":"Simple",
		"ingredients":[{"ingredient_id":%q,"amount":10,"unit":"grams"}]
	}`, ingID)
	rr = do(t, r, http.MethodPost, "/v1/foods", foodBody, admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var created map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&created))
	assert.Equal(t, float64(4), created["portions"])
}

func TestFood_List_WithFilters(t *testing.T) {
	db, err := database.OpenSQLite(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.RunMigrations(db, migrations.SQLiteFS, "sqlite"))
	t.Cleanup(func() { db.Close() })

	tokenSvc := auth.NewJWTService("test-secret")
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	foodRepo := sqliteadapter.NewFoodRepo(db)
	ingHandler := handlers.NewIngredientHandler(ingRepo)
	foodHandler := handlers.NewFoodHandler(foodRepo, ingRepo)

	r := chi.NewRouter()
	r.Use(middleware.Auth(tokenSvc))
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		r.Post("/v1/ingredients", ingHandler.Create)
		r.Post("/v1/foods", foodHandler.Create)
	})
	r.Get("/v1/foods", foodHandler.List)

	admin := adminToken(t, tokenSvc)
	user := userToken(t, tokenSvc)

	// Seed ingredients
	rr := do(t, r, http.MethodPost, "/v1/ingredients",
		`{"name":"beef","display_name":"Beef","base_unit":"grams","food_group":"meats-and-proteins","allergens":["red-meat"],"unit_map":{"grams":1}}`,
		admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var beefMap map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&beefMap))
	beefID := beefMap["id"].(string)

	rr = do(t, r, http.MethodPost, "/v1/ingredients",
		`{"name":"veg","display_name":"Veg","base_unit":"grams","food_group":"fruits-and-vegetables","allergens":[],"unit_map":{"grams":1}}`,
		admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var vegMap map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&vegMap))
	vegID := vegMap["id"].(string)

	// Seed foods
	beefFood := fmt.Sprintf(`{"name":"beef-stew","display_name":"Beef Stew","labels":["beef","hearty"],"ingredients":[{"ingredient_id":%q,"amount":500,"unit":"grams"}]}`, beefID)
	vegFood := fmt.Sprintf(`{"name":"veggie-soup","display_name":"Veggie Soup","labels":["vegan","healthy"],"ingredients":[{"ingredient_id":%q,"amount":300,"unit":"grams"}]}`, vegID)

	rr = do(t, r, http.MethodPost, "/v1/foods", beefFood, admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	rr = do(t, r, http.MethodPost, "/v1/foods", vegFood, admin)
	require.Equal(t, http.StatusCreated, rr.Code)

	// List all
	rr = do(t, r, http.MethodGet, "/v1/foods", "", user)
	require.Equal(t, http.StatusOK, rr.Code)
	var all []any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&all))
	assert.Len(t, all, 2)

	// Filter by label
	rr = do(t, r, http.MethodGet, "/v1/foods?label=beef", "", user)
	require.Equal(t, http.StatusOK, rr.Code)
	var byLabel []map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&byLabel))
	assert.Len(t, byLabel, 1)
	assert.Equal(t, "beef-stew", byLabel[0]["name"])

	// Exclude allergen red-meat → only veggie-soup
	rr = do(t, r, http.MethodGet, "/v1/foods?allergen_free=red-meat", "", user)
	require.Equal(t, http.StatusOK, rr.Code)
	var allergenFree []map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&allergenFree))
	assert.Len(t, allergenFree, 1)
	assert.Equal(t, "veggie-soup", allergenFree[0]["name"])

	// Search
	rr = do(t, r, http.MethodGet, "/v1/foods?search=veggie", "", user)
	require.Equal(t, http.StatusOK, rr.Code)
	var searched []map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&searched))
	assert.Len(t, searched, 1)
}

func TestFood_Update(t *testing.T) {
	db, err := database.OpenSQLite(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.RunMigrations(db, migrations.SQLiteFS, "sqlite"))
	t.Cleanup(func() { db.Close() })

	tokenSvc := auth.NewJWTService("test-secret")
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	foodRepo := sqliteadapter.NewFoodRepo(db)
	ingHandler := handlers.NewIngredientHandler(ingRepo)
	foodHandler := handlers.NewFoodHandler(foodRepo, ingRepo)

	r := chi.NewRouter()
	r.Use(middleware.Auth(tokenSvc))
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		r.Post("/v1/ingredients", ingHandler.Create)
		r.Post("/v1/foods", foodHandler.Create)
		r.Put("/v1/foods/{id}", foodHandler.Update)
	})
	r.Get("/v1/foods/{id}", foodHandler.GetByID)

	admin := adminToken(t, tokenSvc)
	user := userToken(t, tokenSvc)

	rr := do(t, r, http.MethodPost, "/v1/ingredients",
		`{"name":"pasta","display_name":"Pasta","base_unit":"grams","food_group":"grains-and-starches","allergens":["gluten"],"unit_map":{"grams":1},"nutrition":{"calories_per_base":3.7}}`,
		admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var ingM map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&ingM))
	ingID := ingM["id"].(string)

	createBody := fmt.Sprintf(`{"name":"pasta-dish","display_name":"Pasta Dish","portions":2,"labels":["italian"],"ingredients":[{"ingredient_id":%q,"amount":100,"unit":"grams"}]}`, ingID)
	rr = do(t, r, http.MethodPost, "/v1/foods", createBody, admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var created map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&created))
	id := created["id"].(string)

	// Update: change portions to 2 and use 200g
	updateBody := fmt.Sprintf(`{"name":"pasta-dish","display_name":"Pasta Dish Updated","portions":2,"labels":["italian","updated"],"ingredients":[{"ingredient_id":%q,"amount":200,"unit":"grams"}]}`, ingID)
	rr = do(t, r, http.MethodPut, "/v1/foods/"+id, updateBody, admin)
	require.Equal(t, http.StatusOK, rr.Code)
	var updated map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&updated))
	assert.Equal(t, "Pasta Dish Updated", updated["display_name"])

	// Nutrition recomputed: 200g * 3.7 = 740kcal total, /2 portions = 370 per portion
	n := updated["nutrition"].(map[string]any)
	assert.InDelta(t, 740.0, n["calories_total"], 0.1)
	assert.InDelta(t, 370.0, n["calories_per_portion"], 0.1)

	// Verify via GET
	rr = do(t, r, http.MethodGet, "/v1/foods/"+id, "", user)
	var got map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.Equal(t, "Pasta Dish Updated", got["display_name"])
}

func TestFood_Delete(t *testing.T) {
	db, err := database.OpenSQLite(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.RunMigrations(db, migrations.SQLiteFS, "sqlite"))
	t.Cleanup(func() { db.Close() })

	tokenSvc := auth.NewJWTService("test-secret")
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	foodRepo := sqliteadapter.NewFoodRepo(db)
	ingHandler := handlers.NewIngredientHandler(ingRepo)
	foodHandler := handlers.NewFoodHandler(foodRepo, ingRepo)

	r := chi.NewRouter()
	r.Use(middleware.Auth(tokenSvc))
	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireRole("admin"))
		r.Post("/v1/ingredients", ingHandler.Create)
		r.Post("/v1/foods", foodHandler.Create)
		r.Delete("/v1/foods/{id}", foodHandler.Delete)
	})
	r.Get("/v1/foods/{id}", foodHandler.GetByID)

	admin := adminToken(t, tokenSvc)
	user := userToken(t, tokenSvc)

	rr := do(t, r, http.MethodPost, "/v1/ingredients",
		`{"name":"onion","display_name":"Onion","base_unit":"grams","food_group":"fruits-and-vegetables","allergens":[],"unit_map":{"grams":1}}`,
		admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var ingM map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&ingM))

	createBody := fmt.Sprintf(`{"name":"onion-soup","display_name":"Onion Soup","ingredients":[{"ingredient_id":%q,"amount":200,"unit":"grams"}]}`, ingM["id"])
	rr = do(t, r, http.MethodPost, "/v1/foods", createBody, admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var created map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&created))
	id := created["id"].(string)

	rr = do(t, r, http.MethodDelete, "/v1/foods/"+id, "", admin)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	rr = do(t, r, http.MethodGet, "/v1/foods/"+id, "", user)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestFood_CreateRequiresAdmin(t *testing.T) {
	handler, svc, _ := setupFoodServer(t)
	user := userToken(t, svc)
	rr := do(t, handler, http.MethodPost, "/v1/foods",
		`{"name":"x","display_name":"X","ingredients":[]}`, user)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestFood_GetByID_NotFound(t *testing.T) {
	handler, svc, _ := setupFoodServer(t)
	user := userToken(t, svc)
	rr := do(t, handler, http.MethodGet, "/v1/foods/no-such-id", "", user)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestFood_Validation(t *testing.T) {
	handler, svc, _ := setupFoodServer(t)
	admin := adminToken(t, svc)

	cases := []struct {
		name string
		body string
	}{
		{"missing name", `{"display_name":"X","ingredients":[{"ingredient_id":"id","amount":1,"unit":"grams"}]}`},
		{"no ingredients", `{"name":"x","display_name":"X","ingredients":[]}`},
		{"invalid ingredient amount", `{"name":"x","display_name":"X","ingredients":[{"ingredient_id":"id","amount":0,"unit":"grams"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := do(t, handler, http.MethodPost, "/v1/foods", tc.body, admin)
			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}
}
