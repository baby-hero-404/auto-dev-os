# Phase 7: Project Detail & Task Creation (`/projects/[id]`)

## Goal
Inside a project, make it easy to see what is happening and to create a new task. The Settings tab (Phase 6 rules, Phase 5 skills in project-agent view) lives here.

## Layout

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

### 7.1 — Repositories Sidebar

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
- Git Account dropdown (from Phase 2). If no git accounts exist, show link: `"Connect a Git account first → /settings"`
- Provider auto-detected from URL.

### 7.2 — Tasks Tab (main panel)

**Empty state**:
```
No tasks yet.

To get started:
1. Make sure at least one agent is assigned to this project.
2. Click "Create Task" in the sidebar.
```

**Task card** (uses `.glow-on-hover`):
```
┌─────────────────────────────────────────┐
│ Fix login redirect bug          [easy]  │
│ Status: analyzing  •  2m ago           │
│ Agent: Backend Specialist               │
│                              [Details] │
└─────────────────────────────────────────┘
```
- Status badge uses existing `Badge` component with color mapping.
- Running/analyzing tasks show a pulsing status dot via `animate-pulse-dot`.

**Create Task sidebar panel** (slide-in from right, overlay):
```
Title *       [ Short imperative title       ]
Description   [ What to do and why           ]
Complexity    ○ Easy  ● Medium  ○ Hard
Priority      [ 0                            ]
Labels        [ bug, backend                 ]
Agent         [ Auto-assign ▼               ]  ← dropdown of project agents
              [ Create Task ]
```

- **Agent dropdown**: lists agents assigned to this project. "Auto-assign" (default) lets the system pick based on role matching. This maps to `agent_id: null` in the API call.
- After creation: task appears in the Tasks tab, status = `pending`.

**Task Detail page**: Clicking `[Details]` navigates to `/projects/[id]/tasks/[taskId]`. This page is **not scoped in Phase 7** — it already exists from prior work. Phase 7 focuses only on the project dashboard and task creation.

### 7.3 — Members Tab

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

### 7.4 — Settings Tab
- Project name/description edit form.
- **Rules section** (Phase 6 content rendered here — including edit/delete).
- Agent skill assignment visible per agent (links to Phase 5 skill panel).

### 7.5 — Loading & Skeleton States
- Sidebar repos: show skeleton card while `GET /projects/:id/repositories` loads.
- Tasks tab: show skeleton cards grid while `GET /projects/:id/tasks` loads.
- Members tab: show skeleton rows while `GET /projects/:id/agents` loads.
- Auto-refresh: poll tasks every 5s **only** when at least one task has status `analyzing` or `running`. Stop polling when all tasks are settled.

## Acceptance Criteria
- [x] Repository can be linked from sidebar with Git Account selection.
- [x] Git Account dropdown shows "Connect first" link if no accounts exist.
- [x] Task creation panel is accessible from sidebar button.
- [x] Agent dropdown in task creation lists project-assigned agents.
- [x] Tasks tab shows live status from API (auto-refresh every 5s when active tasks exist).
- [x] Members tab splits auto-join vs manual agents.
- [x] Settings tab shows rules list + add-rule form + edit/delete.
- [x] Skeleton loading states render during data fetch.

## Files
- `web/src/app/projects/[id]/page.tsx` — major restructure
- `web/src/components/dashboard/create-task-panel.tsx` — new slide-in panel
- `web/src/components/dashboard/repo-sidebar.tsx` — new sidebar component
