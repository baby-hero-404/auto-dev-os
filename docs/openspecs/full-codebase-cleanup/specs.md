# Expected Behavior: Full Codebase Cleanup Scan

## Scenario: Unused symbol detection
**When:**
- A subsystem pass runs `tokensave_dead_code`, `tokensave_unused_imports`, and `tokensave_circular` against its directory scope.

**Then:**
- Every candidate is cross-checked with a plain `grep -rn` for the symbol name and, for Go, `go vet ./...` (plus `staticcheck` if the repo has it configured) before removal — a graph miss (e.g. reflection-based call) must not become a deletion.
- Only candidates confirmed unused by both the graph and the backstop tool are removed in the same commit; anything the two disagree on is left in place and listed as "potentially unused."

## Scenario: Ambiguous / dynamically-referenced code
**When:**
- A symbol has zero static callers but is exported, registered in a string-keyed map (e.g. `models.CLIProfiles`, a workflow step registry), or matched by JSON tag during (de)serialization.

**Then:**
- It is left in the codebase and listed under "potentially unused" in the final report with the reason it wasn't removed — never deleted on the strength of the graph alone.

## Scenario: Database-mapped struct or field looks unused
**When:**
- `tokensave_dead_code` (or a zero-caller check) flags a struct or field in `server/pkg/models` (or any ORM-mapped type elsewhere) that carries a `gorm:`, `db:`, `sql:`, or `bson:` tag.

**Then:**
- It is never deleted in this pass, regardless of how confident the graph is. A field with no Go-side reader can still be a live, populated database column — removing the struct field desyncs the ORM mapping from the schema and produces a runtime error (scan mismatch, missing column) rather than a compile error, so the usual backstop tools (`go vet`, tests) will not catch it.
- It is listed under "potentially unused" with a note that removing it requires a paired schema migration, which is explicitly out of scope for this cleanup pass (see proposal.md Out of Scope).

## Scenario: Framework & Configuration Reflection Boundaries
**When:**
- `tokensave_dead_code` (or a zero-caller check) flags a struct or field in `server/pkg/config` (or any config loader) carrying `yaml:`, `mapstructure:`, `env:`, or `config:` tags, OR an export in Next.js App Router (`web/src/app/**/{page,layout,loading,error,route,not-found}.{tsx,ts}`).

**Then:**
- They are treated as framework/reflection-bound and NEVER deleted. Page and route exports in Next.js App Router are invoked dynamically by directory convention, and config fields are populated dynamically via reflection from environment/YAML files.

## Scenario: Implicit Interface Satisfaction (Go Duck Typing)
**When:**
- A struct method is flagged as having zero direct callers in the Go graph.

**Then:**
- Before marking it safe to remove, verify whether removing it breaks any internal interface satisfaction (check interface declarations across the package or imported packages). A method implementing an interface method must be preserved.

## Scenario: Cross-boundary API usage (backend handler/payload vs. frontend)
**When:**
- `tokensave_dead_code` or `tokensave_hotspots` flags an exported Go symbol in `server/internal/handler` (or any HTTP-handler/response-payload type) as having few or zero incoming edges within the Go graph.

**Then:**
- Before treating it as unused, grep `web/src` for the route path string and for the JSON field names of the payload type — an HTTP round trip is invisible to a same-language call graph, so a handler with zero Go callers can still be the live target of every request the frontend sends.
- A handler/payload is only eligible for removal if it has zero Go-side callers **and** no matching route/fetch/field reference anywhere in `web/src`, **and** the route itself is confirmed removed from `router.go`. Any one of those three failing keeps it out of the "safe to remove" bucket.

## Scenario: Duplicate logic
**When:**
- `tokensave_redundancy` / `tokensave_similar` surfaces near-identical logic across two or more files in the same subsystem.

**Then:**
- The duplication is consolidated into one shared helper using composition (a shared function/struct the callers invoke), not inheritance — matching the pattern already established by `cli-code-review-refactor`'s `wsTerminalTicketStore` and `finishCLIRun` extractions.
- Each caller's observable behavior (inputs/outputs, error strings, wire format) stays identical; this is verified by running that subsystem's existing tests before and after.

## Scenario: Complex implementation flagged for refactor
**When:**
- `tokensave_complexity`, `tokensave_god_class`, or `tokensave_hotspots` flags a function or file above the tool's default threshold.

**Then:**
- If the flagged code has existing test coverage, it is refactored (split into smaller functions, nesting reduced, naming clarified) with no behavior change, and the existing tests must still pass unmodified.
- If it has no test coverage, coverage is added first (success + failure path) before any structural change — refactoring uncovered complex code is not permitted in this pass.
- If a flagged function's complexity is inherent to the problem (e.g. a genuinely branchy state machine) rather than accidental, it is left alone and logged as a reviewed-and-accepted hotspot rather than force-split.

## Scenario: Stale docs/openspecs pruning (final phase only)
**When:**
- The docs/openspecs pass runs, after every code-side subsystem phase has landed.

**Then:**
- A spec folder is only removed or archived if its `tasks.md` is fully checked off **and** the code it describes still exists as described, or has been superseded by a later spec that supersedes it explicitly.
- A spec describing code that a code-removal phase above just deleted is flagged for the user, not silently deleted — removing the doc without confirming the deletion was intentional could hide a real regression.

## Rules
- No business logic changes anywhere in this pass; no API/wire-contract changes unless the entire contract is proven dead.
- Every removal in the final report cites its evidence: which tool flagged it, and the grep/backstop-tool confirmation.
- The full existing test suite (`go test ./...` for server; the web project's build/lint/test) must pass, unchanged, after every subsystem's phase — not just at the very end.
- One PR per subsystem phase (see tasks.md ordering), never one repo-wide diff.
- Never delete a struct/field carrying a `gorm:`/`db:`/`sql:`/`bson:` tag without an accompanying schema migration — always route it to "potentially unused" instead (see Database-mapped struct scenario above).
- Never treat an exported handler function or HTTP response/request payload type as dead on Go-graph evidence alone — the frontend cross-check in the Cross-boundary API scenario above is mandatory for anything under `server/internal/handler` or any `*Request`/`*Response` payload type referenced by it.

## Constraints
- Excludes third-party `references/` directories, `node_modules`, and any vendor/generated code.
- Before touching a file, check whether any other `docs/openspecs/*/tasks.md` still has unchecked boxes referencing that file — if so, skip it in this pass and note the conflict instead of racing the other spec's work.
- No dependency upgrades, no schema migrations, no re-architecture — structural refactors only, and only where a test can prove behavior held.
