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

## Phase 5 — Auth Use-Cases + HTTP 🔜
Planned: register/login/refresh endpoints, JWT middleware, `GET /health`.

## Phase 6 — Ingredient CRUD 🔜
## Phase 7 — Food CRUD + Nutrition Computation 🔜
## Phase 8 — Shopping List Endpoint 🔜
## Phase 9 — Random Meal Plan Endpoint 🔜
## Phase 10 — User Preferences 🔜
## Phase 11 — Security Hardening 🔜
## Phase 12 — PostgreSQL Adapter 🔜
## Phase 13 — Dockerfile + CI 🔜
## Phase 14 — OpenAPI Docs 🔜
