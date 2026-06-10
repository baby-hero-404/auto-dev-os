# PLAN: UI Main Flow Enhancement

> Reference: `docs/references/Learning_Report.md` — Multica, AI-SDLC, 9router, Prompt Base.
>
> Current repo check: this plan is written against the existing Next.js app in `web/src/app/*`, API client in `web/src/lib/api.ts`, and Go routes in `server/internal/handler/router.go`.

## Design Direction: Global -> Local

The UI should make one platform concept obvious:

```text
Organization / Global
├── AI provider defaults
├── Agent pool
│   ├── auto_join agents: inherited by every project
│   └── manual agents: assigned to selected projects
└── Future global integrations
    └── Git accounts, provider model discovery, default rules/skills

Project / Local
├── Repositories
├── Tasks
├── Inherited agents from global pool
├── Manual project-specific assignments
└── Project rules and agent skill assignments
```

Rationale:

- Multica-style squads map well to the existing `agents` and `project_agents` model.
- Prompt Base-style seeding maps to `auto_join` agents and global skills/rules.
- 9router-style provider tiers map to the existing `gateway`, `fast`, `balanced`, and `powerful` model choices.

## Current Backend Reality

Already available:

| Capability | Existing route/API |
| --- | --- |
| List projects | `GET /organizations/:orgID/projects` |
| List project repos | `GET /projects/:projectID/repositories` |
| List project tasks | `GET /projects/:projectID/tasks` |
| List org agents | `GET /organizations/:orgID/agents` |
| Hire org agent | `POST /organizations/:orgID/agents` |
| List effective project agents | `GET /projects/:projectID/agents` |
| Assign org agent to project | `POST /projects/:projectID/agents` with `agent_id` |
| Delete org agent | `DELETE /agents/:agentID` |
| Project rules | `GET/POST /projects/:projectID/rules` |
| Global skills and agent skills | `GET /skills`, `GET/POST /agents/:agentID/skills` |

Important constraints:

- `api.listAgents(projectID)` returns both `auto_join` agents and manually assigned agents. Split by `assignment_strategy`.
- There is no endpoint to remove only a manual project assignment. `DELETE /agents/:agentID` deletes the org-level agent.
- There is no Git Accounts API yet.
- There is no provider model discovery/proxy API yet.
- `Agent.project_id` in the frontend type is stale for org-level agents; the database now uses `org_id` plus `project_agents`.

## Scope

Implement a UI-first MVP using existing APIs, and avoid blocking on new backend work.

| Page | Route | Scope |
| --- | --- | --- |
| Settings | `/settings` | Rewrite into Global Config Hub with tabs |
| Projects list | `/` | Enhance project cards with real derived stats |
| Project detail | `/projects/[id]` | Move Members into a dedicated tab and show Global vs Project-specific |
| Agents page | `/agents` | Keep route working; optionally redirect or reuse Settings Members content |

Out of scope for this pass:

- Persisting Git provider accounts.
- Fetching live provider model lists.
- Removing a single manual project assignment.
- Global rules UI unless backend support is confirmed or added.

## Task 1: Settings Page (`/settings`)

### Goal

Turn Settings into the organization-level control center. The page should feel functional now, even where backend persistence is future work.

### Tabs

```text
Settings
├── AI Providers
├── Members
└── Git Accounts
```

Use tab order above because Members and provider defaults work today; Git Accounts is the least-backed section.

### Tab: AI Providers

Use existing frontend constants from `/agents/page.tsx` as the initial source of truth:

- Providers: `gateway`, `openai`, `anthropic`, `gemini`, `9router`
- Gateway tiers: `fast`, `balanced`, `powerful`
- Current model options:
  - `openai`: `gpt-4o-mini`, `gpt-4o`
  - `anthropic`: `claude-sonnet-4-20250514`, `claude-opus-4-20250514`
  - `gemini`: `gemini-2.5-flash`, `gemini-2.5-pro`
  - `9router`: `balanced`, `fast`, `powerful`, `premium-coding`

