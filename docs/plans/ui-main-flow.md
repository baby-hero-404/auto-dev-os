# PLAN: UI Onboarding & Main Flow Enhancement

> **Goal**: Make it easy and obvious for a new user to set up everything they need before running their first AI task.
>
> **Reference**: `docs/ROADMAP.md`, existing Next.js app `web/src/app/*`, API client `web/src/lib/api/`.

---

## Mental Model: The Setup Journey

A new user needs 7 things before they can run their first task:

```
Step 1 → Add an AI Provider Key        (so agents can call LLMs)
Step 2 → Connect a GitHub Account       (so agents can clone & push)
Step 3 → Create a Project               (the container for everything)
Step 4 → Add at least one Agent         (the worker that runs tasks)
Step 5 → Assign Skills to Agents        (what tools the agent can use)
Step 6 → Add Rules to the Project       (what the agent must/must not do)
Step 7 → Create a Task                  (the unit of work to execute)
```

> **Steps 5 and 6 are optional but strongly recommended.** Without skills, agents run with no tool constraints. Without rules, agents have no behavioral guardrails.

The UI must make this sequence **obvious** on first load and **fast** for repeat users.

---

## Architecture Reference

```
Organization (global)
├── AI Provider Credentials  ← Phase 1
├── Git Accounts             ← Phase 2
└── Agent Pool
    ├── auto_join agents     ← inherited by every project (Phase 4)
    ├── manual agents        ← assigned to selected projects (Phase 4)
    └── Agent Skills         ← capabilities assigned per agent (Phase 5)

Project (local)
├── Repositories             ← Phase 3 (link repo when creating project)
├── Project Rules            ← Phase 6 (strict/advisory behavioral guardrails)
├── Tasks                    ← Phase 7
├── Inherited Agents (auto_join)
└── Manually assigned Agents
```

---

## Current Backend Reality

| Capability | Route | Status |
|---|---|---|
| List / Create projects | `GET/POST /organizations/:orgID/projects` | ✅ Working |
| List / Create repositories | `GET/POST /projects/:projectID/repositories` | ✅ Working |
| List / Create tasks | `GET/POST /projects/:projectID/tasks` | ✅ Working |
| List org agents | `GET /organizations/:orgID/agents` | ✅ Working |
| Create org agent | `POST /organizations/:orgID/agents` | ✅ Working |
| List project agents (effective) | `GET /projects/:projectID/agents` | ✅ Working |
| Assign agent to project | `POST /projects/:projectID/agents` with `agent_id` | ✅ Working |
| Delete org agent | `DELETE /agents/:agentID` | ✅ Working |
| **List all global skills** | **`GET /skills`** | ✅ Working |
| **List skills assigned to agent** | **`GET /agents/:agentID/skills`** | ✅ Working |
| **Assign skill to agent** | **`POST /agents/:agentID/skills`** with `skill_id` | ✅ Working |
| **Replace agent skills (bulk)** | **`PUT /agents/:agentID/skills`** | ✅ Working |
| **List project rules** | **`GET /projects/:projectID/rules`** | ✅ Working |
| **Create project rule** | **`POST /projects/:projectID/rules`** | ✅ Working |
| List / Create provider credentials | `GET/POST /organizations/:orgID/provider-credentials` | ✅ Working |
| Test provider credential | `POST /provider-credentials/:id/test` | ✅ Working |
| List / Create Git accounts | `GET/POST /organizations/:orgID/git-accounts` | ✅ Working |
| Delete Git account | `DELETE /git-accounts/:id` | ✅ Working |
| Test Git account | `POST /git-accounts/:id/test` | ✅ Working |

---

## Phase Overview

| Phase | Page / Component | Scope | Blocks |
|---|---|---|---|
| **Phase 1** | `/ai-providers` | AI provider credential management | Nothing — do first |
| **Phase 2** | `/settings` → Git Accounts tab | GitHub / GitLab account connection | Phase 1 done |
| **Phase 3** | `/` Projects list + Create modal | Project creation with optional repo link | Phase 2 done |
| **Phase 4** | `/settings` → Members tab | Hire agents, seed default fleet | Phase 1 done |
| **Phase 5** | `/settings` → Members tab / Agent card | Assign Skills to agents | Phase 4 done |
| **Phase 6** | `/projects/[id]` → Settings tab | Add Rules to project (strict/advisory) | Phase 3 done |
| **Phase 7** | `/projects/[id]` | Task creation & project detail tabs | Phase 3 + 4 + 5 + 6 done |
| **Phase 8** | Setup Checklist Banner | Global onboarding status indicator | All phases done |

