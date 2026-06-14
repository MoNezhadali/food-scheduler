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

## Phase 3 — SQLite Adapter + Migrations 🔜
Planned: DB connection, migration files, SQLite implementations of all repository interfaces.

## Phase 4 — Seeder 🔜
Planned: load existing JSON files from `foods/` and `ingredients/` into DB.

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
