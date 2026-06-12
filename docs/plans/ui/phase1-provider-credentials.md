# Phase 1: AI Provider Credentials (`/ai-providers`)

## Goal
Allow users to add, test, and delete API keys for LLM providers so agents can call models.

## What Already Works
The page at `/ai-providers` exists and has full CRUD. **Phase 1 is mostly polish.**

## Changes

### 1.1 — Layout & Visual Polish
- Add `.glow-on-hover` to each provider card for accent border glow on hover.
- Show a pulsing `active` dot (using `animate-pulse-dot`) in the status badge when credentials are present.
- Animate the `CheckCircle` icon on status badge with `animate-pulse` when `count > 0`.

### 1.2 — Form UX
- **API Key show/hide toggle**: Add `Eye` / `EyeOff` button inside the API key field. ✅ (already done)
- **Context-aware Base URL placeholder**: Change placeholder based on selected provider. ✅ (already done)
- **Priority hint text**: Add `"Lower = runs first (0 = highest priority)"` label next to Priority.
- **After save**: Flash a `✓ Saved` success state on the Save button for 2 seconds before reset. Use global toast strategy on error.

### 1.3 — Empty State Guidance
When no credentials exist for a provider, replace the dashed border with a message:
```
"No keys yet. Add one to enable this provider for your agents."
```

### 1.4 — Loading State
- Show skeleton shimmer for provider cards while `GET /provider-credentials` loads.
- Disable Test button and show spinner during `POST .../test` call.

## Acceptance Criteria
- [x] User can add a credential and immediately test it without page reload.
- [x] Status badge updates after test (success → green, failure → red, then resets after 3s).
- [x] Show/hide toggle works on the API key field.
- [x] Provider cards with credentials are visually distinct from empty ones.
- [x] Skeleton loading states render during initial data fetch.

## Files
- `web/src/app/ai-providers/page.tsx` — primary file ✅ done
- No new files needed.
