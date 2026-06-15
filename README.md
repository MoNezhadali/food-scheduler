# FoodScheduler

A REST API for weekly meal planning, shopping list generation, and food/ingredient management. Built in Go with a hexagonal architecture so the storage driver (SQLite or PostgreSQL) can be swapped by changing a single environment variable.

---

## Features

- **Ingredient & Food CRUD** — full lifecycle management with admin-role protection
- **Nutrition computation** — calories, protein, carbs, and fat calculated automatically on food create/update
- **Random meal plan** — generates a week's worth of meals from your food catalogue, with optional per-protein-type minimums (e.g. "at least 1 beef, 1 chicken, 1 fish")
- **Shopping list** — aggregates ingredients across multiple foods, converts all amounts to base units, and groups them by food category
- **User preferences** — per-user allergen exclusions and dietary restrictions that are applied automatically when generating meal plans
- **JWT auth** — stateless Bearer tokens (15 min access + 7 day refresh), bcrypt-hashed passwords
- **Security hardening** — CORS allowlist, secure response headers, request body limit, per-IP rate limiting (20 req/min on auth routes, 300 req/min elsewhere)
- **Two storage backends** — SQLite (zero-ops, default) and PostgreSQL (production-ready); switch with `DB_DRIVER=postgres`
- **OpenAPI 3.0 spec** + interactive ReDoc browser UI at `/docs`

---

## Tech Stack

