# Auto Code OS

Auto Code OS is an AI-Native Software Development Life Cycle (SDLC) platform that orchestrates autonomous AI agents to analyze tasks, draft specifications, construct codebases (backend & frontend), perform local testing, generate Pull Requests, and execute human review loops in a secure sandboxed environment.

---

## Features

### 1. Autonomous Execution Pipeline
- **Orchestrator Core**: A robust task execution state machine supporting multi-step DAG workflows.
- **Org-Scoped Agent Pools**: Manage special-purpose AI workers (planners, coders, testers, and reviewers) matching task profiles.
- **Docker Sandbox Worktree**: Securely isolates execution scripts and file modifications with custom memory limits, CPU bounds, and network isolation policies.

### 2. AI Gateway & Cost Circuit Breaker
- **Tier-based Routing**: Automatically routes tasks to LLMs based on complexity (`easy` -> fast models, `hard` -> powerful reasoning models).
- **Multi-Provider Fallback**: Transparently cascades requests between OpenAI, Anthropic, and Gemini in case of provider rate limits or downtime.
- **Runaway Budget Protection**: Restricts agent costs using active tokens and USD circuit breakers.

### 3. Developer Skill System
- **Markdown-Based Portable Playbooks**: Portable skill definitions leveraging standard JSON Schema tool contracts.
- **Search-and-Replace patches (`apply_patch`)**: Minimizes context tokens by applying precise search-and-replace block changes instead of full file rewrites.
- **Judge-based Evals**: CI/CD integration with LLM-as-a-judge formatting to validate prompt quality and skill outputs against golden datasets.

### 4. Human-in-the-Loop Decision Gate
- **Interactive Review Card**: Direct interface to inspect AI-generated PR summaries, risk level badges, modified files list, and interactive mock code diff editors.
- **Repair Loop Rejection**: Reject PRs with explicit feedback, automatically transitioning tasks back to a fixing execution status for autonomous repair.
- **Compliant Audit Tracing**: Real-time compliance logging tracking user actions, secret accesses, agent status shifts, and policy edits.

---

## Tech Stack

- **Backend Language**: Go 1.26+
- **Backend Framework**: Chi Router v5 (3-layer design pattern: Handler -> Service -> Repository)
- **Database**: PostgreSQL 17 + pgvector (Migrations handled via `golang-migrate`)
- **Frontend Framework**: Next.js 16 (App Router, TypeScript, React 19)
- **Frontend Styling & Visuals**: Tailwind CSS v4, Recharts, Lucide React
- **Infrastructure**: Docker & Docker Compose

---

## Architecture Overview

```mermaid
graph TD
    User([Platform Reviewer]) -->|Manages Tasks / Reviews PRs| WebUI[Next.js 16 Web Portal]
    WebUI -->|API Requests| APIServer[Go Chi Monolith Server]
    APIServer -->|Aggregates & Metrics| DB[(PostgreSQL 17 + pgvector)]
    APIServer -->|Triggers Execution| Orchestrator[Workflow DAG Orchestrator]
    Orchestrator -->|Queries Agent Pool| DB
    Orchestrator -->|AES Decrypted Env Injection| SecretVault[Encrypted AES-GCM Vault]
    Orchestrator -->|Mounts Workspace| Sandbox[Docker Sandbox Runtime]
    Orchestrator -->|Optimized Tools| SkillSystem[Skill System Runtime]
    Orchestrator -->|Tiered Model Routing & Budget Checks| LLMGateway[LLM Gateway Router]
    LLMGateway -->|API Fallbacks| LLMProviders[OpenAI / Anthropic / Gemini]
```

---

## Project Structure

