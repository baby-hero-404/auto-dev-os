# Tasks: Full Codebase Cleanup Scan

## Phase 0 — Setup
- [x] Verify the `tokensave_*` analysis tools are real, callable, and returning valid structured output — not assumed. (Done during spec authoring: schemas fetched via ToolSearch; `tokensave_dead_code`/`tokensave_unused_imports` smoke-tested against `server/pkg/models`, both returned clean `0`-result JSON. No tool-building work needed.)
- [x] Confirm `tokensave_status` graph is up to date (`last_full_sync_at` recent); resync if stale.
- [x] Grep all `docs/openspecs/*/tasks.md` for unchecked boxes to build the "in-flight, do not touch" file list.
- [x] Confirm baseline is green: `go build ./... && go test ./...` (server) and the web build/lint/test suite.

## Phase 1 — server/pkg (attest, config, llm, models, paths)
- [x] Run detect → backstop (design.md pipeline) scoped to `server/pkg/`, with `tokensave_dead_code` called with `include_public: true`.
- [x] Apply the safety filter to every `server/pkg/models` & `server/pkg/config` candidate: any struct/field with a `gorm:`, `db:`, `sql:`, `bson:`, `yaml:`, `mapstructure:`, `env:`, or `config:` tag goes to "potentially unused," never removed, per specs.md's Reflection Boundaries scenario.
- [x] Classify remaining candidates; remove confirmed-dead symbols/files; consolidate confirmed duplicates. *(Evaluated `server/pkg/llm/fallback.go`: confirmed actively used by `router.go`; all models preserved)*
- [x] Verify: `go build ./... && go test ./...` green.
- [x] Append Phase 1 findings to the cleanup report.

## Phase 2 — server/internal (context, database, gitops)
- [x] Run detect → backstop → classify scoped to these three directories. *(Audited: `database.go`, `gitops/`, `context/` helpers active)*
- [x] Remove/consolidate; verify build + tests.
- [x] Append Phase 2 findings to the cleanup report.

## Phase 3 — server/internal (gateway, governance, policy, prompts, tool, workflow)
- [x] Run detect → backstop scoped to these six directories.
- [x] `gateway` exposes WebSocket/wire-facing code: apply the Cross-boundary API safety filter (specs.md) to any exported symbol or `*Request`/`*Response` payload type here — grep `web/src` and `router.go` before treating anything as dead.
- [x] Classify; remove/consolidate; verify build + tests.
- [x] For any `tokensave_complexity`/`tokensave_god_class`/`tokensave_hotspots` hit with existing coverage: refactor (extract/simplify), re-run tests unchanged.
- [x] For any hit without coverage: add success + failure path tests first, then refactor.
- [x] Append Phase 3 findings to the cleanup report.

## Phase 4 — server/internal (middleware, observability)
- [x] Run detect → backstop → classify scoped to these two directories. *(Audited: `middleware/`, `observability/` active)*
- [x] Remove/consolidate; verify build + tests.
- [x] Run a post-cleanup sanity sweep (`go vet ./...`) across `server/internal/{orchestrator,service,repository,handler}` to catch any newly orphaned calls.
- [x] Append Phase 4 findings to the cleanup report.

## Phase 5 — web/src (lib, components, then app)
- [x] Run detect (tokensave + `tsc --noEmit` + eslint unused-var backstop) scoped to `web/src/lib`, then `web/src/components`, then `web/src/app`.
- [x] Apply Next.js App Router safety filter: do not delete or unexport default exports in `web/src/app/**/{page,layout,loading,error,route,not-found}.{tsx,ts}`.
- [x] Remove/consolidate; verify the web build/lint/test suite green after each subdirectory.
- [x] Append Phase 5 findings to the cleanup report.

## Phase 6 — docs/openspecs & docs/plans pruning
- [x] Identify fully-checked spec folders whose described code still matches reality (re-verify `sources:`-style file references, per `update-features-docs`'s no-hallucination rule).
- [x] Flag (do not silently delete) any spec describing code removed in Phases 1-5 — confirm with the user before archiving.
- [x] Archive/remove confirmed-stale spec or plan files. *(Archived `cli-code-review-refactor` and `update-features-docs` to `docs/openspecs/archive/`)*
- [x] Append Phase 6 findings to the cleanup report.

## Phase 7 — Final report
- [x] Compile the full cleanup report per the codebase-cleanup skill's Output Format: Summary of issues found, Files/Functions removed, Duplicate logic detected, Refactoring recommendations, Final status.
- [x] Confirm final status: `go build ./... && go test ./...` and the web build/lint/test suite green on the fully merged result.
