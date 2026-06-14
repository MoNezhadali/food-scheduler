# FoodScheduler — Development Plan

> Status: DRAFT — awaiting approval

---

## 1. Key Decisions (Locked)

| Decision | Choice | Rationale |
|---|---|---|
| Language | Go 1.23+ | Statically typed, single binary, native concurrency |
| Database (dev/start) | SQLite | Zero-ops local storage, file-based, easy to ship |
| Database (prod target) | PostgreSQL | Native arrays, JSONB, connection pooling for EKS/ECS |
| DB flexibility | Ports & Adapters | Repository interfaces abstract the driver; swap without touching domain |
| Auth | User accounts + JWT | Standard, stateless, EKS-friendly |
| Deployment | Docker → EKS or ECS | Containerised, cloud-native |
| Admin | REST API | No additional tooling needed |

---

## 2. Why SQL (not NoSQL) for Food Ingredients

Foods have a variable number of ingredients with per-food amounts — this is textbook SQL:

```
food_ingredients table:
  food_id      | ingredient_id   | amount | unit
  chinese-meal | chicken-breast  | 500    | grams
  chinese-meal | rice            | 4      | cups
  bread-cheese | sangak-bread    | 300    | grams   ← same ingredient, different amount in different food
  bread-cheese | sheep-cheese    | 200    | grams
```

This handles all core features better than NoSQL embedded arrays:

- **Allergen filtering**: `JOIN food_ingredients → ingredients WHERE allergens NOT IN (...)` — one SQL query
- **Shopping list aggregation**: `GROUP BY ingredient_id, SUM(amount)` — one query, zero application-side looping
- **Referential integrity**: DB enforces that food cannot reference a non-existent ingredient
- **"Which foods use ingredient X?"**: instant reverse lookup, needed for smart substitutions later

NoSQL embedding would make single-food reads marginally simpler, but makes your two primary features (allergen filtering, shopping list) painful. SQL wins for this domain.

---

## 3. Technology Stack

