# Tasks: Pluggable Execution Engine

> Thứ tự thực hiện từ trên xuống. Mỗi nhóm hoàn thành kèm test trước khi sang nhóm sau.

## 1. Data model & migration (REQ-006)

- [ ] 1.1 Migration: `projects.execution_engine` (default `api_native`), `projects.cli_engine_config` (jsonb `{}`), `tasks.execution_engine` (nullable)
- [ ] 1.2 `server/pkg/models/project.go`: fields + `CLIEngineConfig` struct + validation helpers (`ValidExecutionEngines`)
- [ ] 1.3 `server/pkg/models/task.go`: `ExecutionEngine *string` + `CreateTaskInput`
- [ ] 1.4 Router handlers: project PATCH validate + env masking (`***` giữ giá trị cũ); task create nhận override
- [ ] 1.5 Tests: validation matrix (engine enum, command required khi cli, mask/unmask round-trip)

## 2. Engine package (REQ-001, REQ-002)

- [ ] 2.1 `server/internal/orchestrator/engine/engine.go`: interface + `CodeStepRequest/Result` + `ResolveEngine(task, project) string`
- [ ] 2.2 `api_native.go`: adapter delegate về llmrunner path hiện tại (refactor call-site trong steps để đi qua interface, hành vi không đổi)
- [ ] 2.3 Unit tests: resolution precedence (task override > project > default), api_native passthrough

## 3. CLI engine (REQ-003, REQ-004, REQ-005)

- [ ] 3.1 `preflight.go`: `command -v` trong sandbox + config sanity; error message theo spec REQ-004
- [ ] 3.1b Auth preflight: `auth_check_command` (timeout 30s) + spawn với `CI=1` stdin đóng + tests (REQ-004b)
- [ ] 3.1c Loop-kill monitor trên stream log: normalize error lines (strip số/hex/path → hash), **frequency-in-window** (ring buffer 50, bất kỳ hash ≥10 → kill) + tests: multi-line stack trace loop `ABCDE×N` bị bắt, 9 lần rồi thoát không kill (REQ-004c)
- [ ] 3.2 `cli.go`: ghi prompt file → render args (placeholder substitution, `QuoteShellArg` từng arg) → spawn qua `sandbox.CommandRequest` (mở rộng `Env` field nếu chưa có) → `git status --porcelain` → `HasChanges`
- [ ] 3.3 Dọn `.autocode/` trước checkpoint (exclude khỏi `git add -A`)
- [ ] 3.4 Secret redaction trên captured output trước khi lưu log
- [ ] 3.5 Tests: placeholder rendering, timeout, no-changes → fail, nonzero exit → fail với stderr tail, env không lộ trong log

## 4. Worker integration (REQ-M01)

- [ ] 4.1 `orchestrator/worker.go`: resolve engine 1 lần khi claim job, gắn vào job context
- [ ] 4.2 Code steps (`steps/code_backend.go`, `code_frontend.go`, `fix.go`) dispatch qua engine interface
- [ ] 4.3 Preflight chạy như bước đầu của code step đầu tiên khi engine=cli
- [ ] 4.4 Integration test: task với engine=cli dùng MockEngine (mẫu `MockSpawner` của ai-sdlc) chạy hết pipeline

## 5. Web UI (REQ-007)

- [ ] 5.1 `types.ts` + `api/projects.ts`: fields mới
- [ ] 5.2 Project settings section (radio + CLI config form, env key-value editor với masked values)
- [ ] 5.3 Task create dialog: Engine select (Inherit/API-native/CLI)
- [ ] 5.4 Task detail: engine badge
- [ ] 5.5 Web build + lint pass

## 6. Docs & wrap-up

- [ ] 6.1 Cập nhật `ARCHITECTURE.md` (engine layer + dependency map)
- [ ] 6.2 Sandbox image README: ghi chú cách cài CLI vào image (ví dụ Claude Code)
- [ ] 6.3 Update status icons trong `specs.md` + roadmap
