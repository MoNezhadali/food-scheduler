package handlers_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func setupIngredientServer(t *testing.T) (http.Handler, auth.Service) {
	t.Helper()

	db, err := database.OpenSQLite(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.RunMigrations(db, migrations.SQLiteFS, "sqlite"))
	t.Cleanup(func() { db.Close() })

	tokenSvc := auth.NewJWTService("test-secret")
	ingRepo := sqliteadapter.NewIngredientRepo(db)
	ingHandler := handlers.NewIngredientHandler(ingRepo)

	r := chi.NewRouter()
	r.Use(middleware.Auth(tokenSvc))
	r.Route("/v1/ingredients", func(r chi.Router) {
		r.Get("/", ingHandler.List)
		r.Get("/{id}", ingHandler.GetByID)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin"))
			r.Post("/", ingHandler.Create)
			r.Put("/{id}", ingHandler.Update)
			r.Delete("/{id}", ingHandler.Delete)
		})
	})
	return r, tokenSvc
}

func do(t *testing.T, handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func userToken(t *testing.T, svc auth.Service) string {
	t.Helper()
	p, err := svc.IssueTokens("uid-user", "user@example.com", "user")
	require.NoError(t, err)
	return p.AccessToken
}

func adminToken(t *testing.T, svc auth.Service) string {
	t.Helper()
	p, err := svc.IssueTokens("uid-admin", "admin@example.com", "admin")
	require.NoError(t, err)
	return p.AccessToken
}

const validIngredientBody = `{
	"name":         "test-chicken",
	"display_name": "Test Chicken",
	"base_unit":    "grams",
	"food_group":   "meats-and-proteins",
	"allergens":    ["poultry"],
	"unit_map":     {"grams": 1},
	"nutrition":    {"calories_per_base": 1.65, "protein_per_base": 0.31}
}`

// ── tests ─────────────────────────────────────────────────────────────────────

func TestIngredient_CreateAndGetByID(t *testing.T) {
	handler, svc := setupIngredientServer(t)
	admin := adminToken(t, svc)
	user := userToken(t, svc)

	// Create
	rr := do(t, handler, http.MethodPost, "/v1/ingredients", validIngredientBody, admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var created map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&created))
	id := created["id"].(string)
	assert.NotEmpty(t, id)
	assert.Equal(t, "test-chicken", created["name"])
	assert.Equal(t, "meats-and-proteins", created["food_group"])

	// Get by ID
	rr = do(t, handler, http.MethodGet, "/v1/ingredients/"+id, "", user)
	require.Equal(t, http.StatusOK, rr.Code)
	var got map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.Equal(t, id, got["id"])
	assert.Equal(t, "Test Chicken", got["display_name"])
}

func TestIngredient_List(t *testing.T) {
	handler, svc := setupIngredientServer(t)
	admin := adminToken(t, svc)
	user := userToken(t, svc)

	bodies := []string{
		`{"name":"ing-a","display_name":"A","base_unit":"grams","food_group":"meats-and-proteins","allergens":[],"unit_map":{"grams":1}}`,
		`{"name":"ing-b","display_name":"B","base_unit":"grams","food_group":"dairy","allergens":["dairy"],"unit_map":{"grams":1}}`,
		`{"name":"ing-c","display_name":"C","base_unit":"grams","food_group":"meats-and-proteins","allergens":[],"unit_map":{"grams":1}}`,
	}
	for _, b := range bodies {
		rr := do(t, handler, http.MethodPost, "/v1/ingredients", b, admin)
		require.Equal(t, http.StatusCreated, rr.Code)
	}

	// List all
	rr := do(t, handler, http.MethodGet, "/v1/ingredients", "", user)
	require.Equal(t, http.StatusOK, rr.Code)
	var all []map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&all))
	assert.Len(t, all, 3)

	// Filter by food_group
	rr = do(t, handler, http.MethodGet, "/v1/ingredients?food_group=meats-and-proteins", "", user)
	require.Equal(t, http.StatusOK, rr.Code)
	var filtered []map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&filtered))
	assert.Len(t, filtered, 2)

	// Filter allergen_free=dairy (should exclude ing-b)
	rr = do(t, handler, http.MethodGet, "/v1/ingredients?allergen_free=dairy", "", user)
	require.Equal(t, http.StatusOK, rr.Code)
	var dairyFree []map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&dairyFree))
	assert.Len(t, dairyFree, 2)

	// Search
	rr = do(t, handler, http.MethodGet, "/v1/ingredients?search=ing-b", "", user)
	require.Equal(t, http.StatusOK, rr.Code)
	var searched []map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&searched))
	assert.Len(t, searched, 1)
	assert.Equal(t, "ing-b", searched[0]["name"])
}

