# Auto Code OS — Codebase Review & Cleanup Plan

> **Generated:** 2026-06-17
> **Scope:** Full-stack review of `server/` (Go) and `web/` (Next.js/TypeScript)
> **Approach:** Feature-by-feature review, verifying implementation against `docs/features/` specs, identifying dead code, inconsistencies, and cleanup opportunities.

---

## Executive Summary

After a thorough exploration of the codebase:

- ✅ **Go builds cleanly** — `go build ./...` and `go vet ./...` pass with zero errors.
- ✅ **All Go tests pass** — 15 packages tested, all OK.
- ✅ **TypeScript compiles cleanly** — `tsc --noEmit` passes.
- ⚠️ **Several structural issues found** — dead code, doc/code mismatches, missing routes, and deprecated artifacts that need cleanup.

---

## Issue Index

| # | Feature Area | Severity | Issue | Status |
|---|:-------------|:---------|:------|:-------|
| 1 | Virtual Keys (§5.1.F) | 🟡 Medium | Dead code: Handler, Service, Repo, Model, Frontend exist but **no routes registered** in `router.go` and **not wired** in `main.go`. Docs say "Deferred/Distant Future". | ✅ |
| 2 | Task System (§5.6) | 🟡 Medium | **Deprecated statuses still active in code:** `assigned`, `planning`, `in_progress`, `completed` are defined in `task.go` constants AND included in `ValidTaskTransitions` map, despite docs marking them as deprecated. | ✅ |
| 3 | Task System (§5.6) | 🟡 Medium | `ValidTaskTransitions` contains **stale/inconsistent transitions:** `todo → assigned/planning/coding` and `analyzing → planning/testing` which bypass the documented lifecycle. | ✅ |
| 4 | Handler Services (services.go) | 🟢 Low | `VirtualKeyService` interface defined at L155 but **never consumed** in `Deps` struct or `NewRouter()`. Dead interface. | ✅ |
| 5 | Agent Model (§5.3) | 🟢 Low | `Agent.ModelRoute` field name is `model_route` but docs consistently use "Model Level Group" terminology. The field represents a level (`fast/balanced/powerful`), not a "route". Semantic mismatch. | ✅ |
| 6 | Provider Model (§5.1) | 🟢 Low | `ComboEntry.Tier` field in `provider_model.go` — redundant with `LevelGroup`. Gateway uses both names inconsistently (`tier` vs `level_group`). | ☐ |
| 7 | Router | 🟢 Low | Version string hardcoded as `"0.2.0"` in health endpoint (L77). Should be injected from build or config. | ☐ |
| 8 | Frontend (gateway.ts) | 🟡 Medium | Virtual Keys API client (`gateway.ts:55-82`) calls `/organizations/{orgID}/virtual-keys/*` endpoints but **these routes don't exist** on the backend. Dead client code will always 404. | ✅ |
| 9 | Frontend (settings) | 🟢 Low | `web/src/app/settings/virtual-keys/page.tsx` exists as a UI page for a feature that is deferred and has no backend routes. | ✅ |
| 10 | Frontend (sidebar) | 🟢 Low | `home-sidebar.tsx` references virtual keys navigation — links to a non-functional page. | ✅ |
| 11 | Migration (SQL) | 🟢 Low | Single monolithic `000001_init.up.sql` (31KB) — makes it hard to track schema evolution. Already noted in prior conversation (f29fd1c7). | ✅ |
| 12 | Orchestrator | 🟢 Low | `orchestrator.go` is 56KB / ~1500+ lines — a god-file that handles analysis, coding, reviewing, testing, PR generation, workspace management, and pruning all in one file. | ☐ |

---

## Detailed Plan — Per Feature

### Task 1: Remove Virtual Keys Dead Code (Issues 1, 4, 8, 9, 10)

