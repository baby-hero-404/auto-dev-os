# Proposal: Full Codebase Cleanup Scan

## Problem
Cleanup so far has been module-scoped and reactive: `orchestrator-dead-code-cleanup`, `service-repository-dead-code-cleanup`, `infra-tooling-dead-code-cleanup`, `handler-api-cleanup`, `web-ui-cleanup-enhance` each cleared one corner of the repo when someone happened to be working there. Everything outside those corners — `server/pkg/*`, and the `server/internal/{context,database,gateway,gitops,governance,middleware,observability,policy,prompts,tool,workflow}` directories — has never had a dedicated pass, and dead code that spans a module boundary (e.g. a helper only the now-deleted orchestrator code called) can survive any single-module cleanup untouched. There is also no standing process for catching this on an ongoing basis — each prior spec was a one-off.

## Goal
A repeatable, whole-project scan that finds unused/outdated code and duplicate logic, flags overly complex implementations for refactor, and removes what can be safely proven unused — without changing observable behavior anywhere it touches.

## Success
- Every subsystem listed in Impact has had at least one detection pass, not just the ones already covered by prior specs.
- Everything removed has documented evidence (zero static callers + grep confirmation) in the final report; everything ambiguous is listed as "potentially unused," not deleted.
- `go build ./... && go test ./...` (server) and the web build/lint/test suite pass unchanged after every subsystem's changes land.
- At least the top complexity/duplication hotspots surfaced by the scan have either been refactored or have a tracked follow-up.

## Decisions
- **Detection tool is the tokensave graph, not ad-hoc grep.** This repo already has a synced code graph (`tokensave_dead_code`, `tokensave_unused_imports`, `tokensave_circular`, `tokensave_redundancy`/`tokensave_similar`, `tokensave_complexity`/`tokensave_god_class`/`tokensave_hotspots`). Using it instead of hand-rolled searches gives consistent, reviewable evidence per candidate and is what the rest of this spec's scenarios assume.
- **Compiler/linter output is the backstop, not the primary signal.** `go vet`, `staticcheck` (if configured) and the web project's `tsc --noEmit`/`eslint` unused-var rules catch what the heuristic graph might miss (or over-claim) — every deletion still needs one of these tools' agreement before it's safe, not the graph alone.
- **Split into one pass per subsystem, mirroring the prior single-module specs' shape**, rather than one repo-wide PR. Smaller PRs bisect a regression to one subsystem; a single giant diff would be unreviewable and would block on the slowest part.
- **Docs/openspecs pass runs last.** Stale-plan detection (finished specs whose `tasks.md` is fully checked and whose proposal has since been superseded, or specs describing code that a code-removal phase above just deleted) is only accurate once the code-side passes have already landed.
- **Deletion requires proof; ambiguity is reported, not resolved by deleting.** Anything reached only via reflection, JSON-tag-driven (de)serialization, a registry map keyed by string (e.g. `models.CLIProfiles`), an exported public API, or a CLI/workflow step-name string lookup gets marked "potentially unused" instead of removed, per the codebase-cleanup skill's safety rule.
- **Complexity findings get an extraction refactor, not a rewrite.** Behavior stays identical; the change is purely structural (split a function, name things better, remove duplication) and must be covered by existing tests before the refactor lands, adding tests first if coverage is missing.

## Trade-offs
- Slower to land than one targeted module pass, but catches cross-module dead code prior specs structurally couldn't see.
- More PRs to review, in exchange for each one being small enough to actually review and easy to revert independently if a regression surfaces.

## Out of Scope
- No business logic changes, no API/wire-contract changes (a contract only gets touched if the entire contract itself is proven dead).
- No schema migrations, no dependency version bumps.
- No changes inside third-party `references/` directories (noise identified during prior CLI tracing work, not application code).
- No re-architecture — this is cleanup and safe refactor, not a redesign.
- Does not re-scope or reopen any prior cleanup spec's already-completed work.

## Impact
- `server/pkg/{attest,config,llm,models,paths}` — no prior dedicated cleanup pass.
- `server/internal/{context,database,gateway,gitops,governance,middleware,observability,policy,prompts,tool,workflow}` — no prior dedicated cleanup pass.
- `web/src/{app,components,lib}` — last touched by `web-ui-cleanup-enhance`; this pass covers anything that spec didn't.
- `docs/openspecs/*`, `docs/plans/*` — stale spec/plan pruning, run last.
