# Tasks: Pluggable Execution Engine

> Thứ tự thực hiện từ trên xuống. Mỗi nhóm hoàn thành kèm test trước khi sang nhóm sau.

## 1. Data model & migration (REQ-006)

- [x] 1.1 Migration: `projects.execution_engine` (default `api_native`), `projects.cli_engine_config` (jsonb `{}`), `tasks.execution_engine` (nullable)
- [x] 1.2 `server/pkg/models/project.go`: fields + `CLIEngineConfig` struct + validation helpers (`ValidExecutionEngines`)
- [x] 1.3 `server/pkg/models/task.go`: `ExecutionEngine *string` + `CreateTaskInput`
- [x] 1.4 Router handlers: project PATCH validate + env masking (`***` giữ giá trị cũ); task create nhận override (handlers decode directly into the input structs, so the new fields flow through automatically; validation/masking lives in the service layer)
- [x] 1.5 Tests: validation matrix (engine enum, command required khi cli, mask/unmask round-trip)

## 2. Engine package (REQ-001, REQ-002)

- [x] 2.1 `server/internal/orchestrator/engine/engine.go`: interface + `CodeStepRequest/Result` + `ResolveEngine(task, project) string`
- [x] 2.2 `api_native.go`: adapter delegate về llmrunner path hiện tại (passthrough qua `Delegate` func; call-site refactor để đi qua interface deferred to 4.2)
- [x] 2.3 Unit tests: resolution precedence (task override > project > default), api_native passthrough

## 3. CLI engine (REQ-003, REQ-004, REQ-005)

- [x] 3.1 `command -v` trong sandbox + config sanity; error message theo spec REQ-004 (in `cli.go` `Preflight`)
- [x] 3.1b Auth preflight: `auth_check_command` (timeout 30s) + spawn với `CI=1` stdin đóng + tests (REQ-004b)
- [x] 3.1c Loop-kill monitor: **frequency-in-window** (ring buffer 50, bất kỳ hash ≥10 → kill) + tests (`loop_detector.go`/`loop_detector_test.go`). Implemented as a post-hoc pass over the captured `CommandResult` output rather than true live-stream kill — `sandbox.Runtime.Run` is a single blocking call with no line-by-line callback, so early-kill mid-run would require extending the `sandbox.Runtime` interface (out of scope for this slice).
- [x] 3.2 `cli.go`: ghi prompt file (base64-encoded via a single `bash -lc` script, avoids argv/escaping limits) → render args (placeholder substitution `{prompt_file}`/`{workdir}`, `QuoteShellArg` từng arg) → spawn qua `sandbox.CommandRequest` (`Env` field already existed) → success judged by exit code + loop-kill, caller still does `git status --porcelain` diffing as before
- [x] 3.3 Dọn `.autocode/` trước checkpoint (script always `rm -rf` the `.autocode` dir after the CLI exits, success or failure, before returning)
- [x] 3.4 Secret redaction trên captured output trước khi lưu log (`redact.go`)
- [x] 3.5 Tests: placeholder rendering, timeout config, nonzero exit → fail với stderr, loop-kill, secret redaction, auth preflight CI=1/stdin

## 4. Worker integration (REQ-M01)

- [x] 4.1 Resolve engine once per job: `stepRunners()` (`step_registry.go`) calls `resolveCLIEngineRunner` once when building the step map for a job/resume, not per LLM call
- [x] 4.2 Code steps (`steps/code_backend.go`, `code_frontend.go`, `fix.go`) dispatch qua engine interface — implemented by swapping the `steps.LLMRunner` these three steps receive (`cli_engine_step.go`'s `cliEngineRunner`) rather than forking each step's control flow. This means `runPatchRetryLoop`'s patch/retry/targeted-test/checkpoint logic (~350 lines shared across the three steps) is reused unchanged for both engines — the CLI engine's result is surfaced as `{"parsed": {"summary": ...}}`, which is exactly what the loop's existing agentic-mode branch expects from a successful edit.
- [x] 4.3 Preflight chạy như bước đầu của code step đầu tiên khi engine=cli — `cliEngineRunner` runs `Preflight` once via `sync.Once` on the first `RunLLMStep` call, not on every patch-retry attempt
- [x] 4.4 Tests: `cli_engine_step_test.go` covers engine resolution (api_native → nil runner, task override → cli runner), an end-to-end `RunLLMStep` call against a mock sandbox runtime, and that preflight only runs once across repeated calls. (Scoped as an orchestrator-level unit test rather than a full pipeline run — the existing pipeline/checkpoint integration tests in this package already exercise `stepRunners()` end to end for the api_native path, and now exercise `codeStepLLM` selection without regression.)

## 5. Web UI (REQ-007)

- [x] 5.1 `types.ts` + `api/projects.ts`: fields mới (`ExecutionEngine`, `CLIEngineConfig`, `execution_engine`, `cli_engine_config`)
- [x] 5.2 Project settings section (`project-profile.tsx` & `cli-engine-config-form.tsx` with engine selector, command, args, timeout, auth check, allowNoop, env key-value editor with masked values)
- [x] 5.3 Task create dialog: Engine select (Inherit/API-native/CLI) (`create-task-panel.tsx`)
- [x] 5.4 Task detail: engine badge (`TaskTitleBlock.tsx`)
- [x] 5.5 Web build + lint pass

## 6. Docs & wrap-up

- [x] 6.1 Cập nhật `ARCHITECTURE.md` (engine layer, dependency map, models, migration #13)
- [x] 6.2 Sandbox image README: ghi chú cách cài CLI vào image (`docker/README.md`)
- [x] 6.3 Update status icons trong `specs.md` + roadmap (`docs/openspecs/ROADMAP-cli-execution-engine.md`)

## Docs sync

- [ ] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md