> **Feature:** §5.1.F — Virtual Key Architecture (Deferred)
> **Rationale:** Docs explicitly state "Distant Future / Deferred". The code exists at all layers but is completely disconnected (no routes, not wired). This is dead code that adds confusion and maintenance burden.

**Sub-tasks:**
- [x] **1a.** Remove `server/internal/handler/virtual_key.go`
- [x] **1b.** Remove `server/internal/service/virtual_key.go`
- [x] **1c.** Remove `server/internal/repository/virtual_key.go`
- [x] **1d.** Remove `VirtualKeyService` interface from `server/internal/handler/services.go` (L155-161)
- [x] **1e.** Remove virtual key audit constants from `server/pkg/models/audit.go` (L38-42)
- [x] **1f.** Remove `server/pkg/models/virtual_key.go`
- [x] **1g.** Remove `web/src/app/settings/virtual-keys/` directory
- [x] **1h.** Remove virtual key API client functions from `web/src/lib/api/gateway.ts` (L55-82)
- [x] **1i.** Remove virtual key types from `web/src/lib/types.ts` (L368-404)
- [x] **1j.** Remove virtual keys navigation from `web/src/components/dashboard/home/home-sidebar.tsx`
- [x] **1k.** Remove `virtual_keys` table DDL from `000001_init.up.sql` and `000001_init.down.sql`
- [x] **1l.** Verify Go build, vet, and tests still pass

---

### Task 2: Clean Up Deprecated Task Statuses (Issues 2, 3)

> **Feature:** §5.6 — Task System
> **Rationale:** Docs declare `assigned`, `planning`, `in_progress`, `completed` as deprecated. They exist in code as first-class status constants and transition targets, creating ambiguity for anyone reading the state machine.

**Sub-tasks:**
- [x] **2a.** Add `// Deprecated:` doc comments to the 4 deprecated constants in `task.go`
- [x] **2b.** Remove deprecated statuses from `ValidTaskTransitions` map keys and values where they appear as transition targets
- [x] **2c.** Verify the cleaned transition map matches the documented lifecycle: `todo → analyzing → spec_review → coding ⟷ reviewing ⟷ fixing → testing → human_review → merged`
- [x] **2d.** Run `go test ./...` to verify no tests break

> [!IMPORTANT]
> **Decision needed:** Do we _remove_ the deprecated constants entirely (breaking backfill), or just mark them as deprecated and strip from the transitions map?

---

### Task 3: Rename `ModelRoute` → `ModelLevelGroup` (Issue 5)

> **Feature:** §5.3 — Agent System
> **Rationale:** The field `model_route` semantically means "which Model Level Group an agent uses" (fast/balanced/powerful). Docs, DOMAIN.md, and PROFILE.md all use "Model Level Group" terminology, but code uses `ModelRoute`. This creates a doc/code semantic gap.

**Sub-tasks:**
- [x] **3a.** Rename `Agent.ModelRoute` → `Agent.ModelLevelGroup` in `server/pkg/models/agent.go` (struct, create/update inputs)
- [x] **3b.** Update JSON tag from `model_route` to `model_level_group`
- [x] **3c.** Update all service/handler/orchestrator references
- [x] **3d.** Update `AgentStats.ModelRoute` → `ModelLevelGroup` in `analytics.go`
- [x] **3e.** Update frontend `Agent.model_route` → `model_level_group` in `types.ts` and all component references
- [x] **3f.** Add SQL migration / update migration files directly
- [x] **3g.** Verify Go build + TypeScript compile

> [!WARNING]
> This is a **breaking API change**. Frontend and any API consumers must update simultaneously.

---

### Task 4: Normalize Tier vs LevelGroup Naming (Issue 6)

> **Feature:** §5.1 — Unified AI Gateway
> **Rationale:** The codebase uses `tier` and `level_group` interchangeably. `ComboEntry.Tier`, `TokenUsage.Tier`, LLM package constants (`llm.TierFast`), and gateway code all use "Tier" while the docs and model tables use "level_group". This should be unified.

