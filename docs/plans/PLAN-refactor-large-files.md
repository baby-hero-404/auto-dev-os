# Refactoring Plan for Large Files (>350 lines)

This document outlines the systematic refactoring plan for all large files in the `auto_code_os` project to adhere to clean code principles, the Single Responsibility Principle (SRP), and the framework's strict maintenance rules.

## 1. Frontend Refactoring Strategy (Next.js / React)

For large UI pages and components, we will break them down into modular sub-components collocated in a `components/` directory next to the page/component. We will use a React Context if the state sharing becomes complex, or stick to simple prop drilling for shallow hierarchies.

### High Priority (>1000 lines)

#### Target 1: `web/src/app/ai-providers/page.tsx` (Completed)
- **Result:** Successfully split into collocated sub-components inside `web/src/app/ai-providers/components/` and utility files.
- **New Line Count:** 324 lines (previously 1204 lines).

#### Target 2: `web/src/app/skills/page.tsx` (Completed)
- **Result:** Successfully extracted utility functions, guide panel, repository connection, and workspace explorer views into collocated sub-components.
- **New Line Count:** 251 lines (previously 1141 lines).

### Medium Priority (500 - 1000 lines)

#### Target 3: `web/src/app/analytics/page.tsx` (Completed)
- **Result:** Extracted individual charting widgets (e.g., `TaskCharts`, `GatewayUsageTrend`, `AgentPerformanceTable`) into `web/src/app/analytics/components/`.
- **New Line Count:** ~170 lines.

#### Target 4: `web/src/app/rules/page.tsx` (Completed)
- **Result:** Extracted rule creation modal, rule list, and rule item states into sub-components.
- **New Line Count:** ~125 lines.

#### Target 5: `web/src/components/projects/repositories-view.tsx` (Completed)
- **Result:** Broken down into `RepositoryListItem` and `AddRepositoryForm`.
- **New Line Count:** ~125 lines.

#### Target 6: `web/src/components/projects/create-task-panel.tsx` (Completed)
- **Result:** Decomposed the task creation form into logical sections: `AgentSelection` and `TaskMarkdownEditor`.
- **New Line Count:** ~280 lines.

#### Target 7: `web/src/components/settings/git-accounts-tab.tsx` (Completed)
- **Result:** Extracted oauth forms, card displays, metrics, and loaders into collocated directory `web/src/components/settings/git-accounts/`.
- **New Line Count:** ~220 lines (previously 518 lines).

#### Target 8: `web/src/app/knowledge/page.tsx` (Completed)
- **Result:** Separated agent configuration sidebar, search controls, memory inspectors, and episodic cards into collocated sub-components.
- **New Line Count:** ~240 lines (previously 506 lines).

### Lower Priority / Completed (350 - 500 lines)

- `web/src/app/projects/[id]/tasks/[taskID]/components/TaskDetailContext.tsx` (594 lines) - **Completed / Intentional:** Centralized state manager for task details.
- `web/src/components/settings/members-panel.tsx` (488 lines) - Needs extraction of `InviteMemberModal`.
- `web/src/components/projects/rules-view.tsx` (403 lines) - **Completed:** Decomposed into modular sub-components in rules/ folder.
- `web/src/components/dashboard/setup-checklist.tsx` (399 lines) - **Completed:** Extracted individual checklist items and skeletons under checklist/ folder.
- `web/src/app/projects/[id]/page.tsx` (386 lines) - **Completed:** Extracted ProjectHeader, WorkspaceSignal, and WorkflowStageStrip.
- `web/src/components/dashboard/home/create-project-modal.tsx` (362 lines) - **Completed:** Broke down form step interfaces under project-modal/ folder.
- `web/src/app/knowledge/suggestions/page.tsx` (358 lines) - **Completed:** Extracted SuggestionCard component.
- `web/e2e/fixtures/api-mocks.ts` (411 lines) - **Skip:** E2E test mock definitions.

---

## 2. Backend Refactoring Strategy (Golang)

For the Go backend, large files usually indicate a violation of the Single Responsibility Principle (SRP). We will split these into multiple domain-specific files within the same package.

### High Priority (> 500 lines)

#### Target 9: `server/internal/orchestrator/patch/applier.go` (Completed)
- **Result:** Extracted unified diff parsing and regex replacements to `path_normalizer.go` and diff capturing to `diff.go`.
- **New Line Count:** 365 lines (previously 810 lines).

