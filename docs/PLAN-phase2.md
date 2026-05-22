# Phase 2 Implementation Plan — Git Integration + Web UI + Project System

> **For agentic workers:** Use subagent-driven-development or executing-plans
> to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Connect the platform to real Git repositories, build the Next.js Web UI for task/project management, and enhance the Project System so developers can link repos, browse tasks, and trigger AI workflows from the browser.

**Architecture:** Extends Phase 1's 3-layer backend with a new `gitops` package. Web UI consumes the existing REST API.

**Tech Stack (additions):** go-git, GitHub REST API v3, Next.js 16.x Active LTS (App Router + TypeScript), React 19.x, Node.js >=20.9, Tailwind CSS v4, Radix UI, shadcn/ui, Recharts, Lucide icons, SWR (data fetching), bcrypt, golang-jwt/jwt

---

## ⚠️ Pre-requisite: Human Review Gate

> **MANDATORY:** Before any Phase 2 implementation begins, the team must review and validate:
> 1. `docs/ARCHITECTURE.md` — Confirm the architecture is still accurate after Phase 1.
> 2. `docs/PLAN-phase2.md` (this file) — Walk through each task's scope and acceptance criteria.
> 3. `resources/OpenSpec/schemas/spec-driven/schema.yaml` — Understand the spec-driven approach for task I/O validation.
> 4. `resources/OpenSpec/openspec-parallel-merge-plan.md` — Review parallel execution and merge strategy.
> 5. `resources/Learning_Report.md` §5 (OpenSpec) — Read about Spec-driven Development and Parallel Execution & Merging.
>
> **Only proceed after the team signs off on this plan.**

---

## Task 0: Authentication & API Security

**Files:**
- Create: `server/migration/000002_users_auth.up.sql` — users + API keys tables
- Create: `server/migration/000002_users_auth.down.sql`
- Create: `server/internal/middleware/auth.go` — JWT validation middleware
- Create: `server/internal/service/auth.go` — login, register, token refresh
- Create: `server/internal/handler/auth.go` — auth endpoints

> **Why here?** Without auth, the Web UI is an open dashboard and the API is unprotected. Auth must exist before building UI pages.

