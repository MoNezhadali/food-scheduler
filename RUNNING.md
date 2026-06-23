# Running the FoodScheduler Backend

## Prerequisites

- Go 1.26+ — verify with `go version`

---

## 1. Configure the environment

```bash
cp .env.example .env
```

Open `.env` and set the required values:

```env
JWT_SECRET=any-random-string-at-least-32-chars
PORT=8080
DB_DRIVER=sqlite
DB_PATH=./foodscheduler.db
```

All other variables are optional for local development.

---

## 2. Start the server

```bash
make dev
```

Database migrations run automatically on startup. You should see a JSON log line confirming the server is listening on port 8080.

---

## 3. Seed sample data

In a second terminal:

```bash
make seed
```

This inserts 19 ingredients and 3 sample foods. It is idempotent — safe to run more than once.

---

## 4. Promote a user to admin

User accounts created via `/v1/auth/register` default to `role: "user"`. Admin role is required to create, update, or delete foods and ingredients.

```bash
sqlite3 foodscheduler.db "UPDATE users SET role='admin' WHERE email='your@email.com';"
```

Log out and back in so the new role is reflected in your JWT.

---

## Running with PostgreSQL instead of SQLite

```bash
# In .env:
DB_DRIVER=postgres
DB_URL=postgres://user:pass@localhost:5432/foodscheduler?sslmode=disable
```

Or with Docker Compose (starts a Postgres container automatically):

```bash
make docker-up-pg
```

---

## Useful endpoints

| URL | Description |
|---|---|
| `http://localhost:8080/health` | Health check — confirms DB is reachable |
| `http://localhost:8080/docs` | Interactive API browser (ReDoc) |
| `http://localhost:8080/openapi.yaml` | Raw OpenAPI 3.0 spec |

---

## All Makefile targets

```bash
make dev            # run server
make build          # compile binary to bin/foodscheduler
make test           # run all tests
make seed           # load sample fixtures into DB
make enrich         # interactive USDA nutrition lookup for un-enriched ingredients
make migrate-up     # apply pending migrations manually
make migrate-down   # roll back last migration
make lint           # golangci-lint
make docker-build   # build Docker image
make docker-up      # docker compose up with SQLite
make docker-up-pg   # docker compose up with PostgreSQL
make docker-down    # tear down compose stack
make clean          # remove bin/
```

---

## Stopping the server

`Ctrl+C` in the terminal running `make dev`.

The SQLite database persists at `./foodscheduler.db` between restarts.
