package handlers_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MoNezhadali/foodscheduler/internal/adapters/primary/http/handlers"
	"github.com/MoNezhadali/foodscheduler/internal/adapters/primary/http/middleware"
	sqliteadapter "github.com/MoNezhadali/foodscheduler/internal/adapters/secondary/sqlite"
	appuser "github.com/MoNezhadali/foodscheduler/internal/application/user"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/auth"
	"github.com/MoNezhadali/foodscheduler/internal/infrastructure/database"
	"github.com/MoNezhadali/foodscheduler/migrations"
)

func setupMeServer(t *testing.T) (http.Handler, auth.Service, *sqliteadapter.UserRepo) {
	t.Helper()

	db, err := database.OpenSQLite(":memory:")
	require.NoError(t, err)
	require.NoError(t, database.RunMigrations(db, migrations.SQLiteFS, "sqlite"))
	t.Cleanup(func() { db.Close() })

	tokenSvc := auth.NewJWTService("test-secret")
	userRepo := sqliteadapter.NewUserRepo(db)
	meHandler := handlers.NewMeHandler(userRepo)
	userHandler := handlers.NewUserHandler(
		appuser.NewRegisterUseCase(userRepo),
		appuser.NewLoginUseCase(userRepo, tokenSvc),
		appuser.NewRefreshUseCase(userRepo, tokenSvc),
	)

	r := chi.NewRouter()
	// Public auth routes
	r.Post("/v1/auth/register", userHandler.Register)
	r.Post("/v1/auth/login", userHandler.Login)
	// Protected /me routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(tokenSvc))
		r.Get("/v1/me", meHandler.GetProfile)
		r.Get("/v1/me/preferences", meHandler.GetPreferences)
		r.Put("/v1/me/preferences", meHandler.UpdatePreferences)
	})
	return r, tokenSvc, userRepo
}

// registerAndLogin registers a user and returns the access token.
func registerAndLogin(t *testing.T, r http.Handler, email, password string) string {
	t.Helper()
	rr := do(t, r, http.MethodPost, "/v1/auth/register",
		`{"email":"`+email+`","password":"`+password+`"}`, "")
	require.Equal(t, http.StatusCreated, rr.Code)

	rr = do(t, r, http.MethodPost, "/v1/auth/login",
		`{"email":"`+email+`","password":"`+password+`"}`, "")
	require.Equal(t, http.StatusOK, rr.Code)
	var tok map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&tok))
	return tok["access_token"].(string)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestMe_GetProfile(t *testing.T) {
	r, _, _ := setupMeServer(t)
	token := registerAndLogin(t, r, "alice@example.com", "password123")

	rr := do(t, r, http.MethodGet, "/v1/me", "", token)
	require.Equal(t, http.StatusOK, rr.Code)

	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.Equal(t, "alice@example.com", resp["email"])
	assert.Equal(t, "user", resp["role"])
	assert.NotEmpty(t, resp["id"])

	prefs := resp["preferences"].(map[string]any)
	assert.Equal(t, []any{}, prefs["excluded_allergens"])
	assert.Equal(t, []any{}, prefs["dietary_restrictions"])
}

func TestMe_GetPreferences_Default(t *testing.T) {
	r, _, _ := setupMeServer(t)
	token := registerAndLogin(t, r, "bob@example.com", "password123")

	rr := do(t, r, http.MethodGet, "/v1/me/preferences", "", token)
	require.Equal(t, http.StatusOK, rr.Code)

	var prefs map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&prefs))
	assert.Equal(t, []any{}, prefs["excluded_allergens"])
	assert.Equal(t, []any{}, prefs["dietary_restrictions"])
}

func TestMe_UpdatePreferences(t *testing.T) {
	r, _, _ := setupMeServer(t)
	token := registerAndLogin(t, r, "carol@example.com", "password123")

	body := `{"excluded_allergens":["gluten","dairy"],"dietary_restrictions":["vegetarian"]}`
	rr := do(t, r, http.MethodPut, "/v1/me/preferences", body, token)
	require.Equal(t, http.StatusOK, rr.Code)

	var updated map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&updated))
	assert.ElementsMatch(t, []any{"gluten", "dairy"}, updated["excluded_allergens"])
	assert.Equal(t, []any{"vegetarian"}, updated["dietary_restrictions"])
}

func TestMe_UpdateThenGet(t *testing.T) {
	r, _, _ := setupMeServer(t)
	token := registerAndLogin(t, r, "dave@example.com", "password123")

	// Update
	rr := do(t, r, http.MethodPut, "/v1/me/preferences",
		`{"excluded_allergens":["nuts"],"dietary_restrictions":["vegan"]}`, token)
	require.Equal(t, http.StatusOK, rr.Code)

	// GET /me should reflect the update
	rr = do(t, r, http.MethodGet, "/v1/me", "", token)
	require.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	prefs := resp["preferences"].(map[string]any)
	assert.Equal(t, []any{"nuts"}, prefs["excluded_allergens"])
	assert.Equal(t, []any{"vegan"}, prefs["dietary_restrictions"])
}

func TestMe_ClearPreferences(t *testing.T) {
	r, _, _ := setupMeServer(t)
	token := registerAndLogin(t, r, "eve@example.com", "password123")

	// Set some prefs
	do(t, r, http.MethodPut, "/v1/me/preferences",
		`{"excluded_allergens":["gluten"],"dietary_restrictions":["keto"]}`, token)

	// Clear them
	rr := do(t, r, http.MethodPut, "/v1/me/preferences",
		`{"excluded_allergens":[],"dietary_restrictions":[]}`, token)
	require.Equal(t, http.StatusOK, rr.Code)

	rr = do(t, r, http.MethodGet, "/v1/me/preferences", "", token)
	var prefs map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&prefs))
	assert.Equal(t, []any{}, prefs["excluded_allergens"])
	assert.Equal(t, []any{}, prefs["dietary_restrictions"])
}

func TestMe_UsersIsolated(t *testing.T) {
	r, _, _ := setupMeServer(t)
	tokenA := registerAndLogin(t, r, "user-a@example.com", "password123")
	tokenB := registerAndLogin(t, r, "user-b@example.com", "password123")

	// User A sets gluten-free
	do(t, r, http.MethodPut, "/v1/me/preferences",
		`{"excluded_allergens":["gluten"],"dietary_restrictions":[]}`, tokenA)

	// User B's preferences should be unaffected
	rr := do(t, r, http.MethodGet, "/v1/me/preferences", "", tokenB)
	var prefs map[string]any
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&prefs))
	assert.Equal(t, []any{}, prefs["excluded_allergens"])
}

func TestMe_RequiresAuth(t *testing.T) {
	r, _, _ := setupMeServer(t)

	rr := do(t, r, http.MethodGet, "/v1/me", "", "")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	rr = do(t, r, http.MethodGet, "/v1/me/preferences", "", "")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)

	rr = do(t, r, http.MethodPut, "/v1/me/preferences", `{}`, "")
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
