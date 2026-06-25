# Architecture — Auto Code OS

> AI-Native SDLC Platform — System Architecture & Technical Decisions

## 1. Tech Stack

| Layer              | Technology                          | Rationale                                                    |
| :----------------- | :---------------------------------- | :----------------------------------------------------------- |
| **Backend**        | Go 1.26.x                           | High performance, strong concurrency (goroutines for agents); matches `server/go.mod` |
| **Web UI**         | Next.js 16.x (App Router, TypeScript) | Active LTS, modern SSR/RSC, Turbopack defaults, great DX for dashboards |
| **Database**       | PostgreSQL 17 + pgvector            | Relational + vector search for RAG/memory in one DB          |
| **AI Gateway**     | Internal Go package (`pkg/llm`)     | Abstracts OpenAI, Anthropic, Google Gemini behind one interface |
| **Task Queue**     | PostgreSQL (SKIP LOCKED)            | No extra infra needed for MVP; upgrade to Temporal later     |
| **Git Integration**| Local git CLI + GitHub API          | Clone repos, create branches, push commits, open PRs; supports GitHub Enterprise API base URLs |
| **Workflow Engine**| Internal Go package (Phase 3)       | Orchestrates task → agent → sandbox → review → PR pipeline   |
| **Containerization** | Docker + Docker Compose           | Self-hosted deployment, agent sandboxing                     |

## 2. Monorepo Structure

```
auto_code_os/
├── server/                 # All backend Go code
│   ├── cmd/                # Entry points (main packages)
│   │   ├── api/            # HTTP/gRPC API server
│   │   │   └── main.go
│   │   └── cli/            # CLI tool (PoC entry point)
│   │       └── main.go
│   │
│   ├── internal/           # Private application logic (not importable externally)
│   │   ├── handler/        # HTTP/gRPC request handlers
│   │   ├── service/        # Business logic layer
│   │   ├── repository/     # Database access layer (PostgreSQL queries)
│   │   ├── orchestrator/   # Core workflow engine (task → agent → sandbox)
│   │   ├── sandbox/        # Agent execution isolation (Docker)
│   │   ├── gitops/         # Git operations (clone, branch, commit, PR)
│   │   ├── workflow/       # Workflow engine (task pipeline automation)
│   │   └── middleware/     # Auth, logging, CORS
│   │
│   ├── pkg/                # Shared Go packages (importable by other Go projects)
│   │   ├── llm/            # LLM provider abstraction
│   │   │   ├── provider.go # Interface definition
│   │   │   ├── openai.go   # OpenAI implementation
│   │   │   ├── anthropic.go# Anthropic implementation
│   │   │   ├── router.go   # Tier-based routing (easy→Haiku, hard→Opus)
│   │   │   └── gemini.go   # Google Gemini implementation
│   │   ├── models/         # Domain models (Task, Agent, Project, Rule)
│   │   └── config/         # Configuration loading
│   │
│   ├── migration/          # PostgreSQL migration files (golang-migrate)
│   │   └── 000001_init.up.sql
│   │
│   ├── go.mod
│   └── go.sum
│
├── web/                    # Next.js 16 frontend
│   ├── app/                # App Router pages
│   ├── components/         # React components
│   └── lib/                # API client, utilities
│
├── docker/                 # Dockerfiles
│   ├── Dockerfile.server
│   ├── Dockerfile.web
│   └── Dockerfile.sandbox  # Sandboxed agent runtime
│
├── docs/                   # Documentation
│   ├── ARCHITECTURE.md     # This file
│   ├── ROADMAP.md          # Full product roadmap
│   ├── features/           # Feature-specific documentation (§5.1-§5.11)
│   └── references/         # Learning reports & external project analysis
│
├── resources/              # Open-source reference projects (git cloned)
│
├── docker-compose.yml      # Local dev environment
├── .env.example            # Environment variables template
└── Makefile                # Common dev commands
```

