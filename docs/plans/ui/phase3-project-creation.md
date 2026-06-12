# Phase 3: Project Creation Flow (`/`)

## Goal
Make creating a project fast and clear. Optionally link a repository immediately so the user does not need to go back to add one.

## Changes

### 3.1 — "New Project" Modal Enhancement
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
- If no git accounts exist, dropdown is replaced with a link: `"Connect a Git account first → /git-accounts"`.
- "Skip for now" creates the project without a repo.
- On success → navigate to `/projects/[id]`.
- **Failure rollback**: If project creation succeeds but repo linking fails, keep the project and show an error toast: `"Project created, but repo could not be linked. You can add it later from the project page."`.

### 3.2 — Empty State (no projects)
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

### 3.3 — Project Cards (already improved)
Stats already hydrate from real APIs (repos, agents, tasks). Keep as-is.

### 3.4 — Loading & Skeleton States
- Project cards grid: show 3 skeleton cards (pulsing `bg-surface`) while `projects` SWR is loading.
- Modal steps: disable "Next →" / "Create →" buttons and show spinner while API call is in-flight.

## Acceptance Criteria
- [x] Step 1 → Step 2 modal navigation works with Back/Next.
- [x] Skipping repo link creates project and navigates to project detail.
- [x] Linking a repo on creation creates the repo record immediately.
- [x] Git Account dropdown only shows connected accounts (or fallback link if none).
- [x] Empty state shows below the Phase 8 checklist banner (no duplication).
- [x] Skeleton loading states render while projects are loading.
- [x] Error toast appears if repo linking fails after project creation.

## Files
- `web/src/app/page.tsx` — update New Project modal to 2-step ✅ done
- No new backend files needed.