| Concern | Choice |
|---|---|
| Language | Go 1.26+ |
| HTTP router | [Chi v5](https://github.com/go-chi/chi) |
| SQLite driver | `modernc.org/sqlite` — pure Go, no CGO |
| PostgreSQL driver | `jackc/pgx/v5` |
| Auth | `golang-jwt/jwt/v5` + `golang.org/x/crypto` (bcrypt) |
| Logging | `log/slog` (stdlib) |
| Testing | `testify` + `net/http/httptest` |
| Containerisation | Docker (multi-stage, ~15 MB image) + Docker Compose |
| CI | GitHub Actions (lint → test → Docker build) |

---

## Architecture

The codebase follows **Hexagonal (Ports & Adapters)** architecture:

```
cmd/server/          — entry point: wires everything, starts HTTP server
internal/
  domain/            — pure business logic; zero external dependencies
    ingredient/      — entities, UnitMap, Repository port
    food/            — entities, nutrition domain service
    mealplan/        — random selection algorithm
    shoppinglist/    — aggregation and grouping service
    user/            — User entity, Repository port
  application/       — use-cases that orchestrate domain + call ports
  adapters/
    primary/http/    — HTTP handlers, middleware, router (driving adapter)
    secondary/
      sqlite/        — SQLite implementation of all repository interfaces
      postgres/      — PostgreSQL implementation (same interfaces)
      seed/          — fixture loader from JSON files
  infrastructure/
    config/          — env-based config (12-factor)
    database/        — DB open + migrations runner
    auth/            — JWT sign/verify
migrations/
  sqlite/            — versioned up/down SQL pairs for SQLite
  postgres/          — versioned up/down SQL pairs for PostgreSQL
api/
  openapi.yaml       — OpenAPI 3.0.3 spec (embedded in binary)
```

Both storage adapters implement the same domain `Repository` interfaces. Switching from SQLite to PostgreSQL requires only two env var changes — no code changes.

---

## Quick Start

### Prerequisites

- Go 1.26+
- (Optional) Docker & Docker Compose

### Run locally with SQLite

```bash
# Copy env template and fill in JWT_SECRET (any random string)
cp .env.example .env

# Run the server (migrations run automatically on startup)
make dev

# Seed the database with sample ingredients and foods
make seed
```

The server starts on `http://localhost:8080` by default.

### Run with Docker (SQLite)

```bash
make docker-up
```

### Run with Docker + PostgreSQL

```bash
make docker-up-pg
```

---

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `ENV` | `development` | `development` or `production` |
| `DB_DRIVER` | `sqlite` | `sqlite` or `postgres` |
| `DB_PATH` | `./foodscheduler.db` | SQLite file path |
| `DB_URL` | — | PostgreSQL DSN (required when `DB_DRIVER=postgres`) |
| `JWT_SECRET` | — | **Required.** Signing key for HS256 JWTs |
| `CORS_ORIGINS` | `*` | Comma-separated list of allowed origins |
| `USDA_API_KEY` | — | Optional; required only for `make enrich` |

See [.env.example](.env.example) for the full list.

---

## API Overview

Interactive docs: `http://localhost:8080/docs`  
Raw spec: `http://localhost:8080/openapi.yaml`

All responses are `Content-Type: application/json`.  
Errors return `{ "error": "message", "code": "SNAKE_CASE_CODE" }`.

### Auth (public)
```
POST /v1/auth/register    { email, password }
POST /v1/auth/login       { email, password }  → { access_token, refresh_token }
POST /v1/auth/refresh     { refresh_token }    → { access_token, refresh_token }
```

### Ingredients (read: any authenticated user; write: admin)
```
GET    /v1/ingredients           ?food_group=&allergen_free=&search=
GET    /v1/ingredients/:id
POST   /v1/ingredients           [admin]
PUT    /v1/ingredients/:id       [admin]
DELETE /v1/ingredients/:id       [admin]
```

### Foods (read: authenticated; write: admin)
```
GET    /v1/foods                 ?label=&allergen_free=&search=
GET    /v1/foods/:id
POST   /v1/foods                 [admin]
PUT    /v1/foods/:id             [admin]
DELETE /v1/foods/:id             [admin]
```

### User (authenticated)
```
GET  /v1/me
GET  /v1/me/preferences
PUT  /v1/me/preferences          { excluded_allergens: [], dietary_restrictions: [] }
```

### Meal Plan (authenticated)
```
POST /v1/meal-plan
Body: {
  "count": 5,
  "allergen_free": ["gluten"],
  "labels": ["vegetarian"],
  "preferences": { "min_beef": 1, "min_chicken": 1 }
}
```

### Shopping List (authenticated)
```
POST /v1/shopping-list
Body: { "food_ids": ["uuid1", "uuid2"] }
```

### Health (public)
```
GET /health  → { "status": "ok", "db": "ok" }
```

---

## Makefile Targets

```bash
make build          # compile binary to bin/foodscheduler
make dev            # run server (no hot-reload)
make test           # run all tests
make test-unit      # run only short/unit tests
make test-int       # run integration tests
make lint           # golangci-lint
make seed           # load sample fixtures into DB
make enrich         # interactive USDA nutrition lookup for un-enriched ingredients
make docker-build   # build Docker image
make docker-up      # docker compose up (SQLite)
make docker-up-pg   # docker compose up with PostgreSQL profile
make docker-down    # tear down compose stack
make clean          # remove bin/
```

---

## Running Tests

```bash
# All tests (SQLite integration tests use :memory:, no setup needed)
go test ./...

# PostgreSQL integration tests (requires a running PG instance)
TEST_PG_URL=postgres://user:pass@localhost:5432/foodscheduler_test?sslmode=disable \
  go test ./internal/adapters/secondary/postgres/...
```

97 tests, 0 failures.

---

## Data Model

### Key tables

| Table | Purpose |
|---|---|
| `ingredients` | Master ingredient catalogue with `unit_map`, `base_unit`, and per-base nutrition values |
| `foods` | Food recipes with computed nutrition totals |
| `food_ingredients` | Junction: which ingredients a food uses, and in what amount/unit |
| `users` | User accounts with bcrypt-hashed passwords and a `role` column |
| `user_preferences` | Per-user allergen exclusions and dietary restrictions (one-to-one with `users`) |

The `unit_map` on each ingredient is a JSON object that maps unit names to their base-unit multiplier (e.g. `{"grams": 1, "cups": 200}`). This lets recipes specify ingredients in any unit while nutrition sums always work in a single base unit.

---

## Nutrition Enrichment (optional)

Ingredient nutrition data can be populated interactively from the [USDA FoodData Central](https://fdc.nal.usda.gov/) API:

```bash
export USDA_API_KEY=your_key_here
make enrich
# For each un-enriched ingredient, prints candidates and asks you to pick the best match.
# Safe to re-run — only processes ingredients with NULL nutrition fields.
```