## 3. High-Level Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Developer / User                             │
│                                                                     │
│   ┌──────────┐      ┌───────────────┐      ┌────────────────────┐   │
│   │  CLI     │      │ Chatbots (TG/ │      │ Web UI (Next.js)   │   │
│   │  (PoC)   │      │ Discord/Slack)│      │ Dashboard / Tasks  │   │
│   └────┬─────┘      └───────┬───────┘      └─────────┬──────────┘   │
│        │                    │                        │              │
│        ▼                    ▼                        ▼              │
│   ┌─────────────────────────────────────────────────────────┐       │
│   │              API Server (Go + Chi Router)                │       │
│   │                                                          │       │
│   │  ┌──────────┐  ┌──────────────┐  ┌───────────────────┐  │       │
│   │  │ Task     │  │ Project      │  │ Rule Engine       │  │       │
│   │  │ Handler  │  │ Handler      │  │ (Global + Local)  │  │       │
│   │  └──────────┘  └──────────────┘  └───────────────────┘  │       │
│   │                                                          │       │
│   │  ┌─────────────────────────────────────────────────────┐ │       │
│   │  │              Workflow Engine                         │ │       │
│   │  │  Plan → Code → Review → Fix → Test → PR → Merge    │ │       │
│   │  └──────────────────────┬──────────────────────────────┘ │       │
│   │                         │                                │       │
│   │              ┌──────────▼──────────┐                     │       │
│   │              │   Orchestrator      │                     │       │
│   │              │  (assign agents,    │                     │       │
│   │              │   parallel dispatch)│                     │       │
│   │              └──────────┬──────────┘                     │       │
│   └─────────────────────────┼────────────────────────────────┘       │
│                             │                                        │
│           ┌─────────────────┼─────────────────┐                      │
│           ▼                 ▼                  ▼                      │
│   ┌──────────────┐  ┌──────────────┐  ┌──────────────┐               │
│   │  Sandbox A   │  │  Sandbox B   │  │  Sandbox C   │               │
│   │  (Docker)    │  │  (Docker)    │  │  (Docker)    │               │
│   │              │  │              │  │              │               │
│   │  Agent:      │  │  Agent:      │  │  Agent:      │               │
│   │  Backend     │  │  Frontend    │  │  Reviewer    │               │
│   └──────┬───────┘  └──────┬───────┘  └──────┬───────┘               │
│          │                 │                  │                       │
│          └─────────────────┼──────────────────┘                       │
│                            ▼                                         │
│              ┌─────────────────────────┐                             │
│              │   LLM Gateway (pkg/llm) │                             │
│              │                         │                             │
│              │  Tier-based Routing &   │                             │
│              │  Protocol Normalization │                             │
│              │  Fallback & Quota Ctrl  │                             │
│              └─────────────────────────┘                             │
│                                                                      │
│              ┌───────────────────────────────────────┐               │
│              │  PostgreSQL + pgvector                │               │
│              │                                       │               │
│              │  Orgs │ Projects │ Repos │ Tasks │ Secrets    │               │
│              │  Agents │ Rules │ Skills │ Memories   │               │
│              │  Episodic Memory & User Modeling      │               │
│              └───────────────────────────────────────┘               │
│                                                                      │
│              ┌───────────────────────────────────────┐               │
│              │  Git Provider (GitHub / Enterprise)    │               │
│              │                                       │               │
│              │  Clone → Branch → Commit → Push → PR  │               │
│              └───────────────────────────────────────┘               │
└──────────────────────────────────────────────────────────────────────┘
```

## 4. Core Domain Models

| Model       | Description                                        | Key Fields                                          |
| :---------- | :------------------------------------------------- | :-------------------------------------------------- |
| **Organization** | Top-level tenant                              | `id`, `name`, `created_at`                          |
| **Project** | Groups repos, rules, agents                        | `id`, `org_id`, `name`, `description`               |
| **Repository** | Git repository linked to a project             | `id`, `project_id`, `url`, `provider`, `token`, `git_account_id` |
| **Task**    | Unit of work for an agent (supports sub-tasks)     | `id`, `project_id`, `title`, `status`, `complexity`, `analysis`, `spec_status` |
| **User**    | Developer / reviewer account                       | `id`, `email`, `password_hash`, `org_id`, `role`    |
| **Agent**   | AI worker (supports self-improving loop & subagents)| `id`, `org_id`, `name`, `role`, `goal`, `model_route`, `autonomy_level`, `context_config` |
| **Rule**    | Behavioral constraints & Sandbox directives          | `id`, `scope` (global/project), `content`, `enforcement` |
| **Skill**   | Reusable action an agent can perform               | `id`, `name`, `description`, `schema`               |
| **Memory**  | Episodic memory, semantic search, user modeling    | `id`, `agent_id`, `content`, `embedding` (vector)   |

## 4.1 Task Lifecycle — Complexity & Risk-based Branching

> Aligns with the SDLC workflow in `docs/ROADMAP.md` §2 and `docs/features/5.7-workflow-engine.md`.

```
                    ┌─────────────────────┐
                    │  Developer creates  │
                    │  Task + description │
                    └──────────┬──────────┘
                               ▼
                    ┌─────────────────────┐
                    │  Context Load:      │
                    │  checkout repo,     │
                    │  read conventions,  │
                    │  CI config, docs    │
                    └──────────┬──────────┘
                               ▼
                    ┌─────────────────────┐
                    │  AI Agent analyzes  │
                    │  & classifies task  │
                    │  (Easy/Medium/Hard) │
                    │  + risk assessment  │
                    │                     │
                    │  ⟳ Asks questions   │
                    │  if info is missing │
                    └──────────┬──────────┘
                               │
              ┌────────────────┼────────────────┐
              ▼                ▼                ▼
     ┌────────────────┐ ┌──────────────┐ ┌────────────────────┐
     │  🟢 EASY       │ │ 🟡 EASY      │ │  🟠🔴 MEDIUM/HARD  │
     │  + LOW-RISK    │ │ + HIGH-RISK  │ │                    │
     │                │ │              │ │  spec_status:      │
     │  Auto-approve  │ │ DỪNG chờ     │ │  PENDING_REVIEW    │
     │  spec → code   │ │ human review │ │        │           │
     └───────┬────────┘ └──────┬───────┘ │        ▼           │
             │                │          │  Human reviews     │
             │                │          │  spec + plan +     │
             │                │          │  risks + files     │
             │                │          │  spec_status:      │
             │                │          │  APPROVED          │
             │                │          └────────┬───────────┘
             │                │                   │
             └────────────────┴───────────────────┘
                              │
                              ▼
                    ┌──────────────────────────┐
                    │  PLAN → CODE (parallel   │
                    │  ownership if needed)    │
                    │  → MERGE → REVIEW →     │
                    │  FIX (bounded: max N)    │
                    │  → TEST + LINT + BUILD   │
                    │  → PR (pr_ready)         │
                    │  → HUMAN_REVIEW → MERGED │
                    └──────────────────────────┘
