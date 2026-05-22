.PHONY: build run clean test api db-up db-down migrate clone-resource

# ── Build ────────────────────────────────────────────────
build:
	cd server && go build -o ../bin/auto-code-os ./cmd/cli

# ── Run (PoC) ────────────────────────────────────────────
run:
	cd server && go run ./cmd/cli $(ARGS)

# ── API Server ───────────────────────────────────────────
api:
	cd server && go run ./cmd/api

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
	@echo "  make db-up                    Start PostgreSQL container"
	@echo "  make db-down                  Stop and remove containers"
	@echo "  make test                     Run all tests"
	@echo "  make clean                    Remove build artifacts"
	@echo "  make clone-resource           Clone external repositories into resources directory"