**Sub-tasks:**
- [x] **4a.** Audit all occurrences of `tier` vs `level_group` in the Go codebase
- [x] **4b.** Decide on a single canonical term (recommendation: `level_group` to match DB/docs)
- [x] **4c.** Rename `ComboEntry.Tier` → `ComboEntry.LevelGroup`
- [x] **4d.** Rename `llm.TierFast/TierBalanced/TierPowerful` → `llm.LevelFast/LevelBalanced/LevelPowerful`
- [x] **4e.** Update `TokenUsage.Tier` → `TokenUsage.LevelGroup` (requires SQL migration for `token_usage.tier` column)
- [x] **4f.** Update frontend `TokenUsageSummary.tier` in `types.ts`

> [!NOTE]
> This can be deferred since it's naming consistency, not a functional bug. Prioritize Tasks 1 & 2 first.

---

### Task 5: Inject Build Version Into Health Endpoint (Issue 7)

> **Feature:** Router / DevOps
> **Rationale:** Hardcoded `"0.2.0"` at L77 of `router.go` will become stale. Best practice: inject via `go build -ldflags`.

**Sub-tasks:**
- [x] **5a.** Add `var Version = "dev"` in `main.go` or a `version` package
- [x] **5b.** Pass version to `handler.Deps` → use in health endpoint
- [x] **5c.** Update `Makefile` to inject version at build time via `-ldflags`

---

### Task 6: Orchestrator God-File Refactoring (Issue 12)

> **Feature:** §5.7 — Workflow Engine
> **Rationale:** `orchestrator.go` (56KB, ~1500 lines) handles 8+ distinct responsibilities. Refactoring into focused files improves readability and testability.

**Sub-tasks:**
- [x] **6a.** Extract workspace management (prune, cleanup) → `orchestrator_workspace.go`
- [x] **6b.** Extract workflow execution steps (analyze, code, review, fix, test) → `orchestrator_steps.go`
- [x] **6c.** Extract PR generation logic → already partly in `pr_generator.go`, verify no duplication
- [x] **6d.** Extract worker/queue logic → `orchestrator_worker.go`
- [x] **6e.** Keep `orchestrator.go` as the central struct + constructor + top-level dispatch only
- [x] **6f.** Verify all tests still pass

> [!TIP]
> This is a **refactoring-only** task — no behavioral changes, just file splitting. Safe to do as a separate PR.

---

## Priority Matrix

| Priority | Task | Effort | Impact |
|:---------|:-----|:-------|:-------|
| 🔴 P0 | Task 1 — Remove Virtual Keys dead code | Small | High — removes confusion, reduces surface area |
| 🔴 P0 | Task 2 — Clean deprecated task statuses | Small | High — critical state machine correctness |
| 🟡 P1 | Task 3 — Rename ModelRoute → ModelLevelGroup | Medium | Medium — doc/code alignment |
| 🟡 P1 | Task 5 — Inject build version | Small | Low — quick DevOps improvement |
| 🟢 P2 | Task 4 — Normalize tier/level_group naming | Medium | Low — consistency |
| 🟢 P2 | Task 6 — Orchestrator refactoring | Large | Medium — maintainability |

---

## What's Working Well ✅

- **Clean architecture:** Handler → Service → Repository layer separation is consistent across all features.
- **Gateway design is solid:** Multi-key pool rotation, cooldown/recovery, priority-based routing, telemetry recording with defer — all well-implemented.
- **Proper interface segregation:** Handler layer depends on service interfaces, not concrete types.
- **AES-256-GCM encryption** for provider credentials matches the invariant.
- **Audit logging** is comprehensive and covers credential lifecycle.
- **Test coverage** exists for key packages: gateway, orchestrator, handler, workflow, service, config, gitops.
- **Frontend types** are well-synchronized with Go models.
- **Graceful shutdown** with signal handling, worker cancellation, and wait groups.
