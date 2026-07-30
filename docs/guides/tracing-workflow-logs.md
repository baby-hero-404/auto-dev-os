# Tracing workflow/task issues from logs

A runbook for diagnosing orchestrator failures using only task logs and
filesystem/git state — no debugger attached, no reproducing locally. Written
from a real debugging session that found and fixed 5 separate bugs this way.

## 1. Find the right log file

Every task's full log history is a JSONL file at:

```
server/.data/logs/<task-id>.jsonl
```

Each line: `{"task_id", "job_id", "level", "message", "created_at"}`. `job_id`
is empty for step-internal logs (e.g. repo-path resolution) and set for
step-lifecycle logs (`step X running/success/failed`).

Quick triage — pull just the interesting levels first, not the whole file:

```bash
python3 -c "
import json
with open('server/.data/logs/<task-id>.jsonl') as f:
    for l in f:
        if not l.strip(): continue
        d = json.loads(l)
        if d.get('level') in ('warn', 'error') or 'failed' in d.get('message','').lower():
            print(d['created_at'], d['level'], d.get('job_id','-'), d['message'][:300])
"
```

Then re-read the full tail around any timestamp of interest — warn/error
lines alone often omit the successful steps that set up the failure.

## 2. Correlate log timestamps with git state

Most orchestrator bugs in this codebase involve the workspace git checkout at
`server/.data/workspaces/<task-id>/code/repos/<repo-name>/main`. When a log
timestamp implicates a workspace operation, check reflog immediately —
it's the ground truth for exactly what happened and when:

```bash
cd server/.data/workspaces/<task-id>/code/repos/<repo-name>/main
git reflog --date=iso
git status --short
git log --oneline -10
```

A `reset: moving to HEAD` entry timestamped near a failure is almost always
`ResetExistingWorkspace` (`wkspace/helpers.go`) firing — i.e. `EnsureWorkspaceCloned`
decided the workspace had no work worth preserving and wiped it. Compare that
timestamp against the checkpoint history for the task to see what step's
checkpoint status it was evaluating at that moment.

## 3. Don't trust "it happened once, must be fixed now" — check what's already lost

A fix to `EnsureWorkspaceCloned`'s preserve-logic stops *future* wipes, but
can't resurrect files a *prior* wipe already deleted for a task that's
already in-flight. If a step's resume-shortcut path only *validates* files
exist (rather than regenerating them — e.g. `cli_spec`'s
`SpecStatus == Approved` fast path), a task whose files were wiped before the
fix landed will keep failing forever on retry. Check:

```bash
ls <repo-path>/docs/openspecs/<slug>/   # or whatever the step's expected output is
```

If it's gone and the step can't regenerate it, the fix is correct but this
*specific task instance* is unrecoverable — cut a fresh test task instead of
retrying the poisoned one.

## 4. One error message can hide a second bug behind it

Fixing the first error surfaced often reveals the next one on the same
retry — don't declare victory on log silence for one error class. In this
session: workspace-wipe fix → revealed a task-status-transition bug in the
next step → fixing that revealed cross_review was reviewing an empty diff.
Keep re-running and re-tailing the log after every fix.

## 5. Trace `UpdateTaskStatus` calls when you see "invalid task transition"

`models.ValidTaskTransitions` (`pkg/models/task.go`) is a hand-maintained
map — it does not auto-derive from what steps actually do. When a step's
`Execute` calls `s.status.UpdateTaskStatus(ctx, taskID, X)` and fails with
`invalid task transition from A to X`, check two things, not just the table:

1. Is `A -> X` actually a transition this workflow variant needs? (Add it to
   the table if so — see git history around 2026-07-30 for the
   `coding -> human_review` addition cli_mr needed.)
2. Is `A` even the *real* current status, or is it a stale in-memory
   `rt.Task.Status` left over from an earlier step in the same job run that
   updated the DB but forgot to also set `s.rt.Task.Status = X` afterward?
   `StepRuntime.Task` is a shared pointer across every step constructed for
   a job (`step_registry.go`) — if step N updates the DB status without
   syncing the in-memory field, step N+1's own status guards
   (`if s.rt.Task.Status == ...`) and its own subsequent `UpdateTaskStatus`
   calls are working off of stale data. Grep for the pattern:

   ```bash
   grep -n "UpdateTaskStatus(ctx" internal/orchestrator/steps/*.go
   grep -n "rt.Task.Status = " internal/orchestrator/steps/*.go
   ```

   Every `UpdateTaskStatus` call should have a matching in-memory sync right
   after it unless the step immediately returns/pauses (so no later step in
   the same run reads the stale value).

## 6. A step "passing" isn't proof it did real work

`cross_review` logged `no diff was provided to cross_review step` as a warn
on *every* run, including the one that ultimately passed and got merged. A
warn easy to skim past turned out to mean the review LLM was reviewing an
empty diff and rubber-stamping. When a step logs a warning about missing
input on every single invocation (not just failures), treat that as a
correctness bug, not noise — trace which diff-capture function it calls
(`CaptureWorkspaceDiff`: uncommitted-only, vs `CapturePRDiff`: against base
branch) and whether the *previous* step already committed its changes (check
for `checkpoint_<step>: STAGED_COUNT=N\n<hash>` log lines — that's this
codebase's signature for "this step just committed").

## 7. Debug logging you add while tracing is not free — remove it after

Temporary `s.log.Log(ctx, ..., "debug", ...)` calls added for tracing get
persisted to the DB (`workflows.CreateLog`) and JSONL for every future task
forever, and get rendered in the UI's log console. After confirming a fix
with them, check what fraction of a task's log is your own debug noise
before you consider the job done:

```bash
python3 -c "
import json
lines = [json.loads(l) for l in open('server/.data/logs/<task-id>.jsonl') if l.strip()]
debug = sum(1 for d in lines if d['level']=='debug')
print(f'{debug}/{len(lines)} debug lines')
"
```

Remove anything above a handful of lines per run unless it guards a rare
condition worth keeping permanently (e.g. the branch-reconciliation warning
in `wkspace/state.go` — fires only on an actual mismatch, so it stays).

## 8. Sanity-check odd log content before chasing it

Not everything you see when reading a log file through a tool is actually
*in* the file. Compressed/truncated markers your own tooling inserts when
displaying large output (e.g. `<<ccr:hash,size>>`) can look like an
application bug. Before investigating, grep the raw file directly:

```bash
grep -c "ccr:" server/.data/logs/<task-id>.jsonl
```

If it's zero, the marker was never persisted — it's an artifact of how the
content was displayed to you, not a bug in this codebase.
