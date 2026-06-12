# Phase 4: Agent Management (`/settings` or `/agents`)

## Goal
Make hiring agents simple. Guide new users to "Seed Default Fleet" which creates a ready-to-use team in one click.

## Changes

### 4.1 — Members Tab in Settings

**Stats row**:
```
Total: 5   Auto-join: 1   Manual: 4   Providers: gateway (4), openai (1)
```

**Seed Default Fleet button**:
- Always visible. **Disabled** (not hidden) with tooltip `"Fleet already seeded"` if agents already exist.
- When enabled (0 agents): creates `Planner` (auto-join, fast), `Backend` (manual, balanced), `Frontend` (manual, balanced), `Reviewer` (manual, powerful), `QA` (manual, balanced).
- Partial failure handling: show toast per agent created, surface any errors individually. If some agents fail, the button stays enabled with updated tooltip `"X of 5 agents created — click to retry"`.

**Agent Cards Grid**:
```
┌────────────────────────────┐
│ Backend Specialist         │
│ Role: backend              │
│ ⚖️  balanced  •  idle      │
│ ● Inherited by all         │  ← auto_join
│ Skills (3)                 │  ← skill count badge
│ [Assign to project ▼]     │  ← manual only
│                   [Delete] │
└────────────────────────────┘
```

Cards use `.glow-on-hover` for accent glow effect on hover.

### 4.2 — "Hire Agent" 2-Step Wizard

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

> **Note**: Agent `system_prompt` / `instructions` editing is intentionally deferred. The `goal` field in the Hire Wizard serves as the instruction seed. Full prompt editing will be a future enhancement.

### 4.3 — Agent Assignment to Projects
Manual agents show a dropdown: `"Assign to project"` → lists unassigned projects → calls `POST /projects/:projectID/agents`.
- Requires `GET /organizations/:orgID/projects` to populate the dropdown.

### 4.4 — Loading & Skeleton States
- Agent cards grid: show 3 skeleton cards while `orgAgents` SWR is loading.
- Seed Fleet button: show spinner inside button while seeding is in progress.
- Stats row: show `--` for all counts until data loads.

## Acceptance Criteria
- [x] Seed Default Fleet button creates 5 agents in one click.
- [x] Seed button is disabled (not hidden) with tooltip when agents exist.
- [x] Partial failure shows per-agent toast and button remains actionable.
- [x] Hire Wizard maps tier → `{ provider, model_route }` correctly.
- [x] Custom tier shows full provider + model dropdowns from `model-options.ts`.
- [x] Auto-join agents display "Inherited by all projects" badge.
- [x] Manual agents can be assigned to a project from the card.
- [x] Delete agent shows confirm prompt and removes from org.
- [x] Skeleton loading states render during initial data fetch.

## Files
- `web/src/components/settings/members-panel.tsx` — members tab content ✅ done
- `web/src/lib/model-options.ts` — tier-to-model mapping (already exists) ✅ done
- `web/src/components/dashboard/hire-agent-wizard.tsx` — new 2-step modal component ✅ done
