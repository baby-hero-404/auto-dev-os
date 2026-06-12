# Phase 2: Git Accounts (`/git-accounts`)

## Goal
Allow users to connect a GitHub / GitLab account at the organization level so agents can clone and push repos.

## Context
The backend already has `/organizations/:orgID/git-accounts` with CRUD + test. The UI must expose this clearly.

## What to Build

### 2.1 — Settings Page Tabs
Move `/settings` to a tab-based layout:
```
Settings
├── Git Accounts   ← Phase 2
└── Members        ← Phase 4
```
Use a simple tab component with underline indicator. Tab state stored in URL hash or local state.

> **Component extraction**: Since both this tab and Phase 4 (Members) live in `settings/page.tsx`, extract each tab into its own component file:
> - `web/src/components/settings/git-accounts-tab.tsx`
> - `web/src/components/settings/members-tab.tsx` (Phase 4)

### 2.2 — Git Accounts Tab UI

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

**Error handling**: If `Connect & Test` fails:
- Show inline error message below the form: `"Connection failed: invalid token or unreachable URL"`
- Keep form open with values preserved for correction.
- If creation succeeds but test fails: show the account card with a warning badge `"Not verified"`.

**Connected Account Card** (uses `.glow-on-hover`):
```
┌─────────────────────────────────────┐
│  GitHub icon   My GitHub Account    │
│  github.com  ·  Connected 2h ago   │
│                         [Test] [✕] │
└─────────────────────────────────────┘
```

**Test button states**: `Testing…` → `✓ OK` (green, 3s) or `✗ Failed` (red, 3s) → resets.

### 2.3 — Per-Project Repository Linking (appears in Phase 3)
When creating a repository inside a project, show a dropdown of connected Git accounts so the user can select which account to use for clone/push.

### 2.4 — Loading & Skeleton States
- Show skeleton card while `GET /organizations/:orgID/git-accounts` loads.
- Disable form buttons and show spinner during `POST` and `POST .../test` calls.

## Acceptance Criteria
- [x] User can connect a GitHub account with a token.
- [x] Test connection shows live feedback.
- [x] Connected accounts are listed with display name and provider.
- [x] User can delete an account (with confirm prompt).
- [x] Form token field has show/hide toggle.
- [x] Inline form error handling shows clear message on failure.
- [x] Skeleton loading states render during data fetch.

## Files
- `web/src/app/git-accounts/page.tsx` — dedicated Git Accounts route ✅ done
- `web/src/components/settings/git-accounts-tab.tsx` — new extracted tab content ✅ done
- No new backend files (routes already exist).