func TestIngredient_Update(t *testing.T) {
	handler, svc := setupIngredientServer(t)
	admin := adminToken(t, svc)
	user := userToken(t, svc)

	rr := do(t, handler, http.MethodPost, "/v1/ingredients", validIngredientBody, admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var created map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&created))
	id := created["id"].(string)

	updateBody := fmt.Sprintf(`{
		"name":         "test-chicken",
		"display_name": "Updated Chicken",
		"base_unit":    "grams",
		"food_group":   "meats-and-proteins",
		"allergens":    [],
		"unit_map":     {"grams": 1},
		"nutrition":    {"calories_per_base": 2.0}
	}`)
	rr = do(t, handler, http.MethodPut, "/v1/ingredients/"+id, updateBody, admin)
	require.Equal(t, http.StatusOK, rr.Code)
	var updated map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&updated))
	assert.Equal(t, "Updated Chicken", updated["display_name"])

	// Verify via GET
	rr = do(t, handler, http.MethodGet, "/v1/ingredients/"+id, "", user)
	var got map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&got))
	assert.Equal(t, "Updated Chicken", got["display_name"])
}

func TestIngredient_Delete(t *testing.T) {
	handler, svc := setupIngredientServer(t)
	admin := adminToken(t, svc)
	user := userToken(t, svc)

	rr := do(t, handler, http.MethodPost, "/v1/ingredients", validIngredientBody, admin)
	require.Equal(t, http.StatusCreated, rr.Code)
	var created map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&created))
	id := created["id"].(string)

	rr = do(t, handler, http.MethodDelete, "/v1/ingredients/"+id, "", admin)
	assert.Equal(t, http.StatusNoContent, rr.Code)

	rr = do(t, handler, http.MethodGet, "/v1/ingredients/"+id, "", user)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestIngredient_GetByID_NotFound(t *testing.T) {
	handler, svc := setupIngredientServer(t)
	user := userToken(t, svc)
	rr := do(t, handler, http.MethodGet, "/v1/ingredients/no-such-id", "", user)
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestIngredient_CreateRequiresAdmin(t *testing.T) {
	handler, svc := setupIngredientServer(t)
	user := userToken(t, svc)
	rr := do(t, handler, http.MethodPost, "/v1/ingredients", validIngredientBody, user)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestIngredient_DeleteRequiresAdmin(t *testing.T) {
	handler, svc := setupIngredientServer(t)
	user := userToken(t, svc)
	rr := do(t, handler, http.MethodDelete, "/v1/ingredients/some-id", "", user)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestIngredient_UnauthenticatedRequest(t *testing.T) {
	handler, _ := setupIngredientServer(t)
	rr := do(t, handler, http.MethodGet, "/v1/ingredients", "", "")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestIngredient_CreateValidation(t *testing.T) {
	handler, svc := setupIngredientServer(t)
	admin := adminToken(t, svc)

	cases := []struct {
		name string
		body string
	}{
		{"missing name", `{"display_name":"X","base_unit":"grams","food_group":"dairy","unit_map":{"grams":1}}`},
		{"missing base_unit", `{"name":"x","display_name":"X","food_group":"dairy","unit_map":{"grams":1}}`},
		{"empty unit_map", `{"name":"x","display_name":"X","base_unit":"grams","food_group":"dairy","unit_map":{}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := do(t, handler, http.MethodPost, "/v1/ingredients", tc.body, admin)
			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}
}