---

## Phase 1: AI Provider Credentials (`/ai-providers`)

### Goal
Allow users to add, test, and delete API keys for LLM providers so agents can call models.

### What Already Works
The page at `/ai-providers` exists and has full CRUD. **Phase 1 is mostly polish.**

### Changes

#### 1.1 — Layout & Visual Polish
- Add `glow-on-hover` to each provider card.
- Show a pulsing `active` dot in the status badge when credentials are present.
- Animate the `CheckCircle` icon on status badge with `animate-pulse` when `count > 0`.

#### 1.2 — Form UX
- **API Key show/hide toggle**: Add `Eye` / `EyeOff` button inside the API key field. ✅ (already done)
- **Context-aware Base URL placeholder**: Change placeholder based on selected provider. ✅ (already done)
- **Priority hint text**: Add `"Lower = runs first (0 = highest priority)"` label next to Priority.
- **After save**: Flash a `✓ Saved` success state on the Save button for 2 seconds before reset.

#### 1.3 — Empty State Guidance
When no credentials exist for a provider, replace the dashed border with a message:
```
"No keys yet. Add one to enable this provider for your agents."
```

### Acceptance Criteria
- [ ] User can add a credential and immediately test it without page reload.
- [ ] Status badge updates after test (success → green, failure → red, then resets after 3s).
- [ ] Show/hide toggle works on the API key field.
- [ ] Provider cards with credentials are visually distinct from empty ones.

### Files
- `web/src/app/ai-providers/page.tsx` — primary file ✅ mostly done
- No new files needed.

---

## Phase 2: Git Accounts (`/settings` → Git Accounts Tab)

### Goal
Allow users to connect a GitHub / GitLab account at the organization level so agents can clone and push repos.

### Context
The backend already has `/organizations/:orgID/git-accounts` with CRUD + test. The UI must expose this clearly.

### What to Build

#### 2.1 — Settings Page Tabs
Move `/settings` to a tab-based layout:
```
Settings
├── Git Accounts   ← Phase 2
└── Members        ← Phase 4
```
Use Radix UI `Tabs` (already in `package.json`).

#### 2.2 — Git Accounts Tab UI

**Empty state** (no accounts):
```
┌─────────────────────────────────────────────┐
│  🔗  No Git accounts connected               │
│                                             │
│  Connect GitHub or GitLab to let agents     │
│  clone repositories and open pull requests. │
│                                             │
│  [+ Connect GitHub]  [+ Connect GitLab]     │
└─────────────────────────────────────────────┘
```

**Add Account Inline Form** (slide-down, not modal):
```
Provider      [ GitHub ▼ ]
Display Name  [ My GitHub account ]
Token         [ ghp_xxxx          ] [👁]
Base URL      [ optional, for GitHub Enterprise ]
              [ Cancel ]  [ Connect & Test ]
```

**Connected Account Card**:
```
┌─────────────────────────────────────┐
│  GitHub icon   My GitHub Account    │
│  github.com  ·  Connected 2h ago   │
│                         [Test] [✕] │
└─────────────────────────────────────┘
```

**Test button states**: `Testing…` → `✓ OK` (green, 3s) or `✗ Failed` (red, 3s) → resets.

#### 2.3 — Per-Project Repository Linking (appears in Phase 3)
When creating a repository inside a project, show a dropdown of connected Git accounts so the user can select which account to use for clone/push.

### Acceptance Criteria
- [ ] User can connect a GitHub account with a token.
- [ ] Test connection shows live feedback.
- [ ] Connected accounts are listed with display name and provider.
- [ ] User can delete an account (with confirm prompt).
- [ ] Form token field has show/hide toggle.

### Files
- `web/src/app/settings/page.tsx` — rewrite with tab layout
- No new backend files (routes already exist).

---

## Phase 3: Project Creation Flow (`/`)

### Goal
Make creating a project fast and clear. Optionally link a repository immediately so the user does not need to go back to add one.

### Changes

#### 3.1 — "New Project" Modal Enhancement
Current modal only takes Name + Description. Extend to a **2-step modal**:

**Step 1 — Project Info**:
```
┌──────────────────────────────────────────┐
│  Create New Project                  [✕] │
├──────────────────────────────────────────┤
│  Name *        [ e.g. api-backend      ] │
│  Description   [ Optional goal/scope   ] │
│                                          │
│                      [Cancel]  [Next →]  │
└──────────────────────────────────────────┘
```

**Step 2 — Link a Repository (Optional)**:
```
┌──────────────────────────────────────────┐
│  Link a Repository               [✕]     │
├──────────────────────────────────────────┤
│  Repository URL  [ https://github.com/…] │
│  Branch          [ main              ]   │
│  Git Account     [ My GitHub Account ▼]  │
│                                          │
│  ← Back    [Skip for now]  [Create →]   │
└──────────────────────────────────────────┘
```

- Git Account dropdown pulls from `/organizations/:orgID/git-accounts`.
- "Skip for now" creates the project without a repo.
- On success → navigate to `/projects/[id]`.

#### 3.2 — Empty State (no projects)
When zero projects exist, show a simple empty state with CTA:
```
┌──────────────────────────────────────────────────────┐
│  No projects yet.                                    │
│                                                      │
│  Create your first project to start running AI tasks.│
│                                                      │
│  [+ Create Project]                                  │
└──────────────────────────────────────────────────────┘
```
> **Note**: The full setup checklist with step-by-step progress lives in the **Phase 8 `SetupChecklist` banner**, which renders above this empty state. Phase 3 only owns the simple empty-state message — no duplicate checklist logic here.

#### 3.3 — Project Cards (already improved)
Stats already hydrate from real APIs (repos, agents, tasks). Keep as-is.

### Acceptance Criteria
- [ ] Step 1 → Step 2 modal navigation works with Back/Next.
- [ ] Skipping repo link creates project and navigates to project detail.
- [ ] Linking a repo on creation creates the repo record immediately.
- [ ] Git Account dropdown only shows connected accounts.
- [ ] Empty state banner shows real check/uncheck status.

### Files
- `web/src/app/page.tsx` — update New Project modal to 2-step
- No new backend files needed.

---

## Phase 4: Agent Management (`/settings` → Members Tab)

### Goal
Make hiring agents simple. Guide new users to "Seed Default Fleet" which creates a ready-to-use team in one click.

### Changes

#### 4.1 — Members Tab in Settings

**Stats row**:
```
Total: 5   Auto-join: 1   Manual: 4   Providers: gateway (4), openai (1)
```

**Seed Default Fleet button**:
- Always visible. **Disabled** (not hidden) with tooltip `"Fleet already seeded"` if agents already exist.
- When enabled (0 agents): creates `Planner` (auto-join, fast), `Backend` (manual, balanced), `Frontend` (manual, balanced), `Reviewer` (manual, powerful), `QA` (manual, balanced).
- Partial failure handling: show toast per agent created, surface any errors individually.

**Agent Cards Grid**:
```
┌────────────────────────────┐
│ Backend Specialist         │
│ Role: backend              │
│ ⚖️  balanced  •  idle      │
│ ● Inherited by all         │  ← auto_join
│ [Assign to project ▼]     │  ← manual only
│                   [Delete] │
└────────────────────────────┘
```

#### 4.2 — "Hire Agent" 2-Step Wizard

**Step 1 — Identity**:
```
Name      [ Backend Specialist      ]
Role      [ backend          ▼      ]
Strategy  ○ Auto-join all projects
          ● Assign to specific projects
```

**Step 2 — Capability**:
```
○ ⚡ Fastest & Cheapest
   gateway / fast  — best for planner, QA

● ⚖️  Smart & Balanced          ← default
   gateway / balanced

○ 💎 Most Capable
   gateway / powerful — best for reviewer, hard tasks

○ 🛠  Custom
   Provider [ openai ▼ ]  Model [ gpt-4o ▼ ]
```

Role → suggested tier (non-enforced hint):
| Role | Hint |
|---|---|
| `planner` | ⚡ Fastest |
| `backend` | ⚖️ Balanced |
| `frontend` | ⚖️ Balanced |
| `reviewer` | 💎 Most Capable |
| `qa` | ⚖️ Balanced |

#### 4.3 — Agent Assignment to Projects
Manual agents show a dropdown: `"Assign to project"` → lists unassigned projects → calls `POST /projects/:projectID/agents`.