UI requirements:

- Provider cards with enabled/disabled local UI state.
- API key field as masked input, clearly marked as local-only until backend persistence exists.
- Base URL override field, local-only.
- Model chips grouped by tier: `fast`, `balanced`, `powerful`, `premium`.
- Default model/tier selector used by the Hire Agent form on the same page.
- "Fetch Models" button is disabled with a tooltip or inline note until backend proxy exists.

Implementation notes:

- Do not add real secrets to `localStorage`.
- If temporary UI state is needed, keep it in React state only.
- Extract provider/model constants into a shared module, for example `web/src/lib/model-options.ts`, so Settings and Agents do not drift.

Acceptance criteria:

- User can inspect available provider/model choices.
- User can choose defaults during the current session.
- No fake claim that provider keys are saved or tested.

### Tab: Members

Move or reuse the useful parts of `/agents` here. This is the highest-value Settings tab because org-scoped agents already work.

#### Quick-hire Wizard (Option D — selected)

Replace the flat Hire Agent form with a **2-step modal wizard**. The goal is to reduce the number of decisions to two: "what is this agent for?" and "how capable should it be?".

```text
┌───────────────────────────────────────────────────┐
│ Hire New Agent                               [✕]  │
├───────────────────────────────────────────────────┤
│ Step 1: Identity                                  │
│   Name  [_____________________]                   │
│   Role  [Backend ▼]                               │
│   Strategy  ○ Auto-join all  ● Manual             │
├───────────────────────────────────────────────────┤
│ Step 2: Capability                                │
│                                                   │
│  ○ ⚡ Fastest & Cheapest                          │
│     gateway / fast  — best for routine tasks      │
│                                                   │
│  ● ⚖️  Smart & Balanced          ← default        │
│     gateway / balanced                            │
│                                                   │
│  ○ 💎 Most Capable                               │
│     gateway / powerful — best for review & hard   │
│                                                   │
│  ○ 🛠  Custom                                     │
│     Provider [openai ▼]  Model [gpt-4o ▼]        │
│                                                   │
├───────────────────────────────────────────────────┤
│                          [Cancel]  [Create Agent] │
└───────────────────────────────────────────────────┘
```

Tier to model mapping (defined in `web/src/lib/model-options.ts`):

| Tier | Provider | Model | Typical use |
| --- | --- | --- | --- |
| ⚡ Fastest & Cheapest | gateway | fast | Planner, QA, routine |
| ⚖️ Smart & Balanced | gateway | balanced | Backend, Frontend |
| 💎 Most Capable | gateway | powerful | Reviewer, hard tasks |
| 🛠 Custom | any | any | Power users, special models |

Custom escape hatch exposes:

- Provider dropdown: `gateway`, `openai`, `anthropic`, `gemini`, `9router`
- Model dropdown filtered by selected provider, sourced from `model-options.ts`
- "Auto by level" option preserved for gateway provider

Role-to-tier suggestions shown as non-enforced hint text when role changes:

| Role | Suggested tier |
| --- | --- |
| `planner` | ⚡ Fastest |
| `backend` | ⚖️ Balanced |
| `frontend` | ⚖️ Balanced |
| `reviewer` | 💎 Most Capable |
| `qa` | ⚖️ Balanced |

#### Rest of Members tab UI

- Stats row: Total agents | Auto-join | Manual | Provider distribution
- "Seed Default Fleet" button:
  - Disabled if org already has agents, with tooltip "Fleet already seeded".
  - Creates: Planner (auto-join, fast), Backend (manual, balanced), Frontend (manual, balanced), Reviewer (manual, powerful), QA (manual, balanced).
- Agent cards:
  - Name, role, level badge, provider/model
  - `auto_join` → green pulse badge "Inherited by all projects"
  - `manual` → project assignment dropdown for unassigned projects
  - Assigned project tags
  - Delete button (hover reveal, org-level only, explicit confirm wording)

Implementation notes:

- Extract tier → provider → model mapping into `web/src/lib/model-options.ts`. This is the single source of truth for both the wizard and the AI Providers tab.
- Wizard maps tier selection to `{ provider, model, level }` before calling `api.hireAgent`.
- Reuse `api.listOrgAgents`, `api.hireAgent`, `api.createAgent`, and `api.deleteAgent`.
- Assignment lookup currently requires fetching `api.listAgents(projectID)` for each project. Use SWR and parallel requests.
- Avoid showing a "Remove from project" action until backend supports deleting `project_agents` rows.

Acceptance criteria:

- Wizard opens as a modal on "Hire Agent" click.
- Tier selection maps correctly to provider/model in the API call.
- Custom tier shows full provider+model dropdowns.
- Role hint text updates when role changes.
- Seed Default Fleet creates the expected fleet and does not create duplicates on repeat.
- Hiring an org agent closes the wizard and updates the agent grid.
- Manual agents can be assigned to projects.
- Auto-join agents are visually distinct from manual agents.

### Tab: Git Accounts

This should be framed as a planned integration unless backend is added in the same PR.

UI requirements:

- Empty state / planned state explaining Git account connections are not persisted yet.
- Static provider cards for GitHub, GitLab, Bitbucket.
- Disabled "Connect" and "Test Connection" actions.
- Link users to per-project repository linking as the current working path.

Backend needed later:

- `GET /organizations/:orgID/git-accounts`
- `POST /organizations/:orgID/git-accounts`
- `PATCH /git-accounts/:id`
- `DELETE /git-accounts/:id`
- `POST /git-accounts/:id/test`

Acceptance criteria for this pass:

- Page does not pretend Git accounts are functional.
- UI clearly distinguishes current per-project repo linking from future org-level Git accounts.

## Task 2: Projects List (`/`)

### Goal

Replace hardcoded "active" and fake progress with derived project health.

### New card content

```text
Project name                  status
Description, clamped to 2 lines

Repos: 2    Agents: 3    Tasks: 4/10 done
Progress bar: 40%            Last activity: 2h ago
```

Data sources:

- Repos count: `api.listRepositories(project.id)`
- Agent count: `api.listAgents(project.id)`; includes auto-join plus manual assignments
- Task progress: `api.listTasks(project.id)`
- Last activity: newest `task.updated_at`, fallback to `project.updated_at`

Suggested status derivation:

| Status | Rule |
| --- | --- |
| `blocked` | At least one task status is `failed`, `blocked`, or `needs_changes` |
| `active` | At least one task status is `running`, `in_progress`, `approved`, `queued`, or `analyzing` |
| `done` | Total tasks > 0 and all tasks are `done`, `completed`, or `merged` |
| `idle` | No tasks, or no active/blocked tasks |

Progress derivation:

- `done = count(status in ["done", "completed", "merged"])`
- `total = tasks.length`
- `progress = total === 0 ? 0 : Math.round((done / total) * 100)`

Implementation notes:

- Add a focused `ProjectCard` component if it keeps `page.tsx` readable.
- Use one SWR key per project metadata query, or a single `project-card-meta` fetcher that runs `Promise.all`.
- Keep per-card failure isolated. If one metadata request fails, render `--` for that count and keep the project card usable.
- Avoid introducing a new server aggregate endpoint for the MVP.

Acceptance criteria:

- No hardcoded active badge.
- Progress bar matches real task statuses.
- Repo, agent, and task counts match backend data.
- Projects still render if one metadata endpoint fails.

## Task 3: Project Detail (`/projects/[id]`)

### Goal

Make project membership understandable by separating inherited global agents from manual assignments.

### Layout change

Current:

```text
Left sidebar: Repositories | Project Members | Create Task
Right panel: Tasks | Settings & Skills
```

Target:

```text
Left sidebar: Repositories | Create Task
Right panel: Tasks | Members | Settings & Rules
```

### Members tab

Split `projectAgents` from `api.listAgents(projectID)`:

- Inherited from Global:
  - `agent.assignment_strategy === "auto_join"`
  - readonly
  - badge: `Global`
  - copy: "Inherited by every project"
- Project-specific:
  - `agent.assignment_strategy !== "auto_join"`
  - assigned through `project_agents`
  - badge: `Manual`
  - no remove action until backend supports unassign

Assign form:

- Source: `api.listOrgAgents(orgID)`
- Filter:
  - exclude `auto_join`
  - exclude agents already in `projectAgents`
- Submit via `api.createAgent(projectID, token, { agent_id: staff.id, ...staff fields })`

Each agent card:

- Name
- Role
- Level
- Provider/model
- Status
- Skills count if already loaded from `api.listAgentSkills`

Acceptance criteria:

- Auto-join agents appear under inherited/global.
- Manual agents appear under project-specific.
- Assigning a manual agent updates the Members tab.
- The UI does not offer unsupported remove behavior.

### Settings & Rules tab

Keep current capabilities, but rename and organize:

- Project metadata form
- Project rules list and add-rule form
- Agent skill assignment section

Implementation notes:

- The current "Settings & Skills" tab has useful code; preserve it and retitle to "Settings & Rules" or split sections more clearly.
- `agentSkills` fetch should continue to key off project agents.
- Keep `DashboardLayout` consistency if feasible; project detail currently uses its own `<main>`.

Acceptance criteria:

- Existing project update, rule creation, and skill assignment behavior still works.
- Members are no longer duplicated in the left sidebar and a tab.

## Shared Components And Cleanup

Add shared components only when they reduce repeated code:

| Component | Purpose |
| --- | --- |
| `TabGroup` | Reusable tab header and panel switcher |
| `AgentCard` | Shared member/agent display for Settings and Project Detail |
| `ProjectCard` | Keeps Projects page metadata logic readable |
| `ProviderModelSelector` | Avoids duplicated provider/model constants |

Avoid premature component extraction if a component would only be used once.

## Implementation Order

1. Extract model/provider constants into `web/src/lib/model-options.ts`.
2. Rewrite `/settings` with AI Providers and Members working against existing APIs.
3. Move Project Members from sidebar into a new Project Detail Members tab.
4. Enhance Projects list cards with real repo/agent/task metadata.
5. Optionally make `/agents` redirect to `/settings?tab=members` or keep it as a legacy alias using the same member-management component.

## Test Plan

Manual:

- Login and open `/settings`.
- Hire a manual agent.
- Hire an `auto_join` agent.
- Assign a manual agent to a project.
- Open that project and confirm the two member sections are correct.
- Create a task and confirm Projects list progress updates.
- Link or inspect repositories and confirm repo count updates.

Automated / lint:

- `npm run lint` in `web`
- Add or update Playwright coverage only if existing auth helpers make it practical.
- No Go tests are required for UI-only MVP unless backend routes are added.

## Backend Follow-up Plan

Add these only after the UI MVP lands or if product needs persistence immediately:

1. Git Accounts:
   - table: `git_accounts`
   - encrypted token storage
   - org-scoped CRUD and test endpoint
2. Provider settings:
   - table: `provider_settings`
   - encrypted API keys
   - provider model discovery proxy
3. Project agent unassign:
   - `DELETE /projects/:projectID/agents/:agentID`
   - deletes only from `project_agents`, not `agents`
4. Global rules:
   - org-scoped global rules
   - merged display in project Settings & Rules

## Open Decisions

1. Should `/agents` remain a full page, or redirect to `/settings?tab=members` after Settings ships?
2. Should Git Accounts be shown as a disabled future tab in the MVP, or hidden until backend support exists?
3. Do we want to add backend unassign now, or keep removal out of the UI for this pass?

## References

- Multica (§1): agent assignment into Squad/Project.
- AI-SDLC (§3): governance and project workflow states.
- 9router (§4): provider fallback tiers and model routing.
- Prompt Base (§10): global rules/skills seeded into projects.
- Antigravity Awesome Skills (§11): skill registry and progressive disclosure.
