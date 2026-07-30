# Design: Full Codebase Cleanup Scan

## Tooling verified, not assumed
Every `tokensave_*` tool this design names was checked against the live MCP server before this design was finalized (schema fetched via ToolSearch, then smoke-tested with `tokensave_dead_code` and `tokensave_unused_imports` scoped to `server/pkg/models`, both returning a clean structured `0`-result response). These are real, callable tools against an already-synced graph — Phase 0 below is a scope-correctness check (is the index current, is the "in-flight spec" exclusion list built), not a "does this tool exist" check.

One real tool-behavior detail this design accounts for: `tokensave_dead_code` **excludes exported (`pub`/capitalized) symbols by default** — for a private application repo (not a published library) that default hides most of what actually matters, so every invocation in this pass must pass `include_public: true`. That in turn means more false positives from Go's interface-satisfaction and reflection patterns reach the candidate list, which is exactly what the Backstop step and the two dedicated safety scenarios in specs.md (DB-mapped fields, cross-boundary API) exist to filter back out before anything is deleted.

## Per-subsystem pipeline
Every phase in `tasks.md` runs the same six steps against its directory scope before moving to the next phase:

1. **Detect** — `tokensave_dead_code` (`include_public: true`), `tokensave_unused_imports`, `tokensave_circular`, `tokensave_redundancy` (or `tokensave_similar` for near-duplicates), `tokensave_complexity` + `tokensave_god_class` + `tokensave_hotspots`, scoped via `path`/`path_include`/`path_exclude` to the phase's directory and excluding `references/`, `node_modules`, vendor/generated code.
2. **Backstop** — for each dead-code/unused-import candidate: `grep -rn <symbol>` across the repo, plus `go vet ./...` for Go subsystems or `tsc --noEmit` / the configured `eslint` unused-var rule for `web/src`. A candidate only proceeds to removal if the graph and the backstop tool agree.
3. **Safety filter** — before classifying anything from `server/pkg/models` (or any ORM-mapped type) as safe-to-remove, check for `gorm:`/`db:`/`sql:`/`bson:` struct tags (see specs.md's Database-mapped struct scenario). Before classifying anything exported from `server/internal/handler` or a `*Request`/`*Response` payload type, grep `web/src` and `router.go` for it (see specs.md's Cross-boundary API scenario). Either hit forces the candidate into "potentially unused," full stop.
4. **Classify** — every surviving candidate is bucketed as: safe to remove (zero references anywhere, not exported/dynamically dispatched, cleared the safety filter), potentially unused (ambiguous — reflection, registry map, JSON tag, public API, DB mapping, cross-boundary API), or duplicate logic (a redundancy hit, handled via consolidation instead of deletion).
5. **Apply** — remove the "safe to remove" bucket; consolidate the "duplicate logic" bucket into one shared helper (composition, following the `wsTerminalTicketStore`/`finishCLIRun` precedent from `cli-code-review-refactor`); leave "potentially unused" untouched.
6. **Verify** — run the phase's full existing test suite (and `go build ./...` / the web build, as applicable) unchanged before and after; a failing or newly-broken test blocks that phase, it does not get skipped or the test relaxed.
7. **Report** — append the phase's findings to the running cleanup report using the codebase-cleanup skill's Output Format (Summary / Removed / Duplicates / Recommendations / Final status), with each removal citing its tool + backstop + safety-filter evidence.

## Phase ordering
Ordered by risk and dependency, not by directory alphabetization:

1. `server/pkg/{attest,config,llm,models,paths}` — leaf packages, few internal dependents relative to `server/internal`, lowest blast radius to start with and validate the pipeline itself.
2. `server/internal/{context,database,gitops,paths-adjacent utilities}` — foundational but still largely leaf-consumed.
3. `server/internal/{gateway,governance,policy,prompts,tool,workflow}` — more heavily cross-referenced; run after the leaf packages above are already clean so their own dead-code graph isn't muddied by call sites this phase is about to remove.
4. `server/internal/{middleware,observability}` — cross-cutting but narrow in surface area (request lifecycle, logging/metrics).
5. `web/src/{lib,components}` then `web/src/app` — library/shared code before route-level code, same leaf-first logic as the server phases.
6. `docs/openspecs/*`, `docs/plans/*` — last, once the code-side phases have already landed, so stale-doc detection reflects the actual post-cleanup codebase rather than a moving target.

## Why tokensave over pure grep
The repo already carries a synced call/reference graph (`tokensave_status` reports ~1M nodes / ~1.1M edges as of this scan). `tokensave_dead_code` and friends give a ranked, evidence-backed candidate list instead of a human manually grepping every symbol in five languages' worth of code — but the graph is a heuristic index, not a compiler, so step 2 (Backstop) exists specifically to catch the cases a graph analysis is known to miss (interface satisfaction, reflection, string-keyed registries) before anything is deleted.

## Rollback posture
Each phase is its own PR against a clean state (tests green before starting). If a phase's removal turns out to have broken something not caught by step 5, reverting that one PR fully undoes it without touching any other phase's work — this is the reason phases aren't squashed into one diff.
