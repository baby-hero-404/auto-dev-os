# Workflow Pipeline & Stale Metadata Fixes Implementation Plan

> **For agentic workers:** Use subagent-driven-development or executing-plans
> to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Implement workspace branch recovery logic in the backend and enrich the UI workflow step progress display.

**Architecture:** 
1. Backend: Enhance `LoadTaskWorkspace` in `lifecycle.go` to query the repository's target branch from the database and correct the workspace metadata if the `default_branch` is stale.
2. Frontend: Update `page.tsx` task flow rendering to extract step start times, elapsed execution durations, and step-specific errors from checkpoints, rendering them directly under the corresponding step dot.

**Tech Stack:** Go, TypeScript, Next.js, React

---

### Task 1: Backend Workspace Metadata Recovery

**Files:**
- Modify: `server/internal/orchestrator/wkspace/lifecycle.go`
- Test: `server/internal/orchestrator/wkspace/lifecycle_test.go`

- [x] **Step 1: Implement Dynamic Branch Sync in LoadTaskWorkspace**

Update `LoadTaskWorkspace` to retrieve project repositories using `m.Repositories.ListByProjectID` and reconcile any stale `default_branch` and its dependent `Paths.Main` mapping inside the loaded `metadata.json` (note: other path fields like `Worktrees` are branch-independent and do not require rebuilding). Save updates back to disk immediately.

```go
// Add after unmarshaling meta in server/internal/orchestrator/wkspace/lifecycle.go:
	if m.Repositories != nil {
		projectRepos, err := m.Repositories.ListByProjectID(ctx, task.ProjectID)
		if err == nil {
			repoMap := make(map[string]models.Repository)
			for _, r := range projectRepos {
				repoMap[r.ID] = r
			}

			updated := false
			for i, rWS := range meta.Repos {
				if r, ok := repoMap[rWS.RepoID]; ok {
					expectedBranch := r.Branch
					if expectedBranch == "" {
						expectedBranch = "main"
					}
					if rWS.DefaultBranch != expectedBranch {
						meta.Repos[i].DefaultBranch = expectedBranch
						meta.Repos[i].Paths.Main = orchestratorworkspace.NewPathManager("").RepoMainRelative(rWS.Name, expectedBranch)
						updated = true
					}
				}
			}
			if updated {
				if saveErr := m.SaveTaskWorkspaceMetadata(task, &models.TaskWorkspace{Root: ws.Root, Repos: meta.Repos}); saveErr != nil {
					return nil, fmt.Errorf("failed to save reconciled workspace metadata: %w", saveErr)
				}
			}
		}
	}
```

- [x] **Step 2: Add Unit Test for Stale Branch Synchronization**

Add a unit test in `server/internal/orchestrator/wkspace/lifecycle_test.go` confirming that when `LoadTaskWorkspace` is called with mismatching metadata (e.g. branch is `"main"` in json but `"master"` in DB), it correctly overrides `DefaultBranch` and main relative paths.

- [x] **Step 3: Run and Verify Backend tests**

Run: `go test -v ./internal/orchestrator/wkspace/...`
Expected: PASS

- [x] **Step 4: Commit**

```bash
git add server/internal/orchestrator/wkspace/
git commit -m "feat(orchestrator): auto-sync stale workspace branch from db repository"
```

---

### Task 2: Frontend Workflow Pipeline Step Enhancements

**Files:**
- Modify: `web/src/app/projects/[id]/tasks/[taskID]/page.tsx`

- [x] **Step 1: Calculate step-specific metadata, duration, and error info**

Parse the list of checkpoints in `useMemo` to extract status, error, and timestamp for each step. Compute step-by-step elapsed time durations dynamically based on the actual visible workflow step sequence (ignoring unrelated checkpoints).

```typescript
// Replace latest memo in page.tsx with:
  const stepDurations = useMemo(() => {
    const map = new Map<string, string>();
    if (!workflow?.checkpoints || workflow.checkpoints.length === 0) return map;

    const checkpointsByStep = new Map<string, typeof workflow.checkpoints>();
    for (const cp of workflow.checkpoints) {
      if (!workflowSteps.includes(cp.step)) continue;
      const list = checkpointsByStep.get(cp.step) || [];
      list.push(cp);
      checkpointsByStep.set(cp.step, list);
    }

    let previousStepEnd: number | null = null;
    for (const step of workflowSteps) {
      const cps = checkpointsByStep.get(step);
      if (!cps || cps.length === 0) continue;

      const sortedCps = [...cps].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime());
      const firstCp = sortedCps[0];
      const lastCp = sortedCps[sortedCps.length - 1];
      
      let startMs = new Date(firstCp.created_at).getTime();
      if (sortedCps.length === 1 && firstCp.state?.status !== "running" && previousStepEnd !== null) {
        startMs = previousStepEnd;
      }
      
      const isRunning = lastCp.state?.status === "running";
      const endMs = isRunning ? Date.now() : new Date(lastCp.created_at).getTime();
      previousStepEnd = endMs;
      
      const durationSec = Math.max(0, Math.round((endMs - startMs) / 1000));
      if (durationSec < 60) {
        map.set(step, `${durationSec}s`);
      } else {
        const min = Math.floor(durationSec / 60);
        const sec = durationSec % 60;
        map.set(step, `${min}m ${sec}s`);
      }
    }
    return map;
  }, [workflow, workflowSteps]);
```

- [x] **Step 2: Update Task Flow Render Block**

Modify the JSX template loop inside the "Task Flow" section to display the localized timestamp, calculated duration, and any inline error warnings beneath the corresponding step name.

- [x] **Step 3: Commit**

```bash
git add web/src/app/projects/[id]/tasks/[taskID]/page.tsx
git commit -m "feat(web): render step execution duration, timestamp, and errors in task flow UI"
```
