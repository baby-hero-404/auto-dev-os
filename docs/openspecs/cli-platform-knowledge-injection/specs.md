# Specs: CLI Platform Knowledge Injection (v2 — Context Materializer)

## Added Requirements

### REQ-001: `MaterializeCLIContext` builds Layer 1 (always) + Layer 2 (relevant) files
> ❌ Status: Not Started

**Scenario:**
- WHEN `MaterializeCLIContext(ctx, task, agent, stepID)` is called with a valid task that has a `ProjectID`
- THEN it always returns `manifest.json` (task id/type + counts of available skills/learned_skills/rules + `sources` list) and `README.md` (short pointer text)
- AND it calls the existing `resolveSkills(ctx, task, agent, stepID)` to select up to `maxJITSkills` (5) most relevant skills, rendering each as `relevant/skills/<name>.md` with full `ParsedSkill.Content`
- AND if `learned_skills` context exists for the project, it is rendered as `relevant/learned_skills.md`
- AND if `task.Analysis.TaskRules` is non-empty, it is rendered as `relevant/task_rules.md`
- AND the combined Layer 2 output respects a character budget (~12000 chars); if exceeded, lowest-scored skills are dropped first, `manifest.json` always reflects what was actually included

### REQ-002: `MaterializeCLIContext` degrades gracefully when nothing is available
> ❌ Status: Not Started

**Scenario:**
- WHEN `MaterializeCLIContext` is called and no skills, no learned_skills, and no task_rules exist
- THEN it returns an empty map (not an error)
- AND CLI steps proceed without adding any context pointer to the instruction (no regression, no dangling reference to a non-existent directory)

### REQ-003: `RunCodeStep` materializes `ContextFiles` into `.autocode/context/` and exposes `AUTOCODE_CONTEXT_DIR`
> ❌ Status: Not Started

**Scenario:**
- WHEN `engine.CodeStepRequest.ContextFiles` is non-empty
- THEN `RunCodeStep` writes each entry to `<hostAutocodeDir>/context/<relative-path>` (creating parent directories as needed) before spawning the CLI subprocess
- AND the container environment includes `AUTOCODE_CONTEXT_DIR=<containerWorkDir>/.autocode/context`
- AND the existing `.autocode/` cleanup (in-script `rm -rf` + host-side `defer os.RemoveAll`) removes `context/` along with `prompt.md` — no new lifecycle mechanism, no dedicated cleanup path needed
- AND when `ContextFiles` is empty/nil, behavior is byte-for-byte identical to today (no `context/` dir created, no env var set)

### REQ-004: CLI steps append a thin context pointer instead of embedding content
> ❌ Status: Not Started

**Scenario:**
- WHEN `cli_analyze` / `cli_spec` / `cli_implement` executes and `MaterializeCLIContext` returned a non-empty map
- THEN the instruction gets exactly one short paragraph appended (not skill names, not skill content) telling the agent that platform context exists at `$AUTOCODE_CONTEXT_DIR` and it should inspect it if relevant
- AND when the map is empty, no such paragraph is appended (instruction identical to current behavior)
- AND the original step prompt template and task title/description sections remain unchanged

### REQ-005: Test mock compatibility across all CLI step tests
> ❌ Status: Not Started

**Scenario:**
- WHEN any of `mockStepPromptLoader` (in `cli_analyze_test.go`, `cli_spec_test.go`, `cli_implement_test.go`, `cli_spec_first_integration_test.go`) or the CLI step runner mock in `internal/orchestrator/engine/cli_test.go` is used
- THEN each implements the extended interface (`MaterializeCLIContext` on the prompt loader mock; the extra `contextFiles` parameter on the CLI step runner mock)
- AND all existing tests pass without modification to their assertions (mocks return empty/nil by default)

## Modified Requirements
- (none — this fully replaces the unmerged v1/v1.5 requirement sets below)

## Removed Requirements
- v1 REQ-001..004 ("LoadCLIContext renders Skills page skills as markdown", full-text dump approach) — replaced by REQ-001..003 above (materialize-to-file approach).
- v1.5 REQ-001..003 ("CLI Engine mounts project skills into container workspace" via whole-directory deep-copy) — replaced by the relevant-only Layer 2 materialization in REQ-001, which reuses `resolveSkills()` scoring instead of copying everything indiscriminately.
