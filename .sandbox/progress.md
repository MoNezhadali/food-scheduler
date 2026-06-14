# FoodScheduler — Implementation Progress

## Phase 1 — Project Scaffold ✅
**Date:** 2026-06-14  
**Branch:** main

### What was built
- Go module initialised (`github.com/MoNezhadali/foodscheduler`, Go 1.23)
- Full directory structure following hexagonal architecture
- **Domain layer** — all entities, repository interfaces, and domain services:
  - `ingredient`: `Ingredient`, `FoodGroup` enum, `Allergen` enum, `UnitMap`, `NutritionInfo`, `ToBaseAmount()`, `Repository` port
  - `food`: `Food`, `FoodIngredient`, `NutritionInfo`, `Repository` port, `ComputeNutrition()` domain service
  - `mealplan`: `MealPlan`, `Request`, `Preferences`, `Restrictions`, `Plan()` function
  - `shoppinglist`: `ShoppingList`, `Item`, `Generate()` function
  - `user`: `User`, `Preferences`, `Repository` port
  - `nutrition`: `Provider` port (for USDA FoodData Central adapter, Phase 4b)
  - `domain/errors.go`: sentinel errors (`ErrNotFound`, `ErrAlreadyExists`, etc.)
- **Config** (`internal/infrastructure/config`): env-based config struct (`ENV`, `PORT`, `DB_DRIVER`, `DB_PATH`, `DB_URL`, `JWT_SECRET`, `USDA_API_KEY`)
- **Entry point** (`cmd/server/main.go`): loads config, structured JSON logging via `log/slog`
- `Makefile` with targets: `build`, `dev`, `test`, `lint`, `migrate-up/down`, `seed`, `enrich`, `docker-build`, `clean`
- `.env.example` with all required env vars documented
- `docker-compose.yml` with app service + commented-out Postgres service (Phase 12)
- `.gitignore` updated for Go binaries, SQLite files, `.env`, IDE dirs

### Key design decisions recorded here
- Domain layer has **zero external dependencies** — only stdlib
- `UnitMap` converts any recipe unit to base unit via a multiplication factor (e.g. `"cups": 200` means 1 cup = 200 grams)
- Nutrition stored as `*_per_base` (per 1 base unit); `nil` = unknown/not yet enriched
- `ComputeNutrition` skips ingredients with unknown nutrition; returns partial totals
- `Plan()` random selection uses `math/rand/v2` (unbiased `Perm`)

### Verify
```bash
go build ./...   # passes clean
go run ./cmd/server  # prints JSON startup log
```

---

## Phase 2 — Domain Unit Tests ✅
**Date:** 2026-06-14
**Branch:** main

### What was built
- Added `github.com/stretchr/testify` dependency
- **16 tests, all passing** across three packages:
  - `food/nutrition_test.go` (5 tests): full nutrition, partial nutrition (nil fields skipped), unknown ingredient skipped, zero-portions no-panic, unit conversion (tablespoons → grams)
  - `shoppinglist/generator_test.go` (5 tests): single food, shared ingredient summed across foods, grouped by food group, unknown ingredient skipped, empty food list
  - `mealplan/planner_test.go` (6 tests): correct count, no repeats, exact fit, insufficient pool error, zero count, empty pool error

### Verify
```bash
go test ./internal/domain/... -v   # 16 PASS, 0 FAIL
```

## Phase 3 — SQLite Adapter + Migrations ✅
**Date:** 2026-06-14
**Branch:** main

### What was built
- **Dependencies**: `modernc.org/sqlite` (pure Go, no CGO), `github.com/google/uuid`
- **Migration files** (`migrations/sqlite/`): 5 versioned up/down pairs — ingredients, foods, food_ingredients (with FK + index), users, user_preferences
- **`migrations/embed.go`**: embeds SQL files into the binary via `//go:embed sqlite`
- **`internal/infrastructure/database/`**:
  - `sqlite.go`: `OpenSQLite(path)` — opens connection, enables WAL + foreign keys
  - `migrate.go`: `RunMigrations(db, fs, dialect)` — lightweight custom runner; tracks applied versions in `_schema_migrations` table; safe to re-run
