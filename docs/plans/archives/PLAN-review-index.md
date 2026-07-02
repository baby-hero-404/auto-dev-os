# Code Review Plan — Feature Verification & Cleanup

**Created:** 2026-07-02  
**Scope:** Verify implementation correctness against feature specifications  
**Priority:** Logic correctness → Dead code cleanup → UI consistency

---

## Overview

This plan covers a systematic code review of 5 core features to verify implementation accuracy, identify dead/redundant code, and ensure UI matches backend contracts.

### Feature Scope & Priority

| # | Feature | Priority | Plan File |
|---|---------|----------|-----------|
| 1 | **5.7 Workflow Engine** | 🔴 Critical | [PLAN-review-workflow-engine.md](./PLAN-review-workflow-engine.md) |
| 2 | **5.12 Patch Engine Abstraction** | 🔴 Critical | [PLAN-review-patch-engine.md](./PLAN-review-patch-engine.md) |
| 3 | **5.6 Task System** | 🟡 Medium | [PLAN-review-task-project-ui.md](./PLAN-review-task-project-ui.md) |
| 4 | **5.5 Project System** | 🟡 Medium | [PLAN-review-task-project-ui.md](./PLAN-review-task-project-ui.md) |
| 5 | **5.11 Repository Profile Cache** | 🟢 Low | [PLAN-review-repo-profile-cache.md](./PLAN-review-repo-profile-cache.md) |
| 6 | **Test Coverage (Patch Engine)** | 🟡 Medium | [PLAN-test-coverage.md](./PLAN-test-coverage.md) |

### Execution Order

```
[✅] Phase 1: Workflow Engine (5.7)          ← Core orchestration logic
    └── Includes: DAG steps, state machine, checkpoint/recovery, review-fix cycles
[✅] Phase 2: Patch Engine (5.12)            ← Code application logic
    └── Includes: Applier, Validator, Search & Replace parser, self-healing retry
[✅] Phase 3: Task & Project UI (5.5 + 5.6) ← UI/Backend contract alignment
    └── Includes: Workspace layout, project settings, task lifecycle UI
[✅] Phase 4: Repo Profile Cache (5.11)      ← Planned feature gap analysis
    └── Includes: Verify what exists vs what spec says should exist
```

### Review Methodology

For each file under review:
1. **Read the feature spec** — understand the expected behavior
2. **Trace code path** — verify implementation matches spec
3. **Identify dead code** — find unused functions, stale imports, commented-out blocks
4. **Check test coverage** — verify critical paths have tests
5. **Document findings** — log corrections needed with file:line references
