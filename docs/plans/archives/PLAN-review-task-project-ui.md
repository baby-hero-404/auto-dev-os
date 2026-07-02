# Phase 3: Task & Project System UI Review (Features 5.5 + 5.6)

**Feature Specs:** `docs/features/5.5-project-system.md`, `docs/features/5.6-task-system.md`  
**Priority:** 🟡 Medium — UI/Backend contract alignment  
**Status:** ✅ Review Complete — 2026-07-02 (1 bug fixed)

---

## Scope

Verify that:
- Project settings UI exposes all AI Workflow Defaults from spec
- Task lifecycle UI reflects all 12 states correctly
- Backend API contracts match what frontend consumes
- Workspace layout and metadata structures match spec 5.6

---

## Files Reviewed

### A. Project System — Backend

| File | Reviewed |
|:-----|:---------|
| `pkg/models/project.go` | ✅ |
| `service/project.go` | ✅ |
| `handler/project.go` | ✅ |

**Checklist:**
- [x] Project model includes all AI Workflow Defaults from spec 5.5 Section C:
  - `default_model_level` (balanced) ✅
  - `default_autonomy` (supervised) ✅
  - `auto_review_policy` (complexity_based) ✅
  - `max_retries` (3) ✅
  - `max_review_fix_cycles` (3) ✅
  - `default_branch` (main) ✅
- [x] Project data storage path matches spec 5.5 Section E: `{dataRoot}/projects/{project_id}/rules|skills|docs` ✅
- [x] Knowledge Base injection logic exists (basic keyword matching in `context_load.go`) ✅
- [x] Create/Update/Delete API endpoints complete ✅
- [x] Disk cleanup on Delete (`os.RemoveAll`) ✅
- [x] Async seed on Create (`go s.seeder.SeedProject()`) ✅

### B. Task System — Backend

| File | Reviewed |
|:-----|:---------|
| `pkg/models/task.go` | ✅ |
| `service/task.go` | ✅ |
| `handler/task.go` | ✅ |

**Checklist:**
- [x] Task model defines all 12 states ✅
- [x] Task supports `repository_id` (single-repo, optional) ✅
- [x] `pr_urls` is `pq.StringArray` (multi-repo PR tracking) ✅
- [x] `pr_metadata` is `json.RawMessage` (flexible PR data) ✅
- [x] State machine enforced via `ValidateTaskTransition()` in Update ✅
- [x] `Analyze()` uses `policy.ShouldAutoApproveSpec()` for human gate ✅
- [x] `ApproveAnalysis()` transitions to `coding` ✅
- [x] `RequestAnalysisChanges()` transitions to `spec_review` ✅
- [x] Sub-task support: `CreateSubTask`, `ListSubTasks` ✅
- [x] Orchestrator integration: Create → Execute, Approve → Execute ✅
- [ ] Restart API (`POST /tasks/{id}/restart`) — not found as dedicated endpoint. Restart is handled via status update to `todo` + re-execute

### C. Project UI — Frontend

| File | Reviewed |
|:-----|:---------|
| `web/src/app/projects/[id]/page.tsx` | ✅ |
| `web/src/components/projects/project-profile.tsx` | ✅ |
| `web/src/components/projects/repositories-view.tsx` | ✅ |
| `web/src/components/projects/create-task-panel.tsx` | ✅ |
| `web/src/components/projects/rules-view.tsx` | ✅ |
| `web/src/components/projects/agents-view.tsx` | ✅ |

**Checklist:**
- [x] Project settings page exposes all 6 AI Workflow Defaults ✅
- [x] Repository linking UI exists ✅
- [x] Rules management (CRUD + seed) ✅
- [x] Agent assignment UI ✅
- [x] Keyboard shortcuts (1-5) for view switching ✅
- [ ] Knowledge Base section — not present in UI (spec says basic, backend has it, no frontend)

### D. Task UI — Frontend

| File | Reviewed |
|:-----|:---------|
| `web/src/lib/types.ts` (TaskStatus, TaskAnalysis) | ✅ |
| `web/src/lib/utils/task-utils.ts` | ✅ |
| `web/src/components/projects/tasks-tab.tsx` | ✅ |
| `web/src/components/projects/spec-review-section.tsx` | ✅ |
| `web/src/components/projects/task-pr-review.tsx` | ✅ |
| `web/src/components/projects/task-diff-viewer.tsx` | ✅ |
| `web/src/components/projects/task-clarification-form.tsx` | ✅ |
| `web/src/components/projects/task-action.tsx` | ✅ |

**Checklist:**
- [x] `TaskStatus` type matches all 12 backend statuses ✅
- [x] Workflow stage strip covers all status groups (9 visual stages) ✅
- [x] `failed` handled separately by `isFailedTask()` ✅
- [x] Spec review section with approve/reject actions ✅
- [x] PR review section ✅
- [x] Diff viewer ✅
- [x] Clarification form ✅
- [x] Risk assessment calculation includes `risk_domains` ✅

### E. 🐛 Bug Fixed

**Model Level Dropdown Mismatch** — `project-profile.tsx:L131`

```diff
-<option value="deep">Deep</option>
+<option value="powerful">Powerful</option>
```

UI was sending `"deep"` but backend/gateway expects `"powerful"` (matching `config.yaml` and gateway routing: fast/balanced/powerful).

### F. Missing Features (Backlog)

| Feature | Status |
|:--------|:-------|
| Knowledge Base UI | ❌ Not implemented — backend has basic keyword matching but no frontend |
| Dedicated Task Restart endpoint | ❌ Handled via status update workaround |
