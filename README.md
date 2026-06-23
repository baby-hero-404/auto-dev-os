# Auto Code OS

Auto Code OS is an AI-assisted SDLC platform that coordinates agents to analyze tasks, draft specs, implement code, run tests, create pull requests, and wait for explicit human merge approval.

## Current Scope

- Task workflow from context loading through analysis, coding, review, testing, and PR readiness.
- Git integration for clone, commit, push, PR creation, and merge.
- Project, rule, agent, analytics, gateway, and review management in the web UI.
- Skills are being refactored toward Git-synced, read-only sources.

For feature-level status and implementation notes, see `docs/features/`.

## Tech Stack

- Backend: Go 1.26.1, Chi, GORM, PostgreSQL, Docker
- Frontend: Next.js 16.2.6, React 19.2.4, TypeScript, Tailwind CSS v4, Radix UI, Lucide, Recharts
- Testing: Go test suite and Playwright

## Repository Layout

```text
/
├── server/                # Go backend
│   ├── cmd/               # CLI, API, and migration entry points
│   ├── internal/          # handlers, services, repository, orchestrator, sandbox, workflow
│   └── pkg/               # shared models, config, and LLM types
├── web/                   # Next.js web app
│   ├── src/app/           # application routes
│   ├── src/components/    # shared UI components
│   └── src/lib/           # client helpers and API types
├── docs/                  # feature docs and implementation notes
├── docker-compose.yml     # local infrastructure
└── Makefile               # common dev and test commands
```

## Prerequisites

- Go 1.26+
- Node.js 20+ with npm
- Docker and Docker Compose

## Setup

1. Clone the repository.
2. Copy the example environment file:
   ```bash
   cp .env.example .env
   ```
3. Install frontend dependencies:
   ```bash
   cd web
   npm install
   cd ..
   ```

## Configuration

See `.env.example` for the full list. Common variables:

| Variable | Purpose |
| --- | --- |
| `SERVER_PORT` | Go API port |
| `WEB_PORT` | Next.js dev server port |
| `DATABASE_URL` | PostgreSQL connection string |
| `LLM_PROVIDER` | Active LLM routing mode |
| `OPENAI_API_KEY` | OpenAI credential |
| `ANTHROPIC_API_KEY` | Anthropic credential |
| `GEMINI_API_KEY` | Google Gemini credential |
| `SANDBOX_RUNTIME` | `stub` or `docker` sandbox driver |
| `SANDBOX_WORKSPACE_ROOT` | Workspace root used by task execution |

## Running Locally

Start the full stack:

```bash
make dev
```

Typical local URLs:

- API: `http://localhost:8080`
- Web UI: `http://localhost:32300`

Other useful targets:

```bash
make dev-be   # database, migrations, and API server
make dev-fe   # Next.js frontend only
make api      # Go API server
make web      # Next.js dev server
make migrate  # run database migrations
```

## Testing

Backend:

```bash
make test
```

Frontend end-to-end:

```bash
cd web
npx playwright test
```

## Useful Commands

```bash
make build    # build the CLI binary
make clean    # remove build artifacts and local data
make db-up    # start PostgreSQL
make db-down  # stop containers
```

## Feature References

- `docs/features/5.1-unified-ai-gateway.md`
- `docs/features/5.2a-rule-system.md`
- `docs/features/5.2b-skill-system.md`
- `docs/features/5.3-agent-system.md`
- `docs/features/5.4-git-integration.md`
- `docs/features/5.5-project-system.md`
- `docs/features/5.6-task-system.md`
- `docs/features/5.7-workflow-engine.md`
- `docs/features/5.8-pr-human-review.md`
- `docs/features/5.9-dashboard-analytics.md`
- `docs/features/5.10-multi-channel-interaction.md`
