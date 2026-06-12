# Phase 5: Assign Skills to Agents (`/settings` → Members tab)

## Goal
Let users define **what tools each agent is allowed to use** by assigning Skills from the global skill library. Without skills, agents can only use built-in defaults.

## What are Skills?
Skills are named capability packages registered in the system (e.g. `code-review`, `file-write`, `run-tests`, `create-migration`). An agent can only invoke tools from its assigned skills.

**Backend**: All routes already exist.
- `GET /skills` — list all available skills in the org
- `GET /agents/:agentID/skills` — list skills currently assigned to an agent
- `POST /agents/:agentID/skills` — assign a single skill
- `PUT /agents/:agentID/skills` — bulk replace all skills

## UI Changes

### 5.1 — Skills Panel on Agent Card (expanded view)
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
- "Save Skills" calls `PUT /agents/:agentID/skills` with the full list of checked IDs (bulk replace).
- Show a `✓ Saved` flash for 2 seconds on success.
- Unassigning a skill = uncheck + save all. No individual `DELETE` call needed.

### 5.2 — Skills Summary on Collapsed Card
When collapsed, show a compact skill count badge:
```
Backend Specialist    backend  •  balanced  •  3 skills
```

### 5.3 — Skill Tags with Category Color
Group skills visually by inferring category from the skill **name** (client-side heuristic since the `Skill` model has no `category` field):

| Name pattern | Category | Color |
|---|---|---|
| `*file*`, `*read*`, `*write*` | file | blue |
| `*git*`, `*commit*`, `*push*` | git | emerald |
| `*test*`, `*qa*` | test | amber |
| `*security*`, `*scan*`, `*vuln*` | security | rose |
| `*migration*`, `*db*`, `*schema*` | database | purple |
| everything else | other | slate |

> **Future**: When the backend adds a `category` field to the Skill model, switch from client-side heuristic to server-provided categories.

### 5.4 — Scalability Note
The checkbox grid works well for up to ~15 skills. If the skill library grows significantly:
- Switch to a searchable multi-select with grouped sections.
- Add a search/filter input at the top of the expanded panel.
- This is a future enhancement, not blocking initial implementation.

### 5.5 — Loading States
- Skills panel: show skeleton shimmer while `GET /skills` loads.
- Pre-checked skills: show loading spinner inside the expand button while `GET /agents/:agentID/skills` loads.

## Acceptance Criteria
- [x] Available skills load from `GET /skills`.
- [x] Currently assigned skills are pre-checked on expand.
- [x] Saving updates via `PUT /agents/:agentID/skills` (bulk replace).
- [x] Skill count badge shows correctly on collapsed card.
- [x] Bulk save shows `✓ Saved` flash feedback.
- [x] Skill tags show category colors based on name heuristic.
- [x] Skeleton loading states render during data fetch.

## Files
- `web/src/components/settings/members-tab.tsx` — extend agent card with skills expand section
- No new backend files needed.

## API Client Addition Required
- `api.bulkReplaceAgentSkills(agentID, skillIDs[], token)` → `PUT /agents/:agentID/skills`
