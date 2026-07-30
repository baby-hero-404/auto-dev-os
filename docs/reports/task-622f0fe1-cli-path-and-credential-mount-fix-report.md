# Trace Report: CLI Sandbox Path Leak + Credential Mount Permission Denied (task `622f0fe1-c70c-45e4-912c-294fa741c577`)

**Task:** "code" — Tiny VN OCR implementation, `execution_engine: cli` (Antigravity CLI / `agy`)
**Status at trace time:** paused for human spec review (`cli_spec` succeeded, not failed)
**Data sources:**
- `server/.data/logs/622f0fe1-c70c-45e4-912c-294fa741c577.jsonl`
- `server/.data/workspaces/622f0fe1-c70c-45e4-912c-294fa741c577/{task.json, docs/openspecs/code-622f0fe1/*, code/repos/tiny-vn-ocr/main/}`
- Raw `agy` process stdout/stderr supplied by user (not persisted to the workflow log; the orchestrator only logs `success`/`exit_code`/`output_bytes`, not subprocess stderr lines)
- `docs/features/product/07-task-system.md`

## 1. Summary

Two independent, real platform bugs surfaced from the same task run — neither is a crash, so the workflow log alone (which only shows `success=true`) didn't reveal either:

1. **Path leak:** the CLI agent's `design.md`/`tasks.md` embed the literal sandbox path `code/repos/tiny-vn-ocr/main/` as if it were the repo's own root, instead of clean repo-relative paths. Left unfixed, `cli_implement` (which resolves to the same container cwd) would create doubly-nested directories when the agent follows those instructions literally.
2. **Credential mount permission denied:** `agy`'s own stdout showed repeated `mkdir /home/agent/.gemini/antigravity-cli/{brain,conversations}: permission denied`, non-fatal here but likely dropping cross-step (`cli_analyze` → `cli_spec`) conversation/memory continuity.

## 2. Root Cause 1 — Path Leak

`cliStepRunner.RunCLIStep` (`server/internal/orchestrator/cli_spec_step.go`) sets `ContainerWorkDir` via `containerPathForHostPath(task, repoHostPath, "")`, which calls `paths.ContainerPathForHostPath` (`server/pkg/paths/helpers.go`). That function computes the container cwd as `/workspace` + `Rel(taskWorkspaceRoot, repoHostPath)`. Since the whole task workspace (not just the repo) is mounted at `/workspace`, and `repoHostPath` is `.../code/repos/tiny-vn-ocr/main`, the CLI agent's actual working directory inside the container is literally `/workspace/code/repos/tiny-vn-ocr/main` — a real, observable path, not a hallucination. The agent echoed it into the spec it authored.

This only affects CLI-mode. API-native mode never has this problem because `WorkspaceToRepoRelative`/`CleanPatchPaths` (see `07-task-system.md` §"Path Standardisation & Context Sanitisation") strip the sandbox prefix before any path reaches the LLM — but that sanitisation is invoked only in the API-native prompt/context assembly path, never in `cli_analyze.go`/`cli_spec.go`/`cli_implement.go`.

**Fix:** added `cliWorkingDirNotice` (`server/internal/orchestrator/steps/services.go`), appended to every CLI spec-first step's instruction, telling the agent its cwd already is the repo root and to never re-prefix paths with anything resembling `code/repos/<name>/...`.

## 3. Root Cause 2 — Credential Mount Read-Only

`DockerRuntime.Run` (`server/internal/sandbox/docker.go`) bind-mounted the host's entire `~/.gemini` (and `~/.config/codex`) directory into the container **read-only**, to let the sandbox reuse the host's existing OAuth session. But `agy` also writes its own runtime state (`antigravity-cli/brain`, `antigravity-cli/conversations`) into that same tree on startup — which a read-only mount rejects outright.

**Fix:** split the mount table into `authDirFiles` (single credential files — unchanged, still read-only) and `authDirTrees` (whole CLI home dirs). Tree dirs are now copied into a throwaway per-run staging directory (`copyDirTree`, new helper) and mounted **read-write** from the copy — the CLI gets a writable home, the host's real credential directory is never touched, and the staging copy is discarded (`defer os.RemoveAll`) after the run.

## 4. Verification

`go build ./...`, `go vet ./...`, `gofmt -l` (touched files clean), `go test ./...` all pass, including new `TestCopyDirTree` (file/symlink fidelity + post-copy mkdir succeeds where a read-only host mount would reject it).

## 5. Not Done

- The already-paused task `622f0fe1-...` still has the bad spec on disk; the fix prevents recurrence but doesn't retroactively clean this task's `design.md`/`tasks.md`. Reject-and-regenerate (or manual edit) needed before resuming it.
- No live re-run against real Docker + `agy` credentials to confirm the `mkdir` errors are gone end-to-end (this environment doesn't have that available).
