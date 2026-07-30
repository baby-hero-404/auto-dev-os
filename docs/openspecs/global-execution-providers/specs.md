# Specs: Global Default Execution Providers

## Added Requirements

### REQ-001: Global Configuration Storage
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN an Admin `PATCH`es `/organizations/{orgID}` with `default_execution_providers`
- THEN the backend validates it via `models.ValidateExecutionProviders` (same rules as project-level: valid `type`, known `ref`, `ref=="custom"` requires `cli_config.command`+`credential_id` — but only for `enabled:true` rows, per the fix already shipped this session)
- AND on success, persists it and responds with the updated Organization object
- AND a non-admin request is rejected (403) — enforced by the existing `RequireRole(UserRoleAdmin)` on the route, unchanged

### REQ-002: Orchestrator Fallback Logic
> ✅ Status: Fully Implemented

**Scenario: Project has never configured routing**
- WHEN a Task's project has `execution_providers=[]` and `execution_engine="api_native"` (both untouched defaults), and the organization has a non-empty `default_execution_providers` with an available candidate
- THEN `ResolveExecutionProvider` returns that candidate from the org's list

**Scenario: Project already uses the legacy single-engine field — org default must NOT override it**
- WHEN a Task's project has `execution_providers=[]` but `execution_engine="cli"` with a configured `cli_engine_config`, and the organization also has a `default_execution_providers` list
- THEN `ResolveExecutionProvider` still resolves via the legacy `CLIEngineConfig` path exactly as before — the org default is never consulted (precedence: legacy-if-explicitly-cli beats org default)

**Scenario: Project has its own execution_providers — org default never consulted**
- WHEN a Task's project has a non-empty `execution_providers` list
- THEN `ResolveExecutionProvider` uses only the project's list; if every candidate in it is unavailable, it errors `"no enabled execution provider is available"` — it does **not** fall through to the org default (matches REQ-004 of `cli-execution-provider-routing/`: exhausting an explicit list is a hard error, not a silent fallback)

**Scenario: Neither project nor org has anything configured**
- WHEN both `project.execution_providers` and `organization.default_execution_providers` are empty
- THEN behavior is unchanged from today: plain `api_native`

### REQ-003: AI Providers UI Global Routing Tab
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN the user visits the AI Providers page
- THEN they see a new "Global Routing" tab next to "API Connections" and "CLI Profiles"
- AND they can enable/reorder providers using the same `ExecutionProvidersList` UI already used in Project Settings
- AND saving calls `PATCH /organizations/{orgID}` with the new list

## Modified Requirements

### REQ-M01: Project Settings UI Inherit State
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN a user views Execution Providers in Project Settings, the project's own list is empty, and `execution_engine != "cli"`
- THEN the UI shows an inline indicator that it is inheriting the Global Default, with a "Customize for this project" action that seeds the project's editable list from the org's current default values (a one-time copy, not a live link — after saving, the project has its own explicit list per precedence)

### REQ-M02: `TaskService.validateTaskEngineOverride` honors the org default
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN a task-level `execution_engine="cli"` override is submitted for a project with empty `execution_providers`, `execution_engine != "cli"`, but the organization's `default_execution_providers` has an enabled `cli` entry
- THEN the override is accepted (not rejected with "project has no cli_engine_config configured") — same class of fix already applied for the project-level list this session, now extended one level further

## Removed Requirements
None.

## Correction found during implementation

REQ-002's scenarios above say `execution_providers=[]` as shorthand for "project hasn't configured routing" — the actual trigger condition implemented is **no row in the list is `enabled:true`**, not that the array is literally empty. These are usually the same thing, but not always: the frontend's `ExecutionProvidersList` always persists the full padded set of known provider rows on every Project Settings save (each defaulting to `enabled:false`), so a project's `execution_providers` is essentially never byte-empty once its settings have been saved even once. Gating on `len(providers) > 0` instead of "any row enabled" would have made the org-default fallback (and, it turns out, the pre-existing legacy fallback from `cli-execution-provider-routing/`) effectively dead for any already-saved project. Fixed via `models.HasEnabledProvider`, shared by `ResolveExecutionProvider` and `TaskService.validateTaskEngineOverride`. See tasks.md Task 1.2 for full detail and the regression tests that lock this in.
