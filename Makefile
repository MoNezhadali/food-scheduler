BINARY = foodscheduler
CMD    = ./cmd/server

.PHONY: build dev test lint clean \
        migrate-up migrate-down seed enrich \
        docker-build

## ── Build ────────────────────────────────────────────────────────────────────

build:
	go build -o bin/$(BINARY) $(CMD)

dev:
	go run $(CMD)

## ── Test & Lint ──────────────────────────────────────────────────────────────

test:
	go test ./...

test-unit:
	go test -short ./...

test-int:
	go test -run Integration ./... -count=1

lint:
	golangci-lint run ./...

## ── Database ─────────────────────────────────────────────────────────────────
# These targets are wired up in Phase 3 (migrations) and Phase 4 (seed).

migrate-up:
	go run ./cmd/migrate -dir=migrations/sqlite -action=up

migrate-down:
	go run ./cmd/migrate -dir=migrations/sqlite -action=down

seed:
	go run ./cmd/seed

## ── Nutrition enrichment (Phase 4b) ─────────────────────────────────────────
# Loads ingredients with missing nutrition from DB, looks them up via USDA,
# and prompts you to confirm each match before writing back.

enrich:
	go run ./cmd/enrich

## ── Docker ───────────────────────────────────────────────────────────────────
# Dockerfile added in Phase 13.

docker-build:
	docker build -t $(BINARY):latest .

## ── Housekeeping ─────────────────────────────────────────────────────────────

clean:
	rm -rf bin/
