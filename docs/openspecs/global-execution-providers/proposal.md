# Proposal: Global Default Execution Providers

## Why
Currently, the priority fallback order (Execution Providers) for routing tasks between different CLI engines and API gateways is strictly defined at the Project level. This forces users with multiple projects (e.g. 10+) to manually configure the exact same prioritized list (e.g., Claude Code -> Antigravity -> Codex -> Anthropic API) across all projects. This creates a terrible UX and violates the DRY principle for organizational settings.

> **Corrections from the initial draft** (verified against the actual codebase before implementing):
> - The API layer lives at `server/internal/handler/` (`server/internal/api/` does not exist — there is no `internal/api` package in this repo).
> - `Organization` already has a full CRUD stack (`pkg/models/organization.go`, `internal/repository/organization.go`, `internal/service/organization.go`, `internal/handler/organization.go`, routed at `PATCH /organizations/{orgID}`, already admin-gated via `RequireRole(UserRoleAdmin)`) — Task 1.1 is additive to existing files, not new ones.
> - `Project.ExecutionProviders` is stored as `json.RawMessage` (jsonb), validated through `models.ValidateExecutionProviders`, not as a typed `[]ExecutionProviderConfig` Go slice — `Organization.DefaultExecutionProviders` follows the same convention for consistency and so it can reuse the exact same validator (see design.md).
> - The frontend has no `updateOrganization` API binding yet (`web/src/lib/api/auth.ts` only has `getOrganization`) — needs adding.
> - **Precedence with the legacy single-engine fallback was unspecified in the original draft** — this matters a lot: `cli-execution-provider-routing/`'s REQ-003 guarantees a project that already has `execution_engine="cli"` configured keeps running exactly as before, forever, regardless of what else changes. If the org-default fallback were inserted *above* that legacy path, an org enabling a global default would silently change behavior for every project still on the old single-engine field — a real regression for exactly the "10+ existing projects" scenario this proposal targets. See design.md's "Precedence" section for the resolved ordering.

## What Changes

### Issue 1: Global Settings
- Add `DefaultExecutionProviders json.RawMessage` (jsonb) to the `Organization` model, repository, service (validated via `models.ValidateExecutionProviders`, same as Project), and handler (`UpdateOrganizationInput`) — mirrors exactly how `Project.ExecutionProviders` is already wired.
- Expose a new UI tab "Global Routing" under the AI Providers page to configure this default list, reusing `ExecutionProvidersList` (already a generic `{value, onChange, disabled}` component, no project-specific coupling).

### Issue 2: Orchestrator Fallback
- Extract the candidate-selection loop already inside `ResolveExecutionProvider` into a reusable `resolveFromProviderList` helper (currently only usable inline), so it can run against either `project.ExecutionProviders` or `organization.DefaultExecutionProviders`.
- `ResolveExecutionProvider`: when `project.ExecutionProviders` is empty, **and** the project's legacy `ExecutionEngine` is not explicitly `"cli"`, try `organization.DefaultExecutionProviders` before falling through to the api_native default. A project already running the legacy CLI path is untouched.
- `shouldUseCLISpecFirstWorkflow` gets the same fallback, for consistency with how it already special-cases `project.ExecutionProviders`.
- `TaskService.validateTaskEngineOverride` gets the same fallback so a task-level `execution_engine="cli"` override isn't wrongly rejected when the only thing making CLI available is the org default (same class of bug already fixed once this session for the project-level list).

## Capabilities

### New Capabilities
- Org admins can define a global routing fallback strategy once (e.g. `claude_code` > `antigravity` > `openai_codex` > `anthropic`).

### Modified Capabilities
- Projects with no local `execution_providers` and no explicit legacy `execution_engine="cli"` automatically inherit the org default.

### Removed Capabilities
- None.

## Impact

| Area | Files Affected |
|------|----------------|
| DB Schema | `server/pkg/models/organization.go`, `server/migration/000020_add_org_default_execution_providers.{up,down}.sql` |
| Backend | `server/internal/repository/organization.go`, `server/internal/service/organization.go`, `server/internal/handler/organization.go` (payload passthrough only, handler logic unchanged) |
| Orchestrator | `server/internal/orchestrator/interfaces.go` (new `OrganizationRepository` interface), `server/internal/orchestrator/orchestrator.go` (new `WithOrganizationRepository` option), `server/internal/orchestrator/execution_router.go`, `server/cmd/api/main.go` (wire the option) |
| Task validation | `server/internal/service/task.go` (`TaskService` gains an org repo dependency) |
| UI | `web/src/app/ai-providers/page.tsx`, `web/src/lib/api/auth.ts` (add `updateOrganization`), `web/src/lib/types.ts` (`Organization.default_execution_providers`), `web/src/components/projects/project-profile.tsx` (inherited-state indicator) |
