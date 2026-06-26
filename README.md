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

Frontend checks:

```bash
cd web
npm run lint
npm run build
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

## Development Best Practices

### Work From The Feature Contract

- Treat `docs/features/` as the behavioral source of truth before changing workflow, task, agent, repository, PR, or analytics behavior.
- Keep implementation, UI labels, and TypeScript types aligned with the canonical task lifecycle:
  `todo -> context_loading -> analyzing -> spec_review -> coding -> reviewing -> fixing -> testing -> pr_ready -> human_review -> merged`, with `failed` as the terminal error state.
- Preserve the rule that PR creation does not mean task completion. A task is complete only after explicit human approval and merge.
- Easy tasks should follow the easy graph: `context_load -> analyze -> code_backend -> test -> pr`.
- Medium and hard tasks should follow the standard graph: `context_load -> analyze -> plan -> code_backend/code_frontend -> merge -> review -> fix -> test -> pr`.

### Backend Guidelines

- Keep HTTP handlers thin: parse input, call services/orchestrator, and return structured responses.
- Keep business rules in services or policy packages, not in handlers or repositories.
- Keep repositories focused on GORM-backed persistence and return domain models from `server/pkg/models`.
- Use constants from `server/pkg/models` and `server/internal/workflow` for task statuses, spec statuses, workflow steps, and workflow step states.
- When adding orchestrator behavior, make it durable: update task/job state, create checkpoints, save artifacts when useful, and keep retry/resume behavior intact.
- Do not bypass workspace locking, sandbox execution, or redaction for agent-triggered filesystem, Git, or command execution.

### Frontend Guidelines

- Keep API response types in `web/src/lib/types.ts` aligned with Go models and JSON responses.
- Prefer shared helpers in `web/src/lib/utils/` for task status, completion, and display logic instead of duplicating status arrays in components.
- Task and monitor screens should show workflow state from persisted jobs/checkpoints, not inferred local-only state.
- Use the project default branch when linking repositories, but let users override it per repository.
- Keep UI actions consistent with backend contracts: task actions should preserve success/failure signals when the caller uses them.
- Run `npm run lint` and `npm run build` after UI type, status, routing, or component changes.

### Testing Expectations

- For backend changes, run the narrow package tests first, then `make test` before merging.
- For orchestrator or workflow changes, include tests for state transitions, checkpoint/resume behavior, retry paths, and PR/human-review gates.
- For UI changes, run `npm run lint`, `npm run build`, and Playwright when changing routed pages or core user flows.
- For API contract changes, verify both Go tests and TypeScript build so backend responses and frontend types stay compatible.

### Git, Security, And Sandbox Safety

- Never commit real API keys, provider tokens, Git tokens, private keys, or workspace secrets.
- Prefer Git accounts and repository-linked credentials over hardcoded tokens.
- Keep sandbox runtime behavior isolated from the host; agent code execution should go through the configured sandbox driver.
- Preserve audit artifacts for terminal tasks. Cleanup should remove disposable worktrees without deleting logs, diffs, specs, checkpoints, or metadata needed for debugging.
- Review changes to auth, RBAC, payments, migrations, public API contracts, infrastructure, and secrets handling as high-risk work that requires human approval.

### Documentation Hygiene

- Update the relevant file under `docs/features/` when behavior changes.
- Update `docs/ARCHITECTURE.md` when module boundaries, runtime architecture, or core data flow changes.
- Keep README instructions practical and current: setup, commands, verification, and links to deeper docs.
- Prefer small, focused docs updates near the feature they describe over broad duplicated explanations.

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