**Scope:**
- [ ] **Step 1: Migration** — `users` table (id, email, password_hash, org_id, role, created_at) + `api_keys` table
- [ ] **Step 2: Auth service** — register, login (bcrypt), JWT issue/verify (access + refresh tokens)
- [ ] **Step 3: Auth middleware** — extract JWT from `Authorization: Bearer` header, inject user context
- [ ] **Step 4: Auth endpoints** — `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, `POST /api/v1/auth/refresh`
- [ ] **Step 5: Protect existing endpoints** — all CRUD routes require valid JWT
- [ ] **Step 6: RBAC** — admin vs member permissions on org/project resources
- [ ] **Step 7: Rate limiting** — per-user rate limit middleware (token bucket)
- [ ] **Step 8: Tests** — unit tests for auth service + integration tests for auth flow

---

## References

> Study these resources before starting implementation.

### UI Reference — `resources/ui-demo/`

The existing UI demo is the **primary design reference** for the Web UI. It provides:

| Component | Path | What to Reuse |
|-----------|------|---------------|
| **Design System** | `resources/ui-demo/app/globals.css` | OKLCH color tokens (dark theme), CSS variables, Geist font |
| **UI Primitives** | `resources/ui-demo/components/ui/` | 57 shadcn/ui components (Button, Card, Dialog, Tabs, Badge, etc.) |
| **Home Layout** | `resources/ui-demo/components/dashboard/home/` | `HomeSidebar`, `HomeHeader`, `ProjectsGallery` with create modal |
| **Project Detail** | `resources/ui-demo/components/dashboard/project/` | `ProjectSidebar`, `ProjectHeader`, `ProjectContent` (tabs: Overview, Tasks, Agents, Rules, Skills, Knowledge) |
| **Dashboard Pages** | `resources/ui-demo/components/dashboard/page-content.tsx` | `TasksPage`, `AgentsPage`, `RulesPage`, `SkillsPage`, `KnowledgePage`, `EnvironmentsPage`, `SettingsPage` |
| **Data Viz** | `resources/ui-demo/components/dashboard/` | `stats-cards.tsx`, `metrics-chart.tsx` (Recharts), `workflow-timeline.tsx`, `tasks-list.tsx`, `agents-grid.tsx` |
| **Project Wizard** | `resources/ui-demo/components/dashboard/` | `create-project-modal.tsx`, `project-setup-wizard.tsx` |

**Tech Stack from ui-demo:** Next.js 16, React 19, Tailwind CSS v4, Radix UI, shadcn/ui, Recharts, Lucide React, next-themes, cmdk, react-hook-form + zod

### Advanced UI Architecture Reference — `resources/multica/`

While `ui-demo` is our primary visual design, **Multica's Web UI (`resources/multica/apps/web`)** serves as our primary **architectural reference** for structuring a scalable Next.js application.

| Pattern | Path | What to Learn / Reuse |
|---------|------|-----------------------|
| **Modular Separation** | `resources/multica/packages/` | Multica separates routing (`apps/web`) from business logic (`packages/views`) and design system (`packages/ui`). We will adapt this internally as `web/src/app` (routes), `web/src/views` (page components), and `web/src/components/ui`. |
| **Dashboard Layout** | `resources/multica/packages/views/layout/` | Study `app-sidebar.tsx` and `dashboard-layout.tsx` for handling complex navigation and workspace contexts. |
| **Global Search** | `resources/multica/packages/views/search/` | `SearchCommand` (CMD+K) pattern for quick navigation between projects and agents. |
| **Floating Chat** | `resources/multica/packages/views/chat/` | `ChatFab` (Floating Action Button) and `ChatWindow` for interacting with agents from any page. |

### Learning Resources

| Resource | Path | Key Learnings for Phase 2 |
|----------|------|---------------------------|
| **Learning Report** | `resources/Learning_Report.md` | §1 Multica: Git Native integration, Go+Next.js self-hosted architecture. §3 AI-SDLC: Worktree Isolation pattern. §4 9Router: SSE streaming |
| **Reference Doc** | `resources/Reference_doc.md` | §2.3 Multica: Agent lifecycle management, Go+Next.js+pgvector arch. §3.1 Modular architecture. §3.8 HITL design points for review UI |

### External References

| Project | Relevance |
|---------|-----------|
| GitHub App Docs | Git API patterns for clone, branch, commit, PR |
| Plane | Task UX — kanban board, project management |
| Backstage | Developer portal layout and navigation patterns |
| Multica (`resources/multica/`) | Go backend + Next.js frontend self-hosted reference. See `server/` for API routing, `apps/` for Next.js UI patterns |

---

## Task 1: Git Operations Package & Webhooks (Migration: 000003)

**Files:**
- Create: `server/internal/gitops/gitops.go` — interface + types
- Create: `server/internal/gitops/github.go` — GitHub provider implementation
- Create: `server/internal/handler/webhook.go` — inbound webhook handlers
- Create: `server/migration/000003_repository_git_metadata.up.sql` — repository clone metadata
- Create: `server/migration/000003_repository_git_metadata.down.sql`

> **Study first:** `resources/Learning_Report.md` §1 (Multica Git Native), §3 (AI-SDLC Worktree Isolation)
> **Reference code:** `resources/multica/server/` for Go Git integration patterns

**Scope:**
- [ ] **Step 1: Define GitProvider interface**
  ```go
  type GitProvider interface {
      CloneRepo(ctx, repoURL, token, branch) (localPath, error)
      CreateBranch(ctx, localPath, branchName) error
      CommitAndPush(ctx, localPath, message, token) error
      CreatePR(ctx, owner, repo, title, head, base, body, token) (prURL, error)
      ListRepos(ctx, token) ([]RepoInfo, error)
  }
  ```

- [ ] **Step 2: Implement GitHub provider** using go-git for local ops and `net/http` for GitHub REST API (PR creation, repo listing)

- [ ] **Step 3: Add Webhook listeners** — `POST /api/v1/webhooks/github` to listen for "Issue Opened" or "CI Failed" events to auto-create Tasks.

- [ ] **Step 4: Add tests** — unit tests with mocks for GitHub API calls

---

## Task 2: Repository Service Enhancement

**Files:**
- Modify: `server/internal/service/repository.go`
- Modify: `server/internal/handler/repository.go`

**Scope:**
- [ ] **Step 1: Add ValidateToken endpoint** — `POST /api/v1/repositories/:id/validate` — tests the stored GitHub token by calling the GitHub API
- [ ] **Step 2: Add ListRemoteRepos endpoint** — `GET /api/v1/repositories/remote?token=xxx` — lists repos accessible via the provided token
- [ ] **Step 3: Add CloneRepo endpoint** — `POST /api/v1/repositories/:id/clone` — triggers async clone to local storage

---

## Task 2.5: Project Defaults Initialization (Rules & Skills Seeding)

**Files:**
- Modify: `server/internal/service/project.go`
- Create: `server/internal/service/seeder.go`

**Scope:**
- [ ] **Step 1: Auto-seed Rules** — Upon project creation, automatically parse `resources/prompt_base/GEMINI.md` and `resources/prompt_base/core/rules.md` to insert default `Rule` records into the database for the new project.
- [ ] **Step 2: Auto-seed Skills** — Parse `resources/superpowers/skills/` (for TDD workflows, planning, debugging) and `resources/prompt_base/antigravity/skills/` (for system workflows) to insert default `Skill` records for the new project.
- [ ] **Step 3: Update CreateProject endpoint** — Ensure the handler calls the seeder service synchronously or asynchronously after successful project creation.

---

## Task 3: Task Analysis & Sub-task Support (Migration: 000004)

**Files:**
- Create: `server/migration/000004_task_analysis.up.sql`
- Create: `server/migration/000004_task_analysis.down.sql`
- Modify: `server/pkg/models/task.go`
- Modify: `server/internal/repository/task.go`
- Modify: `server/internal/service/task.go`
- Modify: `server/internal/handler/task.go`

> **Study first:** `resources/Reference_doc.md` §2.1 AI-SDLC (Orchestrator pipeline decomposes tasks into sub-tasks)
> **Aligns with:** `docs/manual/Roadmap.md` §2 — Complexity-based branching (Easy/Medium/Hard)

**Scope:**
- [ ] **Step 1: Migration** — add columns to `tasks` table:
  - `parent_task_id UUID REFERENCES tasks(id)` — sub-task support
  - `analysis JSONB` — AI-generated analysis (scope, affected files, risks, execution plan)
  - `spec_status VARCHAR(20) DEFAULT 'NONE'` — tracks spec review state (NONE, DRAFT, PENDING_REVIEW, CHANGES_REQUESTED, APPROVED, AUTO_APPROVED)
- [ ] **Step 2: Update model** — add `ParentTaskID *uuid.UUID`, `SubTasks []Task`, `Analysis json.RawMessage`, `SpecStatus string`
- [ ] **Step 3: Task classification service** — implement the two-track workflow:
  - **Analyze endpoint** `POST /api/v1/tasks/:id/analyze` — AI agent analyzes task, classifies complexity (Easy/Medium/Hard), generates structured analysis (JSON), and sets `spec_status`:
    - Easy → `spec_status = AUTO_APPROVED`, task proceeds directly to execution.
    - Medium/Hard → `spec_status = PENDING_REVIEW`, task waits for human review.
  - **Clarification loop** — if AI detects missing information during analysis, it generates clarification questions returned in the response. Developer answers via `POST /api/v1/tasks/:id/clarify` with additional context. Agent re-analyzes after each clarification until satisfied.
- [ ] **Step 4: Spec review endpoints** (for Medium/Hard tasks):
  - `GET /api/v1/tasks/:id/analysis` — view the AI-generated analysis
  - `POST /api/v1/tasks/:id/analysis/approve` — human approves spec → `spec_status = APPROVED`
  - `POST /api/v1/tasks/:id/analysis/request-changes` — human requests changes with feedback → `spec_status = CHANGES_REQUESTED`
  - `PATCH /api/v1/tasks/:id/analysis` — human updates the analysis directly
- [ ] **Step 5: Sub-task support** — `ListSubTasks(ctx, parentID)`, `GET /api/v1/tasks/:id/subtasks`, `POST /api/v1/tasks/:id/subtasks`
- [ ] **Step 6: Tests** — unit tests for classification logic, spec review state machine, sub-task CRUD

---

## Task 4: Initialize Next.js Web UI

**Files:**
- Create: `web/` directory (Next.js project)

> **Primary reference:** Use `docs/design-system/autocodeos/MASTER.md` for styling (Colors, Typography) and `resources/ui-demo/` for structural React components.

**Scope:**
- [ ] **Step 1: Bootstrap project**
  ```bash
  npx -y create-next-app@latest ./web --typescript --tailwind --eslint --app --src-dir --use-npm
  ```
  - Use the latest stable Next.js 16.x release generated by `create-next-app@latest`.
  - Require Node.js `>=20.9` for local development and Docker images.

- [ ] **Step 2: Initialize Design System**
  - Translate the hex colors from `docs/design-system/autocodeos/MASTER.md` into OKLCH CSS variables in `web/src/app/globals.css`.
  - Configure Google Fonts for **Fira Code** (headings) and **Fira Sans** (body).
  - Copy structural components from `resources/ui-demo/components/ui/` → `web/src/components/ui/` (but adapt them to use the new CSS variables).
  - Install dependencies: Radix UI, lucide-react, recharts, cmdk, sonner, next-themes, class-variance-authority, clsx, tailwind-merge, react-hook-form, zod.

- [ ] **Step 3: Set up API client & Real-time Context** 
  - `web/src/lib/api.ts` with base fetch wrapper pointing to `http://localhost:8080/api/v1`
  - Create a React Context or Zustand store (`web/src/lib/store/realtime.ts`) to manage Server-Sent Events (SSE) connections for future live pipeline tracking.

