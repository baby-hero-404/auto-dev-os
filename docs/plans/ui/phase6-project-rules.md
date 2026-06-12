# Phase 6: Add Rules to Project (`/projects/[id]` → Settings tab)

## Goal
Let users define **behavioral guardrails** for agents operating inside a project. Rules are short natural-language constraints that the prompt assembler injects into every agent context.

## What are Rules?
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

## UI Changes

### 6.1 — Rules Section in Project Settings Tab

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
- `[✕]` Delete button: confirm prompt → `DELETE /rules/:ruleID` → remove from list with fade-out

### 6.2 — Add Rule Inline Form (below the list)
```
Rule content *   [ Always write unit tests for every new function. ]
Enforcement      ● Strict  ○ Advisory
                 [ Add Rule ]
```

- Textarea, min-height 60px.
- Enforcement toggle, default = `strict`.
- On submit: calls `POST /projects/:projectID/rules`, appends to list on success.
- New rule fades in with 200ms entrance animation.
- Form resets after success.

### 6.3 — Inline Edit Mode (per rule)
When user clicks `[✎]`, the rule row transforms into an edit form:
```
┌──────────────────────────────────────────────────────────┐
│  [ Always write unit tests for every function.        ]  │
│  ● Strict  ○ Advisory              [Cancel]  [Save]     │
└──────────────────────────────────────────────────────────┘
```
- Pre-fill with current `content` and `enforcement` values.
- On save: calls `PATCH /rules/:ruleID` with updated fields.
- Show `✓ Saved` flash for 2s on success.
- Cancel reverts to read-only display.

### 6.4 — Delete Confirmation
When user clicks `[✕]`:
- Show an inline confirmation: `"Delete this rule?" [Yes] [No]`
- On confirm: calls `DELETE /rules/:ruleID`, removes from list with fade-out transition.
- Show error toast if deletion fails.

### 6.5 — Rule Count Badge on Settings Tab
Tab label shows count when rules exist:
```
Settings  ·  2 rules
```

### 6.6 — Loading State
- Show skeleton shimmer for rule list while `GET /projects/:projectID/rules` loads.
- Show spinner on Save/Add buttons during API calls.

## Acceptance Criteria
- [x] Rules list loads from `GET /projects/:projectID/rules` on tab open.
- [x] Add Rule form creates rule and appends to list without page reload.
- [x] Enforcement badge (`strict` / `advisory`) is visually distinct.
- [x] Empty state is shown when no rules exist.
- [x] Rule count badge appears on the Settings tab label.
- [x] Edit button toggles inline edit mode with pre-filled content.
- [x] Save updates rule via `PATCH /rules/:ruleID`.
- [x] Delete button shows confirm prompt and removes rule via `DELETE /rules/:ruleID`.
- [x] Loading skeletons display while data is fetching.

## Files
- `web/src/app/projects/[id]/page.tsx` — Settings tab rules section
- No new backend files needed.

## API Client Additions Required
- `api.updateRule(ruleID, token, input)` → `PATCH /rules/:ruleID`
- `api.deleteRule(ruleID, token)` → `DELETE /rules/:ruleID`
