# Tasks: CLI Execution Provider Routing

## P0 — Backend foundation (blocks everything else)

### Task 1.1: CLI Profile registry
- [x] Create `server/pkg/models/cli_profiles.go` — `CLIProfile` struct, `CLIProfiles` map (`claude_code`/`openai_codex`/`antigravity`), `ProfileOrEmpty()`.
- [x] `server/pkg/models/cli_profiles_test.go`:
  ```go
  func TestProfileOrEmpty_KnownKey(t *testing.T) {
      p, ok := models.ProfileOrEmpty("claude_code")
      require.True(t, ok)
      require.Equal(t, "claude", p.Command)
  }
  func TestProfileOrEmpty_UnknownKey(t *testing.T) {
      _, ok := models.ProfileOrEmpty("not_real")
      require.False(t, ok)
  }
  ```
- Satisfies: REQ-001

### Task 1.2: ⚠️ Verify Codex/Antigravity command+args against real CLIs
- [x] Run `codex --help` / `codex exec --help` locally (or check `docs/references/` if a Codex CLI report exists), confirm `Args` in design.md match actual non-interactive invocation flags.
- [x] Same for `antigravity` CLI.
- [x] Update `CLIProfiles` values in Task 1.1 if design.md's placeholder args are wrong — **do not merge Task 1.1 as final until this is done**, only `claude_code` was manually verified in this session.
- Satisfies: REQ-001 (accuracy)

### Task 1.3: `Project.ExecutionProviders` field + validation
- [x] Add `ExecutionProviderConfig` struct + `ExecutionProviders json.RawMessage` field to `server/pkg/models/project.go`.
- [x] Implement `ValidateExecutionProviders(raw json.RawMessage) ([]ExecutionProviderConfig, error)` per design.md.
- [x] `server/pkg/models/project_test.go` additions:
  - valid api + cli mix round-trips
  - invalid `type` rejected
  - `ref=="custom"` without `cli_config.command` rejected
  - unknown cli `ref` rejected
  - unknown api `ref` rejected (via `IsAllowedProvider`)
- Satisfies: REQ-002

### Task 1.4: Migration — `execution_providers` column
- [x] Add `server/migration/<ts>_add_execution_providers_to_projects.sql`: `ALTER TABLE projects ADD COLUMN execution_providers jsonb NOT NULL DEFAULT '[]';`
- [x] Run migration locally, confirm `\d projects` shows the column with correct default.
- [x] No backfill script — verify empty-array default satisfies REQ-003 (existing rows get `[]` automatically via `DEFAULT`).
- Satisfies: REQ-002, REQ-003

### Task 1.5: Wire `execution_providers` through create/update project service + handler
- [x] `server/internal/service/project.go`: call `ValidateExecutionProviders` in `validateEngineInput` (or equivalent), return 400 on error with the field-level message from Task 1.3.
- [x] `server/internal/handler/*` (project create/update handlers): pass `execution_providers` through request/response body — additive field, no breaking change to existing payload shape.
- [x] Test: `PUT /projects/:id` with invalid `execution_providers[0].type` returns 400 with the exact validation message (REQ-002 scenario).
- Satisfies: REQ-002

## P1 — Execution Router (core of this OpenSpec)

### Task 2.1: `ResolveExecutionProvider`
- [x] Create `server/internal/orchestrator/execution_router.go` per design.md: `ResolvedExecutionProvider` struct, `ResolveExecutionProvider(ctx, task, project)`.
- [x] `hasAvailableCredential(ctx, orgID, provider, pinnedCredID string) bool` — reuse `ProviderCredentialRepo`, exclude `status=="rate_limited"` / `CooldownUntil` in future.
- [x] `resolveCLICandidate(ctx, orgID string, p ExecutionProviderConfig) (*models.CLIEngineConfig, credID string, ok bool)` — for `ref!="custom"`, build `CLIEngineConfig` from `CLIProfiles[ref]`; for `ref=="custom"`, use `p.CLIConfig` directly.
- [x] `legacyResolve(ctx, task, project)` — thin wrapper around existing `ResolveEngine` + old `CLIEngineConfig` read, used when `ExecutionProviders` is empty. Must reuse the existing code path verbatim (call into it), not reimplement.
- Satisfies: REQ-003, REQ-004, REQ-006

### Task 2.2: Unit tests for the Router
- [x] `server/internal/orchestrator/execution_router_test.go`:
  - empty `execution_providers` → falls back to legacy behavior, matches a project with `execution_engine="cli"` + old config (REQ-003)
  - priority-1 CLI candidate active → returned, priority-2 never checked (REQ-004)
  - priority-1 candidate credential rate-limited → priority-2 returned (REQ-004)
  - `enabled:false` candidate skipped even if credential is healthy (REQ-004)
  - all candidates unavailable → error `"no enabled execution provider is available"` (REQ-004)
  - CLI credential `status=="rate_limited"` makes that candidate unavailable, same as API (REQ-006)
- Satisfies: REQ-003, REQ-004, REQ-005 (no mid-task re-resolution — assert Router is called once per task start, not per step), REQ-006

### Task 2.3: Rewire `resolveCLIEngineRunner`
- [x] Update `server/internal/orchestrator/cli_engine_step.go:143` to call `ResolveExecutionProvider` and build the runner from `resolved.CLIConfig`, returning `nil` when `resolved.Type != "cli"` (preserve existing contract for `step_registry.go`/`worker.go` callers — no changes needed there).
- [x] `newCLIEngineRunner` gains `credID string` (from `resolved.CredentialID`) — threaded through so Task 2.4's cooldown call knows which credential to cool down.
- [x] Existing tests in `cli_engine_step_test.go` (if any reference the old direct `CLIEngineConfig` read) updated to go through the new path; add a case with `execution_providers` populated.
- Satisfies: REQ-M01