### Acceptance Criteria
- [ ] Seed Default Fleet button creates 5 agents in one click.
- [ ] Hire Wizard maps tier → `{ provider, model_route }` correctly.
- [ ] Custom tier shows full provider + model dropdowns from `model-options.ts`.
- [ ] Auto-join agents display "Inherited by all projects" badge.
- [ ] Manual agents can be assigned to a project from the card.
- [ ] Delete agent shows confirm text and removes from org.

### Files
- `web/src/app/settings/page.tsx` — Members tab
- `web/src/lib/model-options.ts` — tier-to-model mapping (already exists)
- `web/src/components/dashboard/hire-agent-wizard.tsx` — new 2-step modal component

---

## Phase 5: Assign Skills to Agents (`/settings` → Members tab)

### Goal
Let users define **what tools each agent is allowed to use** by assigning Skills from the global skill library. Without skills, agents can only use built-in defaults.

### What are Skills?
Skills are named capability packages registered in the system (e.g. `code-review`, `file-write`, `run-tests`, `create-migration`). An agent can only invoke tools from its assigned skills.

**Backend**: All routes already exist.
- `GET /skills` — list all available skills in the org
- `GET /agents/:agentID/skills` — list skills currently assigned to an agent
- `POST /agents/:agentID/skills` — assign a single skill
- `PUT /agents/:agentID/skills` — bulk replace all skills

### UI Changes

#### 5.1 — Skills Panel on Agent Card (expanded view)
Each agent card in the Members tab has an expandable **Skills** section:

```
┌──────────────────────────────────────────────┐
│ Backend Specialist     backend  •  balanced   │
│                                              │
│ Skills (3)  [▸ expand]                       │
├──────────────────────────────────────────────┤
│  (expanded)                                  │
│  ✓ read-file     ✓ write-file   ✓ run-tests  │
│  ○ create-migration  ○ scan-vulnerabilities   │
│  ○ analyze-diff  ○ git-commit                │
│                                              │
│  [Save Skills]                               │
└──────────────────────────────────────────────┘
```

- Show all available skills as a **checkbox grid** (fetched from `GET /skills`).
- Pre-check skills already assigned to the agent.
- "Save Skills" calls `PUT /agents/:agentID/skills` with the full list of checked IDs.
- Show a `✓ Saved` flash for 2 seconds on success.

#### 5.2 — Skills Summary on Collapsed Card
When collapsed, show a compact skill count badge:
```
Backend Specialist    backend  •  balanced  •  3 skills
```

#### 5.3 — Skill tags with tier color
Group skills visually by function category (from skill metadata):
| Category | Color |
|---|---|
| `file` | blue |
| `git` | emerald |
| `test` | amber |
| `security` | rose |
| `database` | purple |
| other | slate |

### Acceptance Criteria
- [ ] Available skills load from `GET /skills`.
- [ ] Currently assigned skills are pre-checked on expand.
- [ ] Saving updates via `PUT /agents/:agentID/skills`.
- [ ] Skill count badge shows correctly on collapsed card.
- [ ] Bulk save shows `✓ Saved` flash feedback.

### Files
- `web/src/app/settings/page.tsx` — extend agent card with skills expand section
- No new backend files needed.

---

## Phase 6: Add Rules to Project (`/projects/[id]` → Settings tab)

### Goal
Let users define **behavioral guardrails** for agents operating inside a project. Rules are short natural-language constraints that the prompt assembler injects into every agent context.

### What are Rules?
- **Scope**: `project` — applies only within this project. (Global/org rules are a planned future feature.)
- **Enforcement**:
  - `strict` — violation is treated as a hard failure by the agent.
  - `advisory` — treated as a preference; agent may deviate with reasoning.
- **Examples**:
  - `"Always write unit tests for every function you create."` (strict)
  - `"Prefer async/await over Promise.then() chains."` (advisory)
  - `"Never commit directly to main — always use a feature branch."` (strict)

**Backend**: All CRUD routes already exist.
- `GET /projects/:projectID/rules` — list project rules
- `POST /projects/:projectID/rules` — create a rule
- `PATCH /rules/:ruleID` — update a rule (content, enforcement)
- `DELETE /rules/:ruleID` — delete a rule

### UI Changes

#### 6.1 — Rules Section in Project Settings Tab

**Empty state**:
```
┌────────────────────────────────────────┐
│  No rules yet.                         │
│                                        │
│  Rules guide agent behavior inside     │
│  this project. Add one to get started. │
│  [+ Add Rule]                          │
└────────────────────────────────────────┘
```