```

**Task `spec_status` values:**
| Value | Meaning |
|-------|--------|
| `NONE` | Task not yet analyzed |
| `DRAFT` | AI has produced analysis, not yet reviewed |
| `PENDING_REVIEW` | Medium/Hard or Easy + high-risk task waiting for human review |
| `CHANGES_REQUESTED` | Reviewer requested more info or changes |
| `APPROVED` | Spec finalized, ready to execute |
| `AUTO_APPROVED` | Easy + low-risk task — auto-validated by agent |

## 5. Rule Engine Architecture (Strict Layered Context)

```
┌──────────────────────────────────────────┐
│           Agent Prompt Assembly           │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │  SYSTEM PROMPT (Immutable)         │  │
│  │                                    │  │
│  │  • Global Rules (from DB)          │  │
│  │  • Agent Role Definition           │  │
│  │  • Core Safety Constraints         │  │
│  │                                    │  │
│  │  ⛔ Cannot be overridden           │  │
│  └────────────────────────────────────┘  │
│                                          │
│  ┌────────────────────────────────────┐  │
│  │  TASK CONTEXT (Dynamic)            │  │
│  │                                    │  │
│  │  • Local/Project Rules             │  │
│  │  • Task Description & Files        │  │
│  │  • Relevant Code Context (RAG)     │  │
│  │  • Relevant Memory                 │  │
│  │                                    │  │
│  │  ⚠️ Rejected if conflicts Global  │  │
│  └────────────────────────────────────┘  │
└──────────────────────────────────────────┘
```

## 6. File Dependency Map

| File/Package                    | Depends On                                            |
| :------------------------------ | :---------------------------------------------------- |
| `server/cmd/cli/main.go`       | `server/pkg/llm`, `server/pkg/config`                 |
| `server/cmd/api/main.go`       | `server/internal/*`, `server/pkg/*`, `server/migration/` |
| `server/internal/handler`      | `server/internal/service`                             |
| `server/internal/service`      | `server/internal/repository`, `server/pkg/llm`        |
| `server/internal/repository`   | `server/pkg/models`, PostgreSQL                       |
| `server/internal/orchestrator` | `server/internal/service`, `server/internal/sandbox`  |
| `server/internal/workflow`     | `server/internal/orchestrator`, `server/internal/gitops` |
| `server/internal/gitops`       | `server/pkg/config` (for Git tokens), GitHub API      |
| `server/pkg/llm`               | `server/pkg/config` (for API keys)                    |
| `server/internal/sandbox`      | Docker SDK                                            |
| `server/internal/handler/analytics_dashboard` | `server/internal/service/analytics_dashboard`   |
| `server/internal/handler/audit`| `server/internal/service/audit`                       |
| `server/internal/handler/pr`   | `server/internal/service/task`, `server/internal/service/audit` |
| `server/internal/orchestrator/pr_generator` | `server/pkg/models`                          |
| `web/`                         | `server/cmd/api` (via REST/gRPC API)                  |

## 7. Development Phases

| Phase   | Scope                                                    | Status    |
| :------ | :------------------------------------------------------- | :-------- |
| Phase 0 | PoC CLI: Task → LLM → Code output                       | ✅ Done   |
| Phase 1 | API Server + DB + CRUD (Org/Project/Task/Agent/Rule/Skill) | ✅ Done   |
| Phase 2 | Auth + Git Integration + Web UI + Project System         | ✅ Done   |
| Phase 3a| Sandbox + Agent Manager + Orchestrator Core               | ✅ Done   |
| Phase 3b| Workflow Engine (DAG) + Prompt Assembly + Execution UI   | ✅ Done   |
| Phase 4 | AI Gateway (Tier Routing) + Skill System + Evals         | ⏳ In Progress |
| Phase 5 | Dashboard + Analytics + PR & Human Review                | ✅ Done   |
| Phase 6 | Remote Chatbots + Episodic Memory + Self-improving Agents| 📋 Plan   |

## 7.1 Migration Numbering Map

> **IMPORTANT:** All migration files must follow this sequential numbering to prevent `golang-migrate` conflicts.

| Number | Phase | Table(s) | File |
| :----- | :---- | :------- | :--- |
| `000001` | Phase 1 | organizations, projects, repositories, agents, tasks, rules, skills, memories | `000001_init.up.sql` |
| `000002` | Phase 2 | users, api_keys | `000002_users_auth.up.sql` |
| `000003` | Phase 2 | repositories (clone metadata) | `000003_repository_git_metadata.up.sql` |
| `000004` | Phase 2 | tasks (analysis, spec_status, parent_task_id) | `000004_task_analysis.up.sql` |
| `000005` | Phase 3a | secrets (AES-GCM encrypted) | `000005_secrets_and_agents.up.sql` |
| `000006` | Phase 4 | token_usage | `000006_token_usage.up.sql` |
| `000007` | Phase 5 | audit_logs | `000007_audit_logs.up.sql` |
| `000008` | Phase 6 | episodic_memory (enhance memories) | `000008_episodic_memory.up.sql` |
| `000009` | Phase 3b | workflow_artifacts | `000009_workflow_artifacts.up.sql` |
| `000010` | Phase 2 | git_accounts | `000010_git_accounts.up.sql` |
| `000011` | Phase 4 | provider_credentials, model_routes, credential_usage_logs | `000011_unified_ai_gateway.up.sql` |
| `000012` | Phase 3a | agents (role, autonomy, context) | `000012_role_based_agents.up.sql` |
| `000013` | Phase 3a | org_global_rules | `000013_org_global_rules.up.sql` |

## 8. Reference Projects

> See `resources/` directory and `docs/ROADMAP.md` for full details.

| Layer                | Reference Projects                           |
| :------------------- | :------------------------------------------- |
| Agent Runtime        | OpenHands / OpenClaw                         |
| Orchestration        | Multica / AutoGen / CrewAI                   |
| Workflow             | Temporal / LangGraph / n8n                   |
| AI Gateway           | LiteLLM / 9Router / Free Claude Code         |
| Task UX              | Plane / Linear                               |
| Git Integration      | Gitea / GitLab CE                            |
| AI Observability     | Langfuse / Helicone                          |
| Developer Portal     | Backstage                                    |
| Skills/Tools         | LangChain / Flowise                          |
| Agent Memory         | AgentMemory / Hermes Agent                   |