### Task 2.4: CLI quota detection — write-side of REQ-006
- [x] Create `server/internal/orchestrator/engine/cli_quota.go`: `CLIQuotaRule`, `CLIQuotaRules` map (`claude_code`/`openai_codex`/`antigravity`/`"*"` fallback), `detectQuotaExceeded(ref, combined string, exitCode int) bool` per design.md.
- [x] `server/internal/orchestrator/engine/cli_quota_test.go`: table test per profile ref — known quota phrases match, unrelated failure text (e.g. compile error) does not match, unknown ref falls back to `"*"` rules.
- [x] Add `ProfileRef string` to `CLIEngineConfig` (or a request-scoped field on `CodeStepRequest.CLIConfig`) and `QuotaExceeded bool` to `CodeStepResult`; wire `detectQuotaExceeded` into `RunCodeStep` right after `detectLoop(combined)`.
- [x] `resolveCLICandidate` (Task 2.1) sets `ProfileRef` when building `CLIEngineConfig` from `CLIProfiles[ref]` or from the legacy fallback (`""`/`"custom"` → `"*"` rules).
- [x] New `CooldownSetter` interface (`SetCooldown(ctx, id, model string, until time.Time) error`) + `Orchestrator.WithCooldownSetter(s CooldownSetter) Option`, wired at construction to the existing `*service.CredentialPoolService`.
- [x] `cliEngineRunner.RunLLMStep`: after `RunCodeStep` returns, if `res.QuotaExceeded && r.credID != ""`, call `r.cooldown.SetCooldown(ctx, r.credID, "", time.Now().Add(cliCooldownDuration))` (`cliCooldownDuration = 1 * time.Minute` constant) — does not change the step's existing failure return.
- [x] Test in `cli_engine_step_test.go`: fake `RunCodeStep` result with `QuotaExceeded=true` → assert `CooldownSetter.SetCooldown` called with the right `credID`; `QuotaExceeded=false` → assert not called.
- Satisfies: REQ-006 (write-side scenarios)

## P2 — Frontend

### Task 3.1: `web/src/lib/cliProfiles.ts` registry
- [x] Mirror `CLIProfiles` map (labels/icons only — command/args not needed client-side since the form no longer shows them for known profiles).
- Satisfies: REQ-001 (frontend mirror), REQ-007

### Task 3.2: `execution-providers-list.tsx` component
- [x] New component: list of `Claude API / OpenAI API / Gemini API / Claude Code / OpenAI Codex / Antigravity / Custom CLI` rows, each with Enabled checkbox + Priority ▲/▼ — reuse `ModelRoutingRules.tsx`'s existing row/button styling, don't invent new UI.
- [x] "Custom CLI" row expands to the existing raw form (`command`/`args`/`auth_check`/`env`/`timeout`) — this is literally today's `CLIEngineConfigForm`, kept as-is, just gated behind selecting "Custom CLI".
- [x] "CLI Authentication Profile" credential dropdown per design.md: **required** on the "Custom CLI" row (client-side validation blocks save if empty), **optional** ("Auto (first available)" default) on the 3 known-profile rows, pre-filtered client-side by `provider.startsWith("cli:")` (reuse the credential-fetching logic already in `cli-engine-config-form.tsx`) and further filtered to the profile's `CredentialProvider` for preset rows. Writes `ExecutionProviderConfig.credential_id`.
- [x] `cliConfigToFormValue`/`formValueToCLIConfig`-equivalent for the new `ExecutionProviderConfig[]` shape.
- Satisfies: REQ-007

### Task 3.3: Wire into `project-profile.tsx`
- [x] Replace `CLIEngineConfigForm` usage with `execution-providers-list.tsx`.
- [x] `web/src/lib/types.ts`: add `ExecutionProviderConfig`, `execution_providers?` to the `Project` type.
- Satisfies: REQ-007

### Task 3.4: `ModelRoutingRules.tsx` — remove `cli:` filter
- [x] Delete the `!provider.startsWith("cli:")` condition in the provider tab filter.
- [x] Manually verify in browser: `cli:claude`/`cli:codex`/`cli:antigravity` tabs appear and priority/active toggles work identically to API provider tabs.
- Satisfies: REQ-008

## P3 — Docs & cleanup

### Task 4.1: Docs sync
- [x] Update `docs/features/` entry for Project Settings to describe "Execution Providers" instead of "CLI Engine Config".
- [x] Note in the doc that `execution_engine`/`cli_engine_config` fields remain as a legacy fallback (link to design.md Trade-offs section) — don't imply they're removed.

## Self-review checklist (map every REQ to a task)

| REQ | Task |
|---|---|
| REQ-001 | 1.1, 1.2, 3.1 |
| REQ-002 | 1.3, 1.4, 1.5 |
| REQ-003 | 1.4, 2.1, 2.2 |
| REQ-004 | 2.1, 2.2 |
| REQ-005 | 2.2 (asserts single resolution per task start) |
| REQ-006 | 2.1, 2.2 (read-side), 2.4 (write-side) |
| REQ-007 | 3.1, 3.2 (incl. credential dropdown), 3.3 |
| REQ-008 | 3.4 |
| REQ-M01 | 2.3 |
