# Project Index — Auto Code OS

## Entry Points
- [server/cmd/api/main.go](file:///home/ubuntu/my_projects/auto_code_os/server/cmd/api/main.go) — Main API server entry point
- [server/cmd/cli/main.go](file:///home/ubuntu/my_projects/auto_code_os/server/cmd/cli/main.go) — Legacy CLI PoC entry point
- [web/app/page.tsx](file:///home/ubuntu/my_projects/auto_code_os/web/app/page.tsx) — Web dashboard index page

## Database & Migrations
- [server/migration/](file:///home/ubuntu/my_projects/auto_code_os/server/migration/) — SQL up/down migration scripts
- [server/internal/database/](file:///home/ubuntu/my_projects/auto_code_os/server/internal/database/) — Database connection setup

## Domain Handlers (Go REST API)
- [server/internal/handler/](file:///home/ubuntu/my_projects/auto_code_os/server/internal/handler/) — HTTP endpoints (Org, Project, Task, Agent, Rules, Skills, Memories, PRs)

## Business Services (Go Core Logic)
- [server/internal/service/](file:///home/ubuntu/my_projects/auto_code_os/server/internal/service/) — Core business services (Task parsing, Git client, Secret Vault, staff pool)
- [server/internal/orchestrator/](file:///home/ubuntu/my_projects/auto_code_os/server/internal/orchestrator/) — Execution queue, Prompt assembly, Tier routing
- [server/internal/sandbox/](file:///home/ubuntu/my_projects/auto_code_os/server/internal/sandbox/) — Docker container executor
- [server/internal/workflow/](file:///home/ubuntu/my_projects/auto_code_os/server/internal/workflow/) — Hybrid DAG workflow runner

## Shared Packages & Models
- [server/pkg/models/](file:///home/ubuntu/my_projects/auto_code_os/server/pkg/models/) — Domain model definitions (Structs for Task, Project, Rule, Skill)
- [server/pkg/llm/](file:///home/ubuntu/my_projects/auto_code_os/server/pkg/llm/) — LLM Client router, cost tracker, and provider abstraction

## Next.js Frontend App
- [web/app/](file:///home/ubuntu/my_projects/auto_code_os/web/app/) — Next.js routing pages
- [web/components/](file:///home/ubuntu/my_projects/auto_code_os/web/components/) — Frontend UI components (DAG viewer, analytics graphs)