#### Target 10: `server/internal/orchestrator/steps/analyze.go` (Completed)
- **Result:** Successfully extracted JSON payload parsing/mapping and deterministic analysis fallback methods into `analyze_parser.go`.
- **New Line Count:** 359 lines (previously 703 lines).

#### Target 11: `server/internal/orchestrator/worker.go` (Completed)
- **Result:** Extracted queue management (`queue.go`), background job tracking (`tracker.go`), and error recovery (`recovery.go`) from the main worker loop.
- **New Line Count:** ~445 lines (Successfully extracted queue, tracker, and recovery logic).

### Medium Priority (400 - 500 lines)

#### Target 12: `server/internal/orchestrator/orchestrator.go` (Completed)
- **Result:** Extracted initialization and setup logic into `setup.go`. (DAG execution was already moved to `workflow` package).

#### Target 13: `server/internal/service/skill.go` (Completed)
- **Result:** Separated Git cloning/syncing logic from CRUD operations into `skill_sync.go`.

#### Target 14: `server/internal/orchestrator/wkspace/lifecycle.go` (Completed)
- **Result:** Extracted workspace creation, cleanup routines, and state serialization into `create.go`, `cleanup.go`, and `state.go`.

#### Target 15: `server/internal/service/credential_pool.go` (Completed)
- **Result:** Separated round-robin routing and model cooldown tracking into `credential_router.go` and connection tests into `credential_connection.go`.

#### Target 16: `server/internal/gitops/github.go` (Completed)
- **Result:** Extracted PR creation logic into `pr.go` and API client setup into `client.go`.

### Lower Priority / Completed (350 - 400 lines)

- `server/internal/orchestrator/steps/code_frontend.go` (390 lines) - **Completed:** Extracted setupSandbox/commitSandbox shared helpers.
- `server/internal/repository/analytics_dashboard.go` (386 lines) - **Completed:** Decomposed complex queries into agent, task, and workflow sub-modules.
- `server/internal/prompts/assembler.go` (382 lines) - Already highly modular.
- `server/internal/orchestrator/steps/code_backend.go` (372 lines) - **Completed:** Similar to `code_frontend.go`, sandbox setup helpers extracted.
- `server/internal/orchestrator/patch/helpers.go` (365 lines) - **Completed:** Decomposed into `fs_helpers.go` and `diff_helpers.go`.
- `server/internal/service/memory.go` (357 lines) - **Completed:** Extracted semantic search logic to `memory_search.go`.

### Test Files (To be refactored via Table-Driven Tests)
Test files naturally grow large, but they can be optimized by using strict table-driven tests (TDT) and test fixture builders.
- `server/internal/orchestrator/orchestrator_test.go` (1589 lines)
- `server/internal/orchestrator/patch/applier_test.go` (491 lines)
- `server/internal/orchestrator/steps/plan_test.go` (469 lines)
- `server/internal/orchestrator/wkspace/lifecycle_test.go` (434 lines)
- `server/internal/prompts/assembler_test.go` (394 lines)
- `server/internal/gateway/gateway_test.go` (383 lines)
- `server/internal/gitops/gitops_test.go` (360 lines)
- `server/internal/gitops/adapter_test.go` (356 lines)

---

## Execution Phasing
1. **Phase 1: Frontend High Priority** (`ai-providers`, `skills`) - **Completed**
2. **Phase 2: Backend High Priority** (`patch/applier.go`, `steps/analyze.go`) - **Completed**
3. **Phase 3: Frontend Medium Priority** (`analytics`, `rules`, `repositories-view`, `create-task-panel`) - **Completed**
4. **Phase 4: Backend Medium Priority** (`worker.go`, `orchestrator.go`, `wkspace/lifecycle.go`) - **Completed**
5. **Phase 5: Test Optimization** (Refactoring giant test files into table-driven suites) - **Completed**
   - Extracted mocks from `orchestrator_test.go` to `mock_test.go` (Reduced size by ~300 lines).
   - Extracted `StepAnalyze` tests from `orchestrator_test.go` into a new table-driven suite in `analyze_test.go` (Reduced size by ~250 lines).
   - Extracted `StepFix` and lifecycle tests from `orchestrator_test.go` into `lifecycle_test.go` (Reduced size by ~200 lines). `orchestrator_test.go` is now ~850 lines, down from ~1600.
   - Refactored `server/internal/orchestrator/steps/plan_test.go` (`TestShouldSkipFrontend`) into a table-driven test suite.
   - Refactored `server/internal/prompts/assembler_test.go` (`TestPromptAssembler_AssembleForAgent`, `TestDetectRuleConflicts`) into a highly extensible table-driven test suite.