**Rule list**:
```
┌──────────────────────────────────────────────────────────┐
│  ● Always write unit tests.        [strict]   [✎] [✕]   │
│  ○ Prefer functional patterns.   [advisory]   [✎] [✕]   │
└──────────────────────────────────────────────────────────┘
```
- `strict` badge → red/rose
- `advisory` badge → amber
- `[✎]` Edit button: toggles inline edit mode (textarea + enforcement toggle + Save/Cancel)
- `[✕]` Delete button: confirm prompt → `DELETE /rules/:ruleID` → remove from list

#### 6.2 — Add Rule Inline Form (below the list)
```
Rule content *   [ Always write unit tests for every new function. ]
Enforcement      ● Strict  ○ Advisory
                 [ Add Rule ]
```

- Textarea, min-height 60px.
- Enforcement toggle, default = `strict`.
- On submit: calls `POST /projects/:projectID/rules`, appends to list on success.
- Form resets after success.

#### 6.3 — Inline Edit Mode (per rule)
When user clicks `[✎]`, the rule row transforms into an edit form:
```
┌──────────────────────────────────────────────────────────┐
│  [ Always write unit tests for every function.        ]  │
│  ● Strict  ○ Advisory              [Cancel]  [Save]     │
└──────────────────────────────────────────────────────────┘
```
- On save: calls `PATCH /rules/:ruleID` with updated content/enforcement.
- Cancel reverts to read-only display.

#### 6.4 — Rule Count Badge on Settings Tab
Tab label shows count when rules exist:
```
Settings  ·  2 rules
```

### Acceptance Criteria
- [ ] Rules list loads from `GET /projects/:projectID/rules` on tab open.
- [ ] Add Rule form creates rule and appends to list without page reload.
- [ ] Enforcement badge (`strict` / `advisory`) is visually distinct.
- [ ] Empty state is shown when no rules exist.
- [ ] Rule count badge appears on the Settings tab label.
- [ ] Edit button toggles inline edit mode with pre-filled content.
- [ ] Save updates rule via `PATCH /rules/:ruleID`.
- [ ] Delete button shows confirm prompt and removes rule via `DELETE /rules/:ruleID`.

### Files
- `web/src/app/projects/[id]/page.tsx` — Settings tab rules section
- No new backend files needed.

### API Client Additions Required
- `api.updateRule(ruleID, token, input)` → `PATCH /rules/:ruleID`
- `api.deleteRule(ruleID, token)` → `DELETE /rules/:ruleID`

---

## Phase 7: Project Detail & Task Creation (`/projects/[id]`)

### Goal
Inside a project, make it easy to see what is happening and to create a new task. The Settings tab (Phase 6 rules, Phase 5 skills in project-agent view) lives here.

### Layout

```
/projects/[id]
├── Left Sidebar
│   ├── Repositories (link / add)
│   └── Create Task button
└── Right Main Panel
    ├── Tab: Tasks      ← default
    ├── Tab: Members    ← agents (global vs project-specific)
    └── Tab: Settings   ← project metadata, rules (Phase 6), agent skills
```

#### 7.1 — Repositories Sidebar

**Empty state**:
```
No repository linked.
[+ Link Repository]
```

**Linked repo card**:
```
github.com/org/my-repo
Branch: main  •  Clone: done
[Clone] [Validate]
```

Add-repo inline form:
- URL field
- Branch field (default: `main`)
- Git Account dropdown (from Phase 2). If no git accounts exist, show link: "Connect a Git account first → /settings"
- Provider auto-detected from URL.

#### 7.2 — Tasks Tab (main panel)

**Empty state**:
```
No tasks yet.

To get started:
1. Make sure at least one agent is assigned to this project.
2. Click "Create Task" in the sidebar.
```

**Task card**:
```
┌─────────────────────────────────────────┐
│ Fix login redirect bug          [easy]  │
│ Status: analyzing  •  2m ago           │
│ Agent: Backend Specialist               │
│                              [Details] │
└─────────────────────────────────────────┘
```

**Create Task sidebar panel** (slide-in from right, overlay):
```
Title *       [ Short imperative title       ]
Description   [ What to do and why           ]
Complexity    ○ Easy  ● Medium  ○ Hard
Priority      [ 0                            ]
Labels        [ bug, backend                 ]
Agent         [ Auto-assign ▼               ]  ← NEW: dropdown of project agents
              [ Create Task ]
```

