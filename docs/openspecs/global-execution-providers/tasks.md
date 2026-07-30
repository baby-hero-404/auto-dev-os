# Tasks: Global Default Execution Providers

## P0 — Backend foundation

### Task 1.1: Database & Model Update
> Links to: REQ-001

- [x] Migration `server/migration/000020_add_org_default_execution_providers.{up,down}.sql` — applied live, column verified (`\d organizations`).
- [x] `DefaultExecutionProviders json.RawMessage` added to `Organization` (`pkg/models/organization.go`), and to `UpdateOrganizationInput`.
- [x] `internal/repository/organization.go`: `Update` passes `input.DefaultExecutionProviders` through to the `updates` map.
- [x] `internal/service/organization.go`: `Update` calls `models.ValidateExecutionProviders`, returns `ErrValidation` on failure.
- [x] Tests: `TestOrganizationService_Update_InvalidDefaultExecutionProviders`, `TestOrganizationService_Update_EmptyDefaultExecutionProvidersIsNoop`. Live round-trip verified via curl against a running server (see below).

### Task 1.2: Orchestrator — extract shared candidate-selection helper
> Links to: REQ-002

- [x] `OrganizationRepository` interface (`interfaces.go`) + `WithOrganizationRepository` option (`orchestrator.go`) + `orgs` field.
- [x] Wired `orchestrator.WithOrganizationRepository(orgRepo)` in `cmd/api/main.go` (reused the already-constructed `orgRepo`).
- [x] Extracted `resolveFromProviderList` out of `ResolveExecutionProvider`'s inline loop — existing router tests confirmed byte-identical behavior immediately after.
- [x] Added `resolveFromOrgDefault` per design.md.
- [x] `ResolveExecutionProvider` precedence: project list (if it has an **enabled** row — see the important correction below) → (legacy-if-cli, else org default) → legacy.
- [x] `shouldUseCLISpecFirstWorkflow` simplified to just delegate to `ResolveExecutionProvider` (it already encodes the full chain now — no need to duplicate the empty-list special case).
- [x] Tests: `TestResolveExecutionProvider_OrgDefaultUsedWhenProjectHasNothing`, `TestResolveExecutionProvider_LegacyCLIBeatsOrgDefault`, `TestResolveExecutionProvider_ProjectListNeverFallsThroughToOrgDefault`, `TestResolveExecutionProvider_NeitherConfiguredIsPlainAPINative`, `TestResolveExecutionProvider_NoOrgRepoWiredFallsBackSilently`.

> **⚠️ Important correction found during implementation, not in the original design:** the frontend's `ExecutionProvidersList` always persists the *full padded set* of known provider rows on every Project Settings save (each defaulting to `enabled:false`), not just the rows the user touched. That means `project.ExecutionProviders` is essentially never byte-empty (`len(providers) > 0`) once a project's settings have been saved even once — so the original `len(providers) > 0` gate would treat *every already-saved project* as "explicitly configured" and hard-error instead of ever falling through to the org default, making this whole feature non-functional in the common case. Fixed by changing the gate to `models.HasEnabledProvider(providers)` (at least one row **enabled**, not merely present) — added to `pkg/models/project.go` so both the orchestrator and `TaskService` share the identical definition. This also fixes a latent bug in the already-shipped project-level routing (a project whose settings had been saved once, with nothing enabled, would previously hard-error instead of falling back to the legacy `CLIEngineConfig`). Test `TestResolveExecutionProvider_DisabledSkipped` was replaced with `TestResolveExecutionProvider_OnlyDisabledFallsThroughToDefault` (corrected expectation) + `TestResolveExecutionProvider_DisabledCandidateNeverSelected` (preserves the original "disabled must never be selected" guarantee for the case where something else *is* enabled).

### Task 1.3: `TaskService.validateTaskEngineOverride` — org default awareness
> Links to: REQ-M02

- [x] Threaded an org repo into `TaskService` (`NewTaskService` gained a parameter; `cmd/api/main.go` call site updated).
- [x] Extended the fallback chain with the same `models.HasEnabledProvider` gate correction as Task 1.2.
- [x] Tests: `TestValidateTaskEngineOverride_OrgDefaultEnabledCLI`, `TestValidateTaskEngineOverride_PaddedButAllDisabledFallsThrough`, `TestValidateTaskEngineOverride_LegacyCLIEngineConfigStillWorks` (now also asserts via `mock.ExpectationsWereMet()` that the org lookup is *not* attempted when the project is explicitly on the legacy cli path).

## P1 — Frontend

### Task 2.1: Global Routing UI Tab
> Links to: REQ-003

- [x] `web/src/lib/types.ts`: `Organization.default_execution_providers?: ExecutionProviderConfig[]`.
- [x] `web/src/lib/api/auth.ts`: added `updateOrganization(orgID, token, input)`; exported from `web/src/lib/api/index.ts`.
- [x] New `web/src/app/ai-providers/components/GlobalRoutingPanel.tsx` — fetches the org, renders `ExecutionProvidersList` bound to its default list, saves via `api.updateOrganization`. Render-phase state sync (not `useEffect`+`setState`) to match `project-profile.tsx`'s existing pattern and avoid the cascading-render lint error caught by `eslint react-hooks/set-state-in-effect`.
- [x] `web/src/app/ai-providers/page.tsx`: added third "Global Routing" tab; existing `activeTab === "api" | "cli"` conditionals throughout the file are unaffected since the whole credentials section is now wrapped in `{activeTab === "global" ? <GlobalRoutingPanel /> : (...)}`.
- [x] `npx tsc --noEmit` and `npx eslint` both clean.

### Task 2.2: Project Settings UI Indication
> Links to: REQ-M01

- [x] `project-profile.tsx`: fetches the org (`api.getOrganization`) via `project.org_id` + `token` (props already available, no new prop threading needed).
- [x] Inline banner shown when `!executionProviders.some(p => p.enabled)` (mirrors `models.HasEnabledProvider` exactly) **and** the org has an enabled default — "Customize for this project" copies the org's current default array into local state.
- [x] Note: the frontend `Project` type no longer exposes `execution_engine`/`cli_engine_config` at all (removed by unrelated concurrent frontend work this session) — the banner therefore can't distinguish "inheriting" from "on an old, pre-ExecutionProviders-UI legacy CLI project" the way the backend precisely can. Low-stakes cosmetic gap only (backend routing is unaffected and fully correct regardless); not fixed here since it would require re-adding fields the frontend deliberately dropped.

## Verification

- [x] Full backend suite (`go build`, `go vet`, `go test ./...`) green throughout.
- [x] `npx tsc --noEmit` and `npx eslint` clean on all touched frontend files.
- [x] Live end-to-end verification against a running server + real Postgres (not just unit tests): registered an org, confirmed `default_execution_providers` starts `[]`, created a project, saved it with the UI's real padded-but-all-disabled `execution_providers` shape, set an org default with `claude_code` enabled, then created a task with `execution_engine:"cli"` — **accepted** (proves project→org-default fallthrough works end-to-end, not just in mocks). Cleared the org default and repeated — **rejected with the expected legacy-config error** (proves the negative path too). Cleaned up test data afterward.

## Self-review checklist

| REQ | Task |
|---|---|
| REQ-001 | 1.1 |
| REQ-002 | 1.2 |
| REQ-003 | 2.1 |
| REQ-M01 | 2.2 |
| REQ-M02 | 1.3 |