- **`internal/adapters/secondary/sqlite/`**: three repository implementations
  - `ingredient_repo.go`: full CRUD + `ListMissingNutrition` + `UpdateNutrition`; in-Go filtering for food group, allergens, search
  - `food_repo.go`: full CRUD with transactional create/update (food + food_ingredients atomically); allergen filtering loads ingredient allergens in one extra query
  - `user_repo.go`: create (user + preferences row in one tx), get by ID/email, update preferences
  - `helpers.go`: shared JSON marshal/unmarshal, timestamp formatting, SQL IN-clause helpers
- **Integration tests** (all against `:memory:` SQLite — fast, no disk I/O):
  - 12 ingredient tests: CRUD, filters, nutrition update
  - 9 food tests: CRUD, label filter, allergen exclusion, cascade delete, GetByIDs
  - 6 user tests: CRUD, preferences, duplicate email, not-found

### Key design decisions
- JSON strings in SQLite for arrays (`allergens`, `labels`, `recipe`) and maps (`unit_map`) — simple, portable
- Allergen filtering in food list: loads only the allergen column of affected ingredients (not full objects)
- `food_ingredients` replace-on-update: DELETE + re-INSERT in the same transaction — simpler than diffing
- FK `ON DELETE CASCADE` on `food_ingredients.food_id`; `ON DELETE RESTRICT` on `ingredient_id` (can't delete ingredient in use)

### Verify
```bash
go test ./...   # all packages green
```

## Phase 4 — Seeder ✅
**Date:** 2026-06-14
**Branch:** main

### What was built
- **Fixture files** (`internal/adapters/secondary/seed/fixtures/`): clean new-schema JSON for 19 ingredients and 3 foods
  - `ingredients.json`: all ingredients with correct `base_unit` + `unit_map` (real-world weights: 1 cup rice = 200g, 1 tablespoon olive oil = 14g, etc.)
  - `foods.json`: bread-n-cheese, chinese-meal, istanbuli — with real recipe steps and ingredient references by name
- **`seed.FixtureFS`** (`embed.go`): embeds fixtures directory into binary — no runtime file paths needed
- **`Seeder`** (`seeder.go`): reads fixtures, resolves ingredient names → IDs, inserts ingredients then foods; idempotent (catches `ErrAlreadyExists` and skips)
- **`cmd/seed/main.go`**: wires config → SQLite → migrations → repos → seeder; structured JSON log output

### Verify
```bash
go run ./cmd/seed
# First run:  ingredients_inserted=19 foods_inserted=3
# Second run: ingredients_skipped=19  foods_skipped=3  (idempotent)
```

## Phase 4b — Nutrition Enrichment CLI 🔜
Planned: `make enrich` — interactive USDA FoodData Central lookup for ingredients with NULL nutrition.

## Phase 5 — Auth Use-Cases + HTTP ✅
**Date:** 2026-06-14
**Branch:** main

### What was built
- **`migrations/sqlite/000006_add_user_role.up.sql`**: `ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'user'`
- **`internal/domain/user/entity.go`**: added `Role string // "user" | "admin"` field to `User` struct
- **`internal/adapters/secondary/sqlite/user_repo.go`**: updated to persist/scan `role` column; defaults to `"user"` on create
- **`internal/infrastructure/auth/jwt.go`**: `JWTService` with `IssueTokens()` (HS256, 15min access + 7d refresh) and `Validate()`. Defines `TokenPair`, `Claims`, `TokenType`, and the `Service` interface used by the application layer
- **`internal/application/user/register.go`**: validates email (contains @) and password (≥8 chars), bcrypts, calls `UserRepo.Create`
- **`internal/application/user/login.go`**: fetches by email, bcrypt compare, issues JWT pair; doesn't reveal whether email exists
- **`internal/application/user/refresh.go`**: validates refresh token, re-fetches user to confirm existence, issues new token pair
- **`internal/adapters/primary/http/`**: Chi router wired with RequestID + logging + recovery middleware
  - `middleware/logging.go`: structured slog logging per request
  - `middleware/recovery.go`: panic recovery → 500 JSON
  - `middleware/auth.go`: JWT Bearer validation; sets `*auth.Claims` in context; `ClaimsFromContext()` helper for handlers
  - `handlers/response.go`: `writeJSON()`, `writeError()`, `httpStatusFromError()` (maps domain errors → HTTP codes)
  - `handlers/health.go`: `GET /health` with DB ping
  - `handlers/user.go`: `POST /v1/auth/register`, `/login`, `/refresh`
  - `router.go`: Chi router with public group (`/health`, `/v1/auth/*`) and protected group (populated Phase 6+)
- **`cmd/server/main.go`**: full wiring — OpenSQLite → RunMigrations → JWTService → repos → use-cases → handlers → router → `http.ListenAndServe`

### Tests
- 10 JWT service tests (issue, validate access/refresh, wrong type, bad signature, tampered token)
- 10 use-case tests (register: ok, invalid email, short password, duplicate; login: ok, wrong pw, not found; refresh: ok, bad token, deleted user)

### Verify
```bash
go test ./...  # all packages green
# Then in a terminal:
DB_PATH=":memory:" JWT_SECRET="test-secret" PORT=9099 ./foodscheduler
curl http://localhost:9099/health                        # {"status":"ok"}
curl -X POST localhost:9099/v1/auth/register -d '{"email":"a@b.com","password":"pass1234"}'
curl -X POST localhost:9099/v1/auth/login -d '{"email":"a@b.com","password":"pass1234"}'
curl -X POST localhost:9099/v1/auth/refresh -d '{"refresh_token":"<from above>"}'
```

## Phase 6 — Ingredient CRUD ✅
**Date:** 2026-06-14
**Branch:** main

### What was built
- **`middleware/role.go`**: `RequireRole(role string)` middleware — reads `Claims` from context, returns 403 if role doesn't match
- **`handlers/ingredient.go`**: `IngredientHandler` with `List`, `GetByID`, `Create`, `Update`, `Delete`
  - `List` parses query params: `food_group`, `allergen_free` (repeatable), `search`
  - `Create`/`Update` validate required fields (name, display_name, base_unit, food_group, non-empty unit_map)
  - `toIngredientResponse()` / `toDomain()` converters keep domain types out of HTTP layer
- **`handlers/ingredient_test.go`**: 11 integration tests (httptest + real SQLite `:memory:`)
  - CRUD full lifecycle, filter by food_group/allergen_free/search, auth checks, validation
- **Router** updated with ingredient routes under `GET`/`PUT`/`DELETE`/`POST /v1/ingredients`
- **`cmd/server/main.go`** wires `IngredientRepo` → `IngredientHandler`

### Access control
- `GET /v1/ingredients` and `GET /v1/ingredients/{id}` — any authenticated user
- `POST`, `PUT`, `DELETE` — admin only (`role == "admin"`)

### Verify
```bash
go test ./...  # all packages green (52 tests)
```

## Phase 6 — Ingredient CRUD (old marker, replaced above)
## Phase 7 — Food CRUD + Nutrition Computation ✅
**Date:** 2026-06-14
**Branch:** main

### What was built
- **`handlers/food.go`**: `FoodHandler` with `List`, `GetByID`, `Create`, `Update`, `Delete`
  - `Create`/`Update` compute nutrition from ingredients at write time via `food.ComputeNutrition()`
  - `portions` defaults to 4 if 0/omitted
  - Validates: name required, display_name required, ≥1 ingredient, each ingredient has id + amount > 0 + unit
  - `List` filters: `label` (repeatable, ALL must match), `allergen_free` (repeatable), `search`
- **`handlers/food_test.go`**: 10 integration tests (httptest + real SQLite in-memory)
  - CRUD lifecycle, nutrition computation verified to 0.1 precision, filters, auth/role checks
- **Router** updated with `/v1/foods` routes (same auth/admin split as ingredients)
- **`cmd/server/main.go`** wires `FoodRepo` + `FoodHandler`

### Nutrition on write
1. Handler extracts ingredient IDs from request body
2. Fetches full ingredient entities from `IngredientRepo.GetByIDs()`
3. Calls `food.ComputeNutrition(f, ingredientMap)` — sums base-unit nutrition, divides by portions
4. Stores 8 nutrition columns (total + per-portion for cal/prot/carbs/fat) in the food row

### Verify
```bash
go test ./...  # 62 tests, all green
```

## Phase 7 — Food CRUD + Nutrition Computation (old marker, replaced above)
## Phase 8 — Shopping List Endpoint ✅
**Date:** 2026-06-14
**Branch:** main

### What was built
- **`handlers/interfaces.go`**: shared `ingredientFetcher` and `foodFetcher` interfaces (extracted from food.go so both FoodHandler and ShoppingListHandler can reference them)
- **`handlers/shopping_list.go`**: `ShoppingListHandler.Generate` — `POST /v1/shopping-list`
  1. Validates `food_ids` non-empty
  2. Fetches foods by IDs (`FoodRepo.GetByIDs`)
  3. Collects unique ingredient IDs, fetches ingredient entities
  4. Calls `shoppinglist.Generate(foods, ingMap)` domain service
  5. Returns grouped response: `{total_items, categories: {food_group: [{ingredient_id, name, display_name, total_amount, unit}]}}`
- **`handlers/shopping_list_test.go`**: 5 tests — aggregation across 2 foods (rice shared: 200+150=350g), single food, empty food_ids → 400, no auth → 401, unknown IDs → empty list
- **Router**: `POST /v1/shopping-list` added under the authenticated group
- **`cmd/server/main.go`** wires `ShoppingListHandler`

### Verify
```bash
go test ./...  # 67 tests, all green
```

## Phase 8 — Shopping List Endpoint (old marker, replaced above)
## Phase 9 — Random Meal Plan Endpoint ✅
**Date:** 2026-06-14
**Branch:** main

### What was built
- **`domain/mealplan/planner.go`** updated: implemented preference satisfaction
  - Two-phase selection: first satisfy `min_beef`/`min_chicken`/`min_fish`/`min_pork` by selecting from label-matched foods; then fill remaining slots randomly from the rest of the pool
  - Returns `ErrInsufficientFoods` if not enough foods match a minimum constraint
  - 4 new domain tests (minBeef, minBeef+minChicken, no-repeats-with-prefs, insufficient-for-preference)
- **`handlers/meal_plan.go`**: `MealPlanHandler.Generate` — `POST /v1/meal-plan`
  1. Validates `count >= 1`
  2. Builds `food.Filter` from `allergen_free` and `labels` fields
  3. Fetches filtered pool from `FoodRepo.List()`
  4. Calls `mealplan.Plan()` domain service
  5. Returns `{count, foods: [full food objects...]}`
  6. Maps `ErrInsufficientFoods` → 422 Unprocessable Entity with code `INSUFFICIENT_FOODS`
- **`handlers/interfaces.go`**: added `foodLister` interface
- **`handlers/meal_plan_test.go`**: 7 integration tests — basic count, no repeats, preferences (beef+chicken guaranteed over 10 runs), allergen filter, insufficient foods → 422, count=0 → 400, no auth → 401
- **Router**: `POST /v1/meal-plan` added under authenticated group

### Preference convention
Foods must be labelled `"beef"`, `"chicken"`, `"fish"`, or `"pork"` by admins for minimum constraints to be honoured. The label is the lookup key in the pool.

### Verify
```bash
go test ./...  # 78 tests, all green
```

## Phase 9 — Random Meal Plan Endpoint (old marker, replaced above)
## Phase 10 — User Preferences ✅
**Date:** 2026-06-14
**Branch:** main

### What was built
- **`handlers/me.go`**: `MeHandler` with three endpoints, all reading `claims.UserID` from context:
  - `GET /v1/me` — returns `{id, email, role, preferences: {excluded_allergens, dietary_restrictions}}`
  - `GET /v1/me/preferences` — returns just the preferences object
  - `PUT /v1/me/preferences` — updates and returns the new preferences; nil slices normalised to `[]`
- **`handlers/me_test.go`**: 7 tests — profile read, default empty prefs, update, update-then-get round-trip, clear to empty, two users isolated, all three routes require auth
- **Router**: `/v1/me`, `/v1/me/preferences` added under the authenticated group
- **`cmd/server/main.go`** wires `MeHandler(userRepo)`

### Verify
```bash
go test ./...  # 85 tests, all green
```

## Phase 11 — Security Hardening ✅

**Date:** 2026-06-14
**Branch:** main

### What was built

- **`middleware/cors.go`**: `CORS(allowedOrigins []string)` — wildcard or specific-origin allowlist; blocked origins → 403; preflight OPTIONS → 204 with ACAM/ACAH/ACMA headers
- **`middleware/secure_headers.go`**: `SecureHeaders()` — sets `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `X-XSS-Protection: 1; mode=block`, `Referrer-Policy: strict-origin-when-cross-origin`, `Content-Security-Policy: default-src 'none'` on every response
- **`middleware/body_limit.go`**: `BodyLimit(maxBytes int64)` — checks `Content-Length` header first; wraps `r.Body` with `http.MaxBytesReader`; oversized → 413 `PAYLOAD_TOO_LARGE`
- **`middleware/rate_limit.go`**: `RateLimit(max int, window time.Duration)` — fixed-window per-IP limiter; `sync.Mutex` map with lazy cleanup every 256 requests; exceeded → 429 `RATE_LIMITED` + `Retry-After` header; `clientIP()` honours `X-Forwarded-For`, `X-Real-IP`
- **`internal/infrastructure/config/config.go`**: added `CORSOrigins []string`; parses `CORS_ORIGINS` env var (comma-separated); defaults to `["*"]`
- **`.env.example`**: documented `CORS_ORIGINS` variable
- **`router.go`** updated: global stack now applies `SecureHeaders → CORS → Logging → Recovery → BodyLimit(1MB) → RateLimit(300/min)`; auth group gets a tighter `RateLimit(20/min)` inner middleware; `RouterDeps` gains `CORSOrigins []string`; added `chimiddleware.Timeout(30s)`
- **`cmd/server/main.go`** updated: passes `cfg.CORSOrigins` to `RouterDeps`
- **`middleware/middleware_test.go`**: 12 tests covering all four new middlewares

### Rate limits

| Scope            | Limit               |
|------------------|---------------------|
| `/v1/auth/*`     | 20 req/min per IP   |
| All other routes | 300 req/min per IP  |

### Verify

```bash
go test ./...  # 97 tests, all green
```

---

## Phase 12 — PostgreSQL Adapter ✅

**Date:** 2026-06-14
**Branch:** main

### What was built

- **`github.com/jackc/pgx/v5`** added as dependency (`pgx/v5/stdlib` for `database/sql` compatibility)
- **`migrations/postgres/`** — 5 migration files (000001–000005):
  - Columns typed as `TIMESTAMPTZ` (vs SQLite TEXT), `DOUBLE PRECISION` (vs REAL)
  - Users table includes `role` from the start (no separate migration needed)
  - JSON arrays/maps stored as TEXT — same scan code, no JSONB complexity
- **`migrations/embed.go`** — exports `PostgresFS embed.FS`
- **`internal/infrastructure/database/migrate.go`** — updated `applyMigration` to accept `dialect` and use `$1,$2,$3` placeholders for the tracking-table INSERT when dialect is "postgres"
- **`internal/infrastructure/database/postgres.go`** — `OpenPostgres(url string) (*sql.DB, error)` with connection pool tuning (25 max open, 5 idle, 5 min lifetime)
- **`internal/adapters/secondary/postgres/`** — new package `pgadapter`:
  - `helpers.go`: `toJSON`, `fromJSON`, `inPlaceholders`, `inPlaceholdersAt`, `stringsToAny`
  - `ingredient_repo.go`: full `ingredient.Repository` implementation using `$N` placeholders; timestamps scanned as `time.Time`; duplicate-key detection via `"duplicate key"` substring
  - `food_repo.go`: full `food.Repository` implementation with transactional create/update
  - `user_repo.go`: full `user.Repository` implementation
  - `repos_test.go`: integration tests for all three repos — **skipped** unless `TEST_PG_URL` env var is set; covers CRUD, duplicate detection, filters
- **`cmd/server/main.go`** — rewritten to select adapter by `cfg.DBDriver`:
  - `"postgres"` → `OpenPostgres` + `PostgresFS` + pgadapter repos
  - `"sqlite"` (default) → `OpenSQLite` + `SQLiteFS` + sqliteadapter repos
  - Repo variables typed as domain `Repository` interfaces (`domuser.Repository`, `doming.Repository`, `domfood.Repository`) so handler constructors accept either adapter

### Architecture note

The domain `food.Repository`, `ingredient.Repository`, and `user.Repository` interfaces act as the hexagonal port. Both `sqliteadapter` and `pgadapter` implement these interfaces — swapping drivers requires only changing `DB_DRIVER` (and `DB_URL` for postgres) in the environment.

### Running with PostgreSQL

```bash
export DB_DRIVER=postgres
export DB_URL=postgres://user:pass@localhost:5432/foodscheduler?sslmode=disable
go run ./cmd/server
```

### Running PG integration tests

```bash
TEST_PG_URL=postgres://user:pass@localhost:5432/foodscheduler_test?sslmode=disable \
  go test ./internal/adapters/secondary/postgres/...
```

### Build and test

```bash
go test ./...  # 97 tests pass; postgres package reports "ok" (tests skipped without TEST_PG_URL)
go build ./... # clean
```

---

## Phase 13 — Dockerfile + CI 🔜

## Phase 14 — OpenAPI Docs 🔜