Agent dropdown: lists agents assigned to this project. "Auto-assign" (default) lets the system pick based on role matching.

After creation: task appears in the Tasks tab, status = `pending`.

#### 7.3 — Members Tab

Split agents from `api.listAgents(projectID)` into two groups:

**Global (auto_join)**:
```
─── Inherited from Organization ────────────
  Planner Agent     planner  •  fast  •  idle
```

**Project-specific (manual)**:
```
─── Assigned to this Project ────────────────
  Backend Specialist  backend  •  balanced  •  idle
  [+ Assign Agent from Organization]
```

Assign form: dropdown of org agents that are `manual` and not yet assigned to this project.

#### 7.4 — Settings Tab
- Project name/description edit form.
- **Rules section** (Phase 6 content rendered here).
- Agent skill assignment visible per agent (links to Phase 5 skill panel).

### Acceptance Criteria
- [ ] Repository can be linked from sidebar with Git Account selection.
- [ ] Task creation panel is accessible from sidebar button.
- [ ] Tasks tab shows live status from API (auto-refresh every 5s when a task is `analyzing` or `running`).
- [ ] Members tab splits auto-join vs manual agents.
- [ ] Settings tab shows rules list + add-rule form.

### Files
- `web/src/app/projects/[id]/page.tsx` — major restructure
- `web/src/components/dashboard/create-task-panel.tsx` — new slide-in panel
- `web/src/components/dashboard/repo-sidebar.tsx` — new sidebar component

---

## Phase 8: Setup Checklist Banner (Global)

### Goal
Show a persistent-but-dismissible banner on the Projects page (`/`) to guide new users through the first-time setup sequence.

### Checklist Banner

```
┌──────────────────────────────────────────────────────────────────┐
│  Getting Started                                          [✕]    │
│                                                                  │
│  ✅  Add an AI provider key          /ai-providers               │
│  ✅  Configure organization rules    /rules                      │
│  ✅  Seed or add global skills       /skills                     │
│  ◻   Add an agent                    /agents                     │
│  ✅  Connect a GitHub account        /git-accounts               │
│  ✅  Create a project                /                           │
│  ◻   Configure project rule/skill    Open project → Settings     │
│  ◻   Create your first task          Open a project              │
│                                                                  │
│  Complete setup to run your first AI task.                       │
└──────────────────────────────────────────────────────────────────┘
```

### State Logic
| Check | How to determine |
|---|---|
| AI provider key | `credentials.length > 0` from SWR |
| Organization rules | `globalRules.length > 0` from SWR |
| Global skills | `skills.length > 0` from SWR |
| Organization agent | `orgAgents.length > 0` from SWR |
| GitHub account | `gitAccounts.length > 0` from SWR |
| Project created | `projects.length > 0` from SWR |
| Project rule/skill | First project has a project-scoped rule and at least one org agent has assigned skills |
| Task created | `overview.total_tasks > 0` from SWR |

**Dismiss**: store dismissal in `localStorage`. Once dismissed, never re-show. Re-show if user clears localStorage.

### Acceptance Criteria
- [ ] Banner appears on first load with no setup done.
- [ ] Each check updates in real-time as user completes steps.
- [ ] Banner auto-hides when all 8 required checks pass.
- [ ] Organization rules and global skills remain separate first-class setup steps.
- [ ] User can manually dismiss the banner.

### Files
- `web/src/components/dashboard/setup-checklist.tsx` — new component
- `web/src/app/page.tsx` — render banner above project cards

---

## Implementation Order

Execute phases in this sequence. Each phase is independently deployable.

```
Phase 1 → Polish /ai-providers                    (small, already 80% done)
Phase 2 → Build Git Accounts tab in /settings
Phase 4 → Build Members tab in /settings          (depends on: Phase 1)
Phase 5 → Add Skills panel to agent card           (depends on: Phase 4)
Phase 3 → Enhance project creation modal           (depends on: Phase 2)
Phase 6 → Add Rules section in project Settings    (depends on: Phase 3)
Phase 7 → Restructure /projects/[id]               (depends on: Phase 3 + 4 + 5 + 6)
Phase 8 → Setup checklist banner                   (depends on: all phases)
```

---

## Shared Components

Only extract when used in 2+ places:

| Component | Location | Used by |
|---|---|---|
| `HireAgentWizard` | `components/dashboard/hire-agent-wizard.tsx` | Settings Members tab |
| `CreateTaskPanel` | `components/dashboard/create-task-panel.tsx` | Project detail |
| `RepoSidebar` | `components/dashboard/repo-sidebar.tsx` | Project detail |
| `SetupChecklist` | `components/dashboard/setup-checklist.tsx` | Projects list page |
| `GitAccountsTab` | `components/settings/git-accounts-tab.tsx` | Settings page |
| `MembersTab` | `components/settings/members-tab.tsx` | Settings page |

---

## UX Design Principles (inspired by Multica design system)

### Visual Foundation
- **No emojis as icons**: use Lucide SVG icons throughout.
- **Cursor pointer**: all clickable elements including cards, badges, and dropdowns.
- **Empty states are helpful**: every empty state explains what is missing and what to do next.
- **Progressive disclosure**: show only what is needed at each step. Hide advanced options behind "Custom" or secondary flows.

### Loading & Skeleton States (Global Requirement)
Every component that fetches data on mount MUST implement loading states:
- Use **skeleton shimmer** (pulsing `bg-surface animate-pulse`) for card grids, lists, and tab content.
- Reserve exact layout dimensions to prevent CLS (Cumulative Layout Shift).
- Show `--` as inline placeholder for numeric values that haven't loaded yet.

### Toast & Error Strategy (Global Requirement)
All mutations (create, update, delete) follow the same feedback pattern:
- **Success**: Show `✓ Saved` / `✓ Created` / `✓ Deleted` flash on the trigger button for 2s, then reset.
- **Error**: Show error toast via a global toast provider (Sonner-style) at bottom-right. Toast auto-dismisses after 5s.
- **Destructive actions**: Always require a confirm prompt before executing (delete agent, delete rule, disconnect git account).

### Animations & Micro-interactions (inspired by Multica)
- **Smooth transitions**: all hover/focus state changes use `transition-all duration-150`.
- **Status dots**: pulsing dot (`animate-pulse-dot`) for `active`/`running`; static for `idle`; dimmed for `offline`.
- **Glow-on-hover**: interactive cards use the existing `.glow-on-hover` class for accent border glow.
- **Entrance animations**: new list items (rules, agents, tasks) fade in with a subtle 200ms opacity transition.
- **No layout shift on animation**: never use `transform: translateY()` on elements inside scrollable containers (causes scrollbar flash).

### Accessibility
- All interactive elements must have `:focus-visible` outlines (already defined in `globals.css`).
- Respect `prefers-reduced-motion: reduce` (already handled globally).
- All form inputs must have associated labels (visible or `aria-label`).

---

## API Client Additions Required Before UI Work

The following functions are **missing** from `web/src/lib/api/` and must be added:

| Function | Route | Used by |
|---|---|---|
| `api.updateRule` | `PATCH /rules/:ruleID` | Phase 6 (edit rule) |
| `api.deleteRule` | `DELETE /rules/:ruleID` | Phase 6 (delete rule) |
| `api.bulkReplaceAgentSkills` | `PUT /agents/:agentID/skills` | Phase 5 (save skills) |

---

## Backend Follow-up (Not Blocking UI)

These are needed for full functionality but not for the initial UI pass:

| Feature | Routes needed |
|---|---|
| Remove agent from project only | `DELETE /projects/:projectID/agents/:agentID` |
| Global org-level rules | `GET/POST /organizations/:orgID/rules` |
| Provider model discovery | `GET /providers/:provider/models` |
| Skill categories (for color tags) | Add `category` field to Skill model or derive client-side from name |

---

## Test Checklist (Manual)

Run through this on each phase before marking done:

- [ ] Phase 1: Add OpenAI key → Test → see green success badge.
- [ ] Phase 2: Connect GitHub account → Test → see "Connected" card.
- [ ] Phase 3: Create project → link repo → navigate to project detail.
- [ ] Phase 4: Seed fleet → confirm 5 agents created → assign Backend to project.
- [ ] Phase 5: Open agent card → assign skills → confirm count badge updates.
- [ ] Phase 6: Open project Settings → add a strict rule → rule appears in list.
- [ ] Phase 7: Create task → see it in Tasks tab with `pending` status → Members tab splits auto-join vs manual.
- [ ] Phase 8: Banner shows on fresh load → all checks complete → banner disappears or dims recommended steps.