```text
/
├── server/                    # Go backend monorepo
│   ├── cmd/
│   │   ├── api/               # API server bootstrap entry point
│   │   ├── cli/               # Phase 0 CLI runner utility
│   │   └── migrate/           # Database migration runner CLI
│   ├── internal/
│   │   ├── handler/           # HTTP Request handlers & routing
│   │   ├── service/           # Core business logic services
│   │   ├── repository/        # GORM PostgreSQL repositories
│   │   ├── orchestrator/      # State machine pipeline & prompt builders
│   │   ├── sandbox/           # Container isolation layer (Docker client)
│   │   └── workflow/          # Compiled DAG Workflow Node Engine
│   ├── pkg/
│   │   ├── llm/               # Gateway routers, fallbacks, and token logs
│   │   ├── models/            # Shared database GORM struct models
│   │   └── config/            # Viper configuration manager
│   └── migration/             # SQL up/down migration definitions
├── web/                       # Next.js 16 React portal frontend
│   ├── src/app/               # Application routes (App Router)
│   │   ├── analytics/         # Observer dashboards
│   │   ├── audit/             # Compliance logging page
│   │   ├── gateway/           # Token gateway telemetry
│   │   └── tasks/[id]/        # Interactive progress & PR review loop
│   ├── src/components/        # Shared visual portal UI parts
│   └── src/lib/               # Axios/Fetch client and TypeScript typings
├── docs/                      # Implementation plans, roadmap & manual
├── docker-compose.yml         # Dev environment container orchestrator
└── Makefile                   # Tasks automation runner script
```

---

## Prerequisites

Ensure you have the following packages installed on your local OS:
- **Go**: v1.26+
- **Node.js**: v20+ (with npm)
- **Docker & Docker Compose**: v20.10+ (to mount containers and sandbox workspaces)

---

## Installation

### 1. Clone the repository
```bash
git clone <repository-url>
cd auto-code-os
```

### 2. Configure variables
Create your local environment file:
```bash
cp .env.example .env
```
Open `.env` and fill in your keys (see **Configuration** below).

### 3. Run database migrations
Ensure Docker daemon is running, start PostgreSQL, and run the SQL schema migrations:
```bash
make db-up
make migrate
```

---

## Configuration

| Variable | Description | Default / Example |
|----------|-------------|-------------------|
| `SERVER_PORT` | The port the Go API backend listens on | `8080` |
| `DATABASE_URL` | The PostgreSQL database connection string | `postgres://autocodeuser:autocodepass@localhost:5434/autocodeosdb?sslmode=disable` |
| `LLM_PROVIDER` | Active LLM routing mode (`openai`, `anthropic`, `gemini`, `gateway`) | `gateway` |
| `OPENAI_API_KEY` | API authentication key for OpenAI provider | `sk-proj-...` |
| `ANTHROPIC_API_KEY` | API authentication key for Anthropic provider | `sk-ant-...` |
| `GEMINI_API_KEY` | API authentication key for Google Gemini provider | `AIzaSy-...` |
| `SANDBOX_RUNTIME` | Sandbox runtime driver (`stub` for testing, `docker` for real sandbox) | `stub` |
| `SANDBOX_WORKSPACE_ROOT` | Workspace path where agent workspaces are stored | `/tmp/auto-code-os/workspaces` |

---

## Running the Project

### Development Mode (Both Backend & Frontend)
To run the database, run migrations, and launch both backend API and React Next.js dev server concurrently:
```bash
make dev
```
- Go Backend API: http://localhost:8080
- Next.js Web Portal: http://localhost:32300

### API Backend Only
To boot up the Go server API independently:
```bash
make api
```

### Frontend Next.js Only
To start the Next.js development server independently:
```bash
make web
```

---

## Testing

### Run Backend Unit & Integration Tests
To run all Go testing suites:
```bash
make test
```
To run frontend Playwright end-to-end user path tests:
```bash
cd web && npx playwright test
```

---

## Contributing

1. Fork the repository.
2. Create a clean feature branch (`git checkout -b feature/amazing-feature`).
3. Commit your changes (`git commit -m 'feat: Add amazing feature'`).
4. Push your branch (`git push origin feature/amazing-feature`).
5. Create a new Pull Request.

---

## License

This project is licensed under the MIT License.
