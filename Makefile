# ── Auto Code OS Makefile ─────────────────────────────────────────────────────
# A developer onboarding and workflow automation command center.

-include .env
export

# Default port configurations
SERVER_PORT ?= 8080
WEB_PORT ?= 3000

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.2.0-dev")
LDFLAGS = -ldflags "-X github.com/auto-code-os/auto-code-os/server/internal/handler.Version=$(VERSION)"

.PHONY: help init build run clean test test-be test-fe lint fmt api web dev dev-be dev-fe db-up db-down db-clean migrate sandbox-build clone-references rollout-gate

# Default target displays the help menu
.DEFAULT_GOAL := help

# ── Setup & Initialization ───────────────────────────────────────────────

init: ## Setup local environment (configs & deps)
	@echo "==> Setting up environment..."
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
		echo "Created .env from .env.example"; \
	else \
		echo ".env already exists"; \
	fi
	@echo "==> Installing web dependencies..."
	cd web && npm install
	@echo "==> Installing server dependencies & syncing vendor..."
	cd server && go mod tidy && go mod vendor
	@echo "==> Initialization complete. Please update your .env file with necessary keys."

clone-references: ## Clone external test/skill references
	@echo "==> Cloning external references..."
	bash scripts/clone_references.sh

# ── Infrastructure & Database ────────────────────────────────────────────

db-up: ## Start PostgreSQL database container
	@echo "==> Starting database container..."
	docker compose up -d postgres

db-down: ## Stop and remove Docker compose containers
	@echo "==> Stopping Docker containers..."
	docker compose down

db-clean: ## Destroy database volumes and containers
	@echo "==> Destroying database volumes and containers..."
	docker compose down -v

migrate: db-up ## Run database migrations
	@echo "==> Running database migrations..."
	@sleep 2
	cd server && go run ./cmd/migrate

rollout-gate: ## Evaluate state machine rollout gate (e.g. make rollout-gate SAMPLE=100 THRESHOLD=2.0). THRESHOLD defaults to execution.rollout_violation_threshold_pct from config when omitted.
	@echo "==> Running rollout gate check..."
	cd server && go run ./cmd/rollout-gate -sample $(or $(SAMPLE),100) $(if $(THRESHOLD),-threshold $(THRESHOLD),)

# ── Build & Sandbox ──────────────────────────────────────────────────────

build: ## Build the CLI binary
	@echo "==> Building CLI binary..."
	cd server && go build $(LDFLAGS) -o ../bin/auto-code-os ./cmd/cli

sandbox-build: ## Build the Docker sandbox image for agents
	@echo "==> Building agent Docker sandbox image..."
	docker build -t auto-code-os-sandbox:latest -f docker/Dockerfile.sandbox .

# ── Running Development Servers (Host-Direct + Docker DB) ────────────────

port-clean:
	@echo "==> Checking and clearing port conflicts on ports $(SERVER_PORT) and $(WEB_PORT)..."
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

dev: port-clean db-up migrate ## Start full stack locally (DB in Docker, API and Web on Host)
	@echo "==> Starting full stack..."
	$(MAKE) -j2 api web

dev-be: db-up migrate api ## Run database, apply migrations, and start Go API server

dev-fe: web ## Start Next.js web UI dev server only

api: ## Run Go API server on Host
	@echo "==> Starting Go API server on port $(SERVER_PORT)..."
	cd server && go run $(LDFLAGS) ./cmd/api

web: ## Run Next.js web app on Host
	@echo "==> Starting Next.js Web dev server on port $(WEB_PORT)..."
	cd web && NEXT_PUBLIC_API_URL=http://localhost:$(SERVER_PORT)/api/v1 PORT=$(WEB_PORT) npm run dev

run: ## Run the CLI (PoC mode) with ARGS='--task "..."'
	cd server && go run $(LDFLAGS) ./cmd/cli $(ARGS)

# ── Quality, Formatting & Testing ─────────────────────────────────────────

test: test-be test-fe ## Run Go backend tests and Playwright E2E frontend tests

test-be: ## Run Go backend tests only
	@echo "==> Running backend Go tests..."
	cd server && go test ./... -v -count=1

test-fe: ## Run Playwright frontend tests only
	@echo "==> Running frontend Playwright tests..."
	cd web && npx playwright test

lint: ## Run backend and frontend linters
	@echo "==> Running linters..."
	cd server && go vet ./...
	cd web && npm run lint

fmt: ## Format Go and TypeScript/JS code
	@echo "==> Formatting code..."
	cd server && go fmt ./...

# ── Clean Up ─────────────────────────────────────────────────────────────

clean: db-clean ## Remove build binaries, logs, and database containers/volumes
	@echo "==> Cleaning build artifacts and temporary files..."
	rm -rf bin/
	rm -rf .data/
	rm -rf server/.data/

# ── Help ─────────────────────────────────────────────────────────────────

help: ## Display this help screen
	@echo "Auto Code OS - Onboarding & Development Workflow Commands"
	@echo ""
	@echo "Usage:"
	@echo "  make <target> [variables]"
	@echo ""
	@echo "Targets:"
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Manual Command Summary:"
	@echo "  make init              - Copy environment template and install web dependencies"
	@echo "  make sandbox-build     - Build the agent Docker sandbox container image"
	@echo "  make dev               - Spin up DB in Docker, migrate, start backend and frontend on host"
	@echo "  make test              - Run backend and frontend tests"
	@echo "  make clean             - Stop and wipe database, and remove build/cache directories"
