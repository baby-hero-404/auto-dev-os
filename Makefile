-include .env
export

.PHONY: build run clean test api web dev dev-be dev-fe db-up db-down migrate db-clean clone-resource

# ── Build ────────────────────────────────────────────────
build:
	cd server && go build -o ../bin/auto-code-os ./cmd/cli

# ── Run (PoC) ────────────────────────────────────────────
run:
	cd server && go run ./cmd/cli $(ARGS)

# ── API Server ───────────────────────────────────────────
api:
	cd server && go run ./cmd/api

# ── Web UI ───────────────────────────────────────────────
web:
	cd web && NEXT_PUBLIC_API_URL=http://localhost:$(SERVER_PORT)/api/v1 PORT=$(WEB_PORT) npm run dev

dev:
	make db-up
	$(MAKE) -j2 api web

# ── Development targets ──────────────────────────────────
dev-be: db-up
	$(MAKE) migrate
	$(MAKE) api

dev-fe:
	$(MAKE) web

migrate:
	cd server && go run ./cmd/migrate

# ── Database ─────────────────────────────────────────────
db-up:
	docker compose up -d postgres

db-down:
	docker compose down

db-clean:
	docker compose down -v

# ── Test ─────────────────────────────────────────────────
test:
	cd server && go test ./... -v -count=1
	cd web && npx playwright test

# ── Clean ────────────────────────────────────────────────
clean:
	rm -rf bin/

# ── Resources ────────────────────────────────────────────
clone-resource:
	bash scripts/clone_resources.sh

# ── Help ─────────────────────────────────────────────────
help:
	@echo "Usage:"
	@echo "  make build                    Build the CLI binary"
	@echo "  make run ARGS='--task \"...\"'  Run the CLI with arguments"
	@echo "  make api                      Run the API server"
	@echo "  make web                      Run the Next.js web UI"
	@echo "  make dev                      Run database, API, and web UI"
	@echo "  make dev-be                   Run database, run migrations, and run API server"
	@echo "  make dev-fe                   Run Next.js web UI dev server"
	@echo "  make migrate                  Run database migrations"
	@echo "  make db-up                    Start PostgreSQL container"
	@echo "  make db-down                  Stop and remove containers"
	@echo "  make test                     Run all tests"
	@echo "  make clean                    Remove build artifacts"
	@echo "  make clone-resource           Clone external repositories into resources directory"