| Concern | Choice |
|---|---|
| Language | Go 1.23+ |
| HTTP framework | [Chi](https://github.com/go-chi/chi) — lightweight, stdlib-compatible |
| SQLite driver | `modernc.org/sqlite` — pure Go, no CGO, easy Docker builds |
| PostgreSQL driver | `jackc/pgx` — used later when scaling |
| DB migrations | `golang-migrate/migrate` — supports both SQLite and PostgreSQL |
| Auth | `golang-jwt/jwt` + `bcrypt` for password hashing |
| Config | `kelseyhightower/envconfig` — 12-factor, env-only |
| Validation | `go-playground/validator` — struct tag validation |
| Logging | `log/slog` (stdlib) — structured, zero deps |
| Testing | `testify` + `net/http/httptest` |
| Containerisation | Docker + Docker Compose |
| API docs | OpenAPI 3.1 (hand-authored `docs/openapi.yaml`) |

---

## 4. Architecture — Hexagonal (Ports & Adapters)

```
foodscheduler/
├── cmd/
│   └── server/
│       └── main.go              # Wire everything, start server
│
├── internal/
│   ├── domain/                  # Pure business logic. Zero external imports.
│   │   ├── ingredient/
│   │   │   ├── entity.go        # Ingredient, FoodGroup (enum), Allergen
│   │   │   └── repository.go    # Port: IngredientRepository interface
│   │   ├── food/
│   │   │   ├── entity.go        # Food, FoodIngredient, NutritionInfo
│   │   │   ├── repository.go    # Port: FoodRepository interface
│   │   │   └── nutrition.go     # Domain service: ComputeNutrition(food, ingredients)
│   │   ├── mealplan/
│   │   │   ├── entity.go        # MealPlan, Preferences, Restriction
│   │   │   └── planner.go       # Domain service: random selection algorithm
│   │   ├── shoppinglist/
│   │   │   ├── entity.go        # ShoppingList, ShoppingItem (grouped by FoodGroup)
│   │   │   └── generator.go     # Domain service: aggregate + group ingredients
│   │   └── user/
│   │       ├── entity.go        # User, UserPreferences
│   │       └── repository.go    # Port: UserRepository interface
│   │
│   ├── application/             # Use-cases: orchestrate domain + call ports
│   │   ├── food/
│   │   │   ├── list_foods.go        # ListFoods(filters Filter) ([]Food, error)
│   │   │   ├── get_food.go          # GetFood(id UUID) (Food, error)
│   │   │   ├── create_food.go       # CreateFood(cmd CreateFoodCmd) (Food, error)
│   │   │   └── update_food.go
│   │   ├── ingredient/
│   │   │   ├── list_ingredients.go
│   │   │   ├── create_ingredient.go
│   │   │   └── update_ingredient.go
│   │   ├── mealplan/
│   │   │   └── generate_random.go   # GenerateRandomMealPlan(req Request) (MealPlan, error)
│   │   ├── shoppinglist/
│   │   │   └── generate.go          # GenerateShoppingList(foodIDs []UUID) (ShoppingList, error)
│   │   └── user/
│   │       ├── register.go          # Register(cmd RegisterCmd) (User, error)
│   │       ├── login.go             # Login(email, password) (TokenPair, error)
│   │       └── update_preferences.go
│   │
│   ├── adapters/
│   │   ├── primary/             # Driving adapters (HTTP → Application)
│   │   │   └── http/
│   │   │       ├── router.go
│   │   │       ├── middleware/
│   │   │       │   ├── auth.go         # JWT validation
│   │   │       │   ├── logging.go
│   │   │       │   ├── recovery.go
│   │   │       │   └── ratelimit.go
│   │   │       └── handlers/
│   │   │           ├── food.go
│   │   │           ├── ingredient.go
│   │   │           ├── mealplan.go
│   │   │           ├── shoppinglist.go
│   │   │           └── user.go
│   │   │
│   │   └── secondary/           # Driven adapters (implement domain repository ports)
│   │       ├── sqlite/          # SQLite implementation (Phase 1 target)
│   │       │   ├── food_repo.go
│   │       │   ├── ingredient_repo.go
│   │       │   └── user_repo.go
│   │       ├── postgres/        # PostgreSQL implementation (Phase 12 target)
│   │       │   ├── food_repo.go
│   │       │   ├── ingredient_repo.go
│   │       │   └── user_repo.go
│   │       └── seed/
│   │           └── seeder.go    # Reads existing JSON files → inserts into DB
│   │
│   └── infrastructure/
│       ├── config/
│       │   └── config.go        # Config struct from env vars
│       ├── database/
│       │   ├── sqlite.go        # Open SQLite connection + run migrations
│       │   └── postgres.go      # Open pgx pool + run migrations
│       ├── auth/
│       │   └── jwt.go           # Sign / verify JWT tokens
│       └── logger/
│           └── logger.go
│
├── migrations/
│   ├── sqlite/
│   │   ├── 001_create_ingredients.up.sql
│   │   ├── 001_create_ingredients.down.sql
│   │   ├── 002_create_foods.up.sql
│   │   ├── 002_create_foods.down.sql
│   │   ├── 003_create_users.up.sql
│   │   └── ...
│   └── postgres/                # Same schema, PG-specific types (TEXT[] instead of JSON text)
│
├── docs/
│   └── openapi.yaml
│
├── docker/
│   ├── Dockerfile
│   └── docker-compose.yml       # App + (optional) Postgres for local prod-like testing
│
├── .sandbox/
├── .env.example
├── sqlc.yaml                    # If we add sqlc later
└── Makefile
```

---

## 5. Data Model

### `ingredients`
| Column | Type (SQLite) | Notes |
|---|---|---|
| id | TEXT (UUID) | Primary key |
| name | TEXT UNIQUE NOT NULL | Slug: `chicken-breast` |
| display_name | TEXT NOT NULL | |
| food_group | TEXT NOT NULL | Validated enum in Go |
| allergens | TEXT NOT NULL | JSON array: `["dairy","gluten"]` |
| unit_map | TEXT NOT NULL | JSON object: `{"grams":1,"cup":200}` — base unit is the key with value 1 |
| base_unit | TEXT NOT NULL | e.g. `grams` — reference unit for nutrition values |
| calories_per_base | REAL | kcal per 1 base_unit (nullable) |
| protein_per_base | REAL | grams protein per 1 base_unit (nullable) |
| carbs_per_base | REAL | grams carbs per 1 base_unit (nullable) |
| fat_per_base | REAL | grams fat per 1 base_unit (nullable) |
| created_at | TEXT (ISO8601) | |
| updated_at | TEXT (ISO8601) | |

**Note on unit_map**: e.g. for rice: `{"grams":1,"cup":200,"cups":200}`. The base_unit is `grams`. To convert `4 cups` to base: `4 * 200 = 800 grams`. Nutrition: `calories_per_base * 800`. This unified approach handles both countable (cups) and weight (grams) ingredients.

### `foods`
| Column | Type (SQLite) | Notes |
|---|---|---|
| id | TEXT (UUID) | |
| name | TEXT UNIQUE NOT NULL | Slug |
| display_name | TEXT NOT NULL | |
| description | TEXT | |
| portions | INTEGER NOT NULL DEFAULT 4 | |
| recipe | TEXT NOT NULL | JSON array of step strings |
| labels | TEXT NOT NULL | JSON array: `["vegetarian","main-course"]` |
| calories_total | REAL | Computed on write: sum across all food_ingredients |
| calories_per_portion | REAL | `calories_total / portions` |
| protein_total | REAL | |
| protein_per_portion | REAL | |
| carbs_total | REAL | |
| carbs_per_portion | REAL | |
| fat_total | REAL | |
| fat_per_portion | REAL | |
| created_at | TEXT | |
| updated_at | TEXT | |

### `food_ingredients` (junction — answers the variable-ingredients question)
| Column | Type | Notes |
|---|---|---|
| food_id | TEXT (FK → foods.id) | |
| ingredient_id | TEXT (FK → ingredients.id) | |
| amount | REAL NOT NULL | Quantity in the food's recipe |
| unit | TEXT NOT NULL | Unit from the ingredient's unit_map |
| PRIMARY KEY | (food_id, ingredient_id) | One row per ingredient per food |

### `users`
| Column | Type | Notes |
|---|---|---|
| id | TEXT (UUID) | |
| email | TEXT UNIQUE NOT NULL | |
| password_hash | TEXT NOT NULL | bcrypt |
| created_at | TEXT | |
| updated_at | TEXT | |

### `user_preferences`
| Column | Type | Notes |
|---|---|---|
| user_id | TEXT (FK → users.id) PRIMARY KEY | One-to-one |
| excluded_allergens | TEXT | JSON array |
| dietary_restrictions | TEXT | JSON array: `["vegetarian","no-gluten"]` |
| updated_at | TEXT | |

---

## 6. API Endpoints

All responses: `Content-Type: application/json`  
Error body: `{ "error": "message", "code": "SNAKE_CASE_CODE" }`

### Auth (public)
```
POST   /v1/auth/register       { email, password }
POST   /v1/auth/login          { email, password } → { access_token, refresh_token }
POST   /v1/auth/refresh        { refresh_token }   → { access_token }
```

### Ingredients (read: public; write: admin role)
```
GET    /v1/ingredients         ?food_group=meats-and-proteins&allergen_free=gluten,dairy
GET    /v1/ingredients/:id
POST   /v1/ingredients         [admin]
PUT    /v1/ingredients/:id     [admin]
DELETE /v1/ingredients/:id     [admin]
```

### Foods (read: authenticated; write: admin)
```
GET    /v1/foods               ?label=vegetarian&exclude_allergens=gluten&search=chicken
GET    /v1/foods/:id
POST   /v1/foods               [admin]
PUT    /v1/foods/:id           [admin]
DELETE /v1/foods/:id           [admin]
```

### User Preferences (authenticated)
```
GET    /v1/me/preferences
PUT    /v1/me/preferences      { excluded_allergens: [], dietary_restrictions: [] }
```

### Meal Plan (authenticated)
```
POST   /v1/meal-plans/random
Body:
{
  "count": 5,
  "use_my_preferences": true,   // apply user's saved restrictions automatically
  "extra_restrictions": ["no-pork"],
  "preferences": {
    "min_beef": 1,
    "min_chicken": 1,
    "min_fish": 1
  }
}
Response: { "foods": [...Food objects with nutrition...] }
```

### Shopping List (authenticated)
```
POST   /v1/shopping-list
Body: { "food_ids": ["uuid1", "uuid2"], "scale_portions": 4 }
Response:
{
  "total_items": 12,
  "categories": {
    "meats-and-proteins": [
      { "ingredient_id": "...", "display_name": "Chicken Breast", "total_amount": 500, "unit": "grams" }
    ],
    "grains-and-starches": [ ... ]
  }
}
```

### Health (public)
```
GET    /health   → 200 { "status": "ok", "db": "ok" }
```

---

## 7. Key Business Logic

### Nutrition Computation (on food create/update)
1. For each `food_ingredient`: convert `amount + unit` → base units using `ingredient.unit_map`
2. Multiply by `ingredient.calories_per_base` (and protein/carbs/fat)
3. Sum all ingredients → `calories_total`
4. Divide by `food.portions` → `calories_per_portion`
5. Store both. This runs in the domain service `nutrition.ComputeNutrition()` which takes only Go structs — no DB calls.

### Random Meal Plan Algorithm (`mealplan.Planner`)
1. Load food pool filtered by user restrictions (SQL WHERE on labels/allergens via repo)
2. Satisfy hard `preferences` first: for each `min_X` constraint, pick randomly from foods matching that protein type, remove from pool
3. Fill remaining `count - satisfied` slots by random sampling from remaining pool (no repeats)
4. If pool exhausted before `count` reached → return `ErrInsufficientFoods` with how many were found
5. Shuffle final list before returning

### Shopping List Aggregation (`shoppinglist.Generator`)
1. Load `food_ingredients` for all given food IDs (one DB query)
2. Convert all amounts to each ingredient's base unit (using unit_map)
3. Group by `ingredient_id`, sum amounts
4. Group output by `food_group` for display
5. Convert back to a human-friendly unit (e.g. `800 grams` → display as `800g` or keep in base unit — TBD with you)

---

## 8. Security

| Concern | Implementation |
|---|---|
| Passwords | bcrypt cost factor 12 |
| Auth tokens | JWT (RS256 preferred; HS256 acceptable for start) with short-lived access tokens (15 min) + refresh tokens (7 days) |
| Input validation | `go-playground/validator` on all request structs; enum checks, max lengths |
| SQL injection | Parameterised queries only (never string interpolation) |
| Rate limiting | Token bucket per IP on all endpoints; stricter on `/v1/auth/*` |
| CORS | Explicit allowlist; no wildcard `*` in prod |
| Secrets | All config via env vars; no secrets in code or Docker image |
| Panic recovery | Middleware catches panics, returns 500 without stack trace in response |
| Admin routes | Checked via `role` claim in JWT (no separate admin endpoints for now; role-based on same endpoints) |
| DB file (SQLite) | File permissions 0600; mounted via Docker volume |

---

## 9. Nutrition Enrichment via USDA FoodData Central

Nutrition data is fetched **once per ingredient** and stored — never queried on every request.

**Why USDA FoodData Central:**
- Free, no rate limits that would affect one-time enrichment of hundreds of ingredients
- Official US government data — high quality and comprehensive
- Returns data per 100g → maps directly to our `base_unit` model (divide by 100 to get per-gram)
- Instant free API key at [fdc.nal.usda.gov](https://fdc.nal.usda.gov/api-guide.html)

**Architecture:**

A `NutritionProvider` port lives at `internal/domain/nutrition/provider.go`:
```go
type Provider interface {
    Search(query string) ([]SearchResult, error)
    GetNutrition(providerID, baseUnit string) (NutritionData, error)
}
```

The USDA adapter implements this at `adapters/secondary/nutrition/usda/`. Swapping to Edamam or any other source = new adapter, zero domain changes.

**Enrichment CLI (`make enrich`):**
1. Load all ingredients where nutrition fields are NULL (never enriched yet — skips already-enriched records)
2. For each, call `Provider.Search(ingredient.Name)` → USDA returns multiple candidates
3. Print candidates + prompt: "Pick the best match (1-N) or skip (s):"
4. On selection, call `Provider.GetNutrition(id, ingredient.BaseUnit)` → write to DB
5. Safe to re-run at any time — only processes NULL fields

Config: `USDA_API_KEY` env var.

---

## 10. Implementation Phases

Each phase = one or more commits, all tests passing, app runnable after each.

| # | Phase | Deliverable |
|---|---|---|
| P1 | **Project scaffold** | `go mod init`, Makefile, `.env.example`, `docker-compose.yml` (app only), empty domain stubs compile |
| P2 | **Domain layer** | All entities, repository interfaces, domain services (nutrition, planner, shopping list) with unit tests |
| P3 | **SQLite adapter + migrations** | SQLite connection, migration files, SQLite implementations of all repository interfaces; integration tests with real SQLite file |
| P4 | **Seeder** | `make seed` reads existing JSON files from `foods/`, `ingredients/`, inserts into DB; idempotent |
| P5 | **Auth use-cases + HTTP** | Register, login, refresh endpoints; JWT middleware; `GET /health` |
| P6 | **Ingredient CRUD** | List/Get/Create/Update/Delete ingredient endpoints; admin role check |
| P7 | **Food CRUD + nutrition** | List/Get/Create/Update/Delete food endpoints; nutrition computed on write |
| P8 | **Shopping list** | `POST /v1/shopping-list` endpoint; full unit + integration test |
| P9 | **Random meal plan** | `POST /v1/meal-plans/random` endpoint; constraint satisfaction; tests with edge cases |
| P10 | **User preferences** | Preferences stored per user; `use_my_preferences: true` wires into meal plan and food filters |
| P11 | **Security hardening** | Rate limiting, CORS, input validation on all routes, panic recovery, security review |
| P12 | **PostgreSQL adapter** | Postgres implementation of all repository interfaces; config flag `DB_DRIVER=sqlite\|postgres` switches at startup; Docker Compose with Postgres service |
| P13 | **Dockerfile + CI** | Multi-stage Dockerfile (build → minimal runtime image); GitHub Actions: lint, test, build image |
| P14 | **OpenAPI doc** | `docs/openapi.yaml` describing all endpoints; validated against handlers |

---

## 10. Makefile Targets

```makefile
make dev          # Run with hot-reload (air)
make build        # Build binary
make test         # Run all tests
make test-unit    # Run only unit tests (no DB)
make test-int     # Run integration tests (spins up SQLite)
make migrate-up   # Apply all pending migrations
make migrate-down # Roll back last migration
make seed         # Load JSON fixtures into DB
make lint         # golangci-lint
make docker-build # Build Docker image
```

---

*Created: 2026-06-13 | Language: Go | DB: SQLite → PostgreSQL | Auth: JWT*