- [ ] **Step 4: Set up shared types** — `web/src/lib/types.ts` mirroring Go models (Organization, Project, Task, Agent, etc.)

- [ ] **Step 5: Configure CORS** — update `server/internal/middleware/cors.go` to allow `http://localhost:3000`

---

## Task 5: Web UI — Layout & Navigation

**Files:**
- Create: `web/src/app/layout.tsx` — root layout with sidebar
- Create: `web/src/components/dashboard/home/home-sidebar.tsx`
- Create: `web/src/components/dashboard/home/home-header.tsx`
- Create: `web/src/components/theme-provider.tsx`

> **Copy & adapt from:** `resources/ui-demo/components/dashboard/home/home-sidebar.tsx`, `home-header.tsx`
> **Design reference:** `docs/design-system/autocodeos/MASTER.md` for visual tokens (Fira fonts, #0F172A background, #22C55E accent).

**Scope:**
- [ ] **Step 1: Apply Design System** — Enforce dark theme by default, Fira Sans font, and the specific CTA/Accent colors.
- [ ] **Step 2: Home Sidebar** — navigation: Projects, Skills, Organization, Settings (matching ui-demo's `HomeSidebar`)
- [ ] **Step 3: Home Header** — search bar, notifications, user avatar (matching ui-demo's `HomeHeader`)
- [ ] **Step 4: Responsive layout** — collapsible sidebar on mobile
- [ ] **Step 5: Wire to API** — replace ui-demo's hardcoded data with SWR hooks fetching from REST API

---

## Task 6: Web UI — Projects Page

**Files:**
- Create: `web/src/app/page.tsx` — home page with projects gallery
- Create: `web/src/app/projects/[id]/page.tsx` — project detail with sidebar tabs
- Create: `web/src/components/dashboard/home/projects-gallery.tsx`
- Create: `web/src/components/dashboard/create-project-modal.tsx`
- Create: `web/src/components/dashboard/project-setup-wizard.tsx`

> **Copy & adapt from:** `resources/ui-demo/components/dashboard/home/projects-gallery.tsx` (project cards with progress bars, status badges, task/agent counts)
> **Copy & adapt from:** `resources/ui-demo/components/dashboard/create-project-modal.tsx`, `project-setup-wizard.tsx`

**Scope:**
- [ ] **Step 1: Projects gallery** — grid of project cards (name, description, status badge, progress bar, task count, agent count, Open/Delete buttons) — adapt from ui-demo's `ProjectsGallery`
- [ ] **Step 2: Create project modal** — multi-step form (name, description, language, framework) — adapt from ui-demo's `CreateProjectModal`
- [ ] **Step 3: Project setup wizard** — post-create wizard for linking repo, adding agents — adapt from ui-demo's `ProjectSetupWizard`
- [ ] **Step 4: Project detail page** — sidebar with tabs (Overview, Tasks, Agents, Rules, Skills, Knowledge) — adapt from ui-demo's `ProjectSidebar` + `ProjectContent`
- [ ] **Step 5: Wire to API** — CRUD operations via `web/src/lib/api.ts`

---

## Task 7: Web UI — Tasks Page

**Files:**
- Create: `web/src/components/dashboard/tasks-list.tsx`
- Create: `web/src/components/dashboard/page-content.tsx` (TasksPage export)

> **Copy & adapt from:** `resources/ui-demo/components/dashboard/tasks-list.tsx` (task list with status, priority, complexity, agent assignment)
> **Study:** `resources/Reference_doc.md` §3.8 — HITL design: task approval/rejection UI points

**Scope:**
- [ ] **Step 1: Tasks list** — table/list view with status badges, complexity (Easy/Medium/Hard color-coded), priority, assigned agent, **spec_status badge** — adapt from ui-demo's `TasksList`
- [ ] **Step 2: Task detail page** — full description, AI analysis panel, sub-tasks list, status transitions, agent assignment
- [ ] **Step 3: Spec review UI** (for Medium/Hard tasks):
  - Display AI-generated analysis (scope, affected files, risks, execution plan)
  - **Approve / Request Changes** buttons — triggers `spec_status` transition
  - Inline editing — reviewer can update analysis directly
  - Clarification Q&A thread — show agent's questions and developer's answers
- [ ] **Step 4: Kanban board** — columns for each task status (TODO → ANALYZING → PENDING_REVIEW → CODING → REVIEWING → FIXING → TESTING → HUMAN_REVIEW → MERGED)
- [ ] **Step 5: Create/edit task form** — title, description, complexity (optional — AI can auto-classify), priority, labels, repository selection

---

## Task 8: Web UI — Agents Page

**Files:**
- Create: `web/src/components/dashboard/agents-grid.tsx`

> **Copy & adapt from:** `resources/ui-demo/components/dashboard/agents-grid.tsx` (agent cards with role, status, model, activity)
> **Study:** `resources/Learning_Report.md` §8 — Hermes Agent self-improving loop (influences agent status display)

**Scope:**
- [ ] **Step 1: Agents grid** — cards showing name, role, provider, model, status (idle/busy), level badge — adapt from ui-demo's `AgentsGrid`
- [ ] **Step 2: Agent detail** — assigned tasks, performance stats placeholder, configuration
- [ ] **Step 3: Create/edit agent form** — name, role (Planner/Backend/Frontend/Reviewer/QA), provider, model, level

---

## Task 9: Web UI — Dashboard & Additional Pages

**Files:**
- Create: `web/src/components/dashboard/stats-cards.tsx`
- Create: `web/src/components/dashboard/metrics-chart.tsx`
- Create: `web/src/components/dashboard/workflow-timeline.tsx`

> **Copy & adapt from:** `resources/ui-demo/components/dashboard/stats-cards.tsx` (stat cards), `metrics-chart.tsx` (Recharts), `workflow-timeline.tsx`
> **Copy & adapt from:** `resources/ui-demo/components/dashboard/page-content.tsx` — `RulesPage`, `SkillsPage`, `KnowledgePage`, `EnvironmentsPage`, `SettingsPage`

**Scope:**
- [ ] **Step 1: Overview stats** — stat cards (total projects, active tasks, running agents, open PRs) — from ui-demo's `StatsCards`
- [ ] **Step 2: Metrics chart** — Recharts visualization (task completion rate, agent activity) — from ui-demo's `MetricsChart`
- [ ] **Step 3: Workflow timeline** — visual timeline of recent workflow executions — from ui-demo's `WorkflowTimeline`
- [ ] **Step 4: Rules page** — toggle-based rule management — from ui-demo's `RulesPage`
- [ ] **Step 5: Skills page** — skill proficiency cards — from ui-demo's `SkillsPage`
- [ ] **Step 6: Knowledge page** — document browser — from ui-demo's `KnowledgePage`
- [ ] **Step 7: Settings page** — settings categories — from ui-demo's `SettingsPage`

---

## Task 10: Docker Compose & Makefile Updates

**Files:**
- Modify: `docker-compose.yml` — add web service
- Modify: `Makefile` — add `web`, `dev` targets

**Scope:**
- [ ] **Step 1: Docker compose** — add Next.js service with hot reload
- [ ] **Step 2: Makefile targets**
  ```makefile
  web:
  	cd web && npm run dev
  dev:
  	make db-up && make api & make web
  ```
- [ ] **Step 3: Update .env.example** — add `NEXT_PUBLIC_API_URL`, `GITHUB_TOKEN`

---

## Execution Order

```
Task 0             (Auth — MUST be first, all other tasks depend on it)
Task 1 → 2 → 2.5 → 3 (Backend: Git + Repo + Init + Sub-tasks)
Task 4 → 5         (Web: Bootstrap + Layout + UI primitives from ui-demo)
Task 6 → 7 → 8 → 9 (Web: Pages — adapt from ui-demo components)
Task 10             (DevOps: Docker + Makefile)
```

**Parallel tracks possible:**
- Backend (Tasks 1-3) and Web bootstrap (Task 4) can run in parallel AFTER Task 0
- Web pages (Tasks 6-9) can be built in parallel once Task 5 is done

## Testing Requirements

| Layer | Tool | Minimum Coverage |
|-------|------|------------------|
| **Go unit tests** | `go test` | All services + repositories |
| **Go integration tests** | `go test` + test DB | API → DB round-trip for auth, git ops, sub-tasks |
| **Web E2E tests** | Playwright | Login flow, project CRUD, task CRUD, navigation |
| **API contract tests** | Manual / httptest | All new endpoints return correct status codes + JSON shapes |

## Verification

```bash
# Auth
curl -X POST localhost:8080/api/v1/auth/register -d '{"email":"test@example.com","password":"secret"}'
curl -X POST localhost:8080/api/v1/auth/login -d '{"email":"test@example.com","password":"secret"}'
# Use returned JWT for all subsequent requests

# Backend
make db-up && make api
curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/health
curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/repositories/remote?token=$GITHUB_TOKEN

# Frontend
cd web && npm run dev
# Open http://localhost:3000 — verify login, dashboard, projects, tasks pages
# Compare against resources/ui-demo for design consistency

# Tests
cd server && GOCACHE=/tmp/autocodeos-go-build go test ./...
cd web && npx playwright test
```
