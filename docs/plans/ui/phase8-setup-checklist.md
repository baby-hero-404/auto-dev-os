# Phase 8: Setup Checklist Banner (Global)

## Goal
Show a persistent-but-dismissible banner on the Projects page (`/`) to guide new users through the first-time setup sequence. This is the **single source of truth** for onboarding progress — no other component should duplicate this checklist logic.

## Checklist Banner

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

> **Ownership**: This banner is the sole owner of the setup checklist UI. Phase 3's empty state shows a simple "No projects yet" message — it does NOT duplicate checklist items.

## State Logic
| Check | How to determine | Required? |
|---|---|---|
| AI provider key | `credentials.length > 0` from SWR | ✅ Required |
| Organization rules | `globalRules.length > 0` from SWR | ✅ Required |
| Global skills | `skills.length > 0` from SWR | ✅ Required |
| Organization agent | `orgAgents.length > 0` from SWR | ✅ Required |
| GitHub account | `gitAccounts.length > 0` from SWR | ✅ Required |
| Project created | `projects.length > 0` from SWR | ✅ Required |
| Project rule + skill | First project has a project-scoped rule and at least one org agent has assigned skills | ✅ Required |
| Task created | `overview.total_tasks > 0` from SWR | ✅ Required |

### Rules Check: Lazy Evaluation
Checking rules across all projects is potentially expensive. Implementation:
- Only check rules for the **first project** (`projects[0].id`) to avoid N+1 queries.
- Show as a required setup item in the full product onboarding flow.
- If the user has 0 projects, skip the rules check entirely.

## Visual Design
- **Required checks**: Green `✅` when complete, hollow `◻` when incomplete.
- **Auto-hide**: Banner auto-hides when all 8 required checks pass.
- **Progress indicator**: Show `"3 of 8 complete"` count in the banner header.

## Animation
- Banner entrance: fade-in with 300ms ease on first render.
- Each check: when a check transitions from ◻ → ✅, use a brief scale-pop animation (similar to Multica's `completion-badge` effect: scale 0 → 1.12 → 1, 300ms).
- Dismissal: fade-out 200ms.

## Dismiss Logic
- **Dismiss**: store dismissal in `localStorage` under key `setup-checklist-dismissed`.
- Once dismissed, never re-show unless localStorage is cleared.
- Banner auto-hides (distinct from dismiss) when all required checks pass — this sets a separate `setup-checklist-auto-completed` key so it stays hidden even if a check regresses.

## Loading State
- Show skeleton banner (pulsing placeholder) while initial SWR data loads.
- Render actual checks only after all required data (`credentials`, `gitAccounts`, `projects`, `orgAgents`, `overview`) have resolved.

## Acceptance Criteria
- [ ] Banner appears on first load with no setup done.
- [ ] Each check updates in real-time as user completes steps.
- [ ] Banner auto-hides when all 8 required checks pass.
- [ ] Organization rules and global skills remain separate required steps.
- [ ] User can manually dismiss the banner.
- [ ] Check completion shows scale-pop animation.
- [ ] Skeleton loading state renders while data loads.
- [ ] No duplicate checklist in Phase 3's empty state.

## Files
- `web/src/components/dashboard/setup-checklist.tsx` — new component
- `web/src/app/page.tsx` — render banner above project cards
