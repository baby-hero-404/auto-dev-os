# Project Overview — Auto Code OS

- **Name**: Auto Code OS
- **Type**: AI-Native SDLC Platform / Monorepo
- **Language/Runtime**: Go 1.26+ (Backend), TypeScript & Node 20+ (Frontend)
- **Framework**: chi/v5 (Go Router), Next.js 16 (App Router)
- **Database**: PostgreSQL 17 + pgvector, migrations via golang-migrate, pgx/v5 driver
- **Architecture**: Clean 3-layer Go backend (Handler → Service → Repository) + Next.js App Router Frontend

## Key Architectural Decisions
- **Docker-based Sandbox isolation**: Isolates agent execution runtime (volume mounts, log streams, network proxy rules) utilizing the official Docker Go SDK.
- **Secure Secret Vault**: Secrets are AES-GCM encrypted in PostgreSQL and decrypted only during sandbox environment runtime initialization.
- **Task Queue & Orchestrator**: Uses PostgreSQL `SKIP LOCKED` for queue-based task management. Execution flow varies by complexity: Easy tasks auto-approve; Medium/Hard tasks trigger a Human-in-the-Loop (HITL) review loop.
- **AI Gateway & Tier Routing**: An internal LLM gateway (`pkg/llm`) maps task tiers to models (fast/cost-efficient for easy tasks, smart/capable for hard tasks) and implements fallback chains and cost circuit breakers.
- **Workflow Engine (DAG)**: A Go-compiled step registry DAG runner supports parallel task orchestration, log streaming (SSE), and execution checkpointing.

## Entry Points
- **API Server Entry**: `server/cmd/api/main.go`
- **CLI PoC Entry**: `server/cmd/cli/main.go`
- **Frontend Page Entry**: `web/app/layout.tsx`, `web/app/page.tsx`
- **Database Migrations**: `server/migration/`

## Key Environment & Config
- **Configuration loading**: Managed via `server/pkg/config/`
- **Critical Variables**:
  - `DATABASE_URL`: PostgreSQL connection string.
  - `DOCKER_HOST`: URI for the Docker daemon.
  - `AES_KEY`: 32-byte key for GCM secret encryption.
  - LLM Provider Keys: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY`.
