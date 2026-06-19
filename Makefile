-include .env
export

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.2.0-dev")
LDFLAGS = -ldflags "-X github.com/auto-code-os/auto-code-os/server/internal/handler.Version=$(VERSION)"

.PHONY: build run clean test api web dev dev-be dev-fe db-up db-down migrate db-clean clone-resource

# ── Build ────────────────────────────────────────────────
build:
	cd server && go build $(LDFLAGS) -o ../bin/auto-code-os ./cmd/cli

# ── Run (PoC) ────────────────────────────────────────────
run:
	cd server && go run $(LDFLAGS) ./cmd/cli $(ARGS)

# ── API Server ───────────────────────────────────────────
api:
	cd server && go run $(LDFLAGS) ./cmd/api

# ── Web UI ───────────────────────────────────────────────
web:
	cd web && NEXT_PUBLIC_API_URL=http://localhost:$(SERVER_PORT)/api/v1 PORT=$(WEB_PORT) npm run dev

dev:
	@echo "Clearing port conflicts..."
	-@for port in $(SERVER_PORT) $(WEB_PORT); do \
		pids=$$(lsof -t -i :$$port 2>/dev/null); \
		if [ ! -z "$$pids" ]; then \
			echo "Killing lsof processes on port $$port: $$pids"; \
			kill -9 $$pids 2>/dev/null || true; \
		fi; \
		sspids=$$(ss -tulpn 2>/dev/null | grep -E ":$$port\b" | grep -oE "pid=[0-9]+" | cut -d= -f2); \
		if [ ! -z "$$sspids" ]; then \
			echo "Killing ss processes on port $$port: $$sspids"; \
			kill -9 $$sspids 2>/dev/null || true; \
		fi; \
		cid=$$(docker ps -q --filter "publish=$$port" 2>/dev/null); \
		if [ ! -z "$$cid" ]; then \
			echo "Stopping container $$cid on port $$port..."; \
			docker stop $$cid 2>/dev/null || true; \
		fi; \
	done
	@sleep 1
	make db-up
	$(MAKE) -j2 api web

# ── Development targets ──────────────────────────────────
dev-be: db-up
	$(MAKE) migrate
	$(MAKE) api

dev-fe:
	$(MAKE) web

migrate: db-up
	sleep 3
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
clean: db-clean
	rm -rf bin/
	rm -rf .data/
	rm -rf server/.data/

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
	@echo "  make clean                    Remove build artifacts, database volumes, and on-disk data"
	@echo "  make clone-resource           Clone external repositories into resources directory"
