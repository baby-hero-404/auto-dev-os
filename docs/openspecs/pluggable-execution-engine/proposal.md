# Proposal: Pluggable Execution Engine (Subprocess-CLI mode)

## Why

Auto Code OS hiện là **API-native** (tự giữ tool-loop trong `server/internal/orchestrator/llmrunner/toolloop.go`, gọi LLM trực tiếp qua `server/pkg/llm/`). Theo phân tích reference (`docs/references/README.md` §"Mô Hình Tích Hợp LLM"), Multica và ai-sdlc chứng minh mô hình **Subprocess-CLI** — spawn một CLI coding agent (Claude Code, Codex CLI, aider…) như subprocess trong worktree cô lập — mang lại:

- User dùng **subscription sẵn có** của CLI thay vì trả token API qua key riêng.
- Server chỉ điều phối + stream log, không phải bảo trì tool-loop cho path này.
- ai-sdlc's `SubagentSpawner` interface (3 implementations: shell-spawn, SDK, mock) là mẫu đã được kiểm chứng cho việc pluggable hóa.

Hiện tại không có lựa chọn nào ngoài API-native — đây là gap chiến lược user đã chọn ưu tiên (Roadmap P1.1).

## What Changes

### Issue 1: Engine abstraction (spawner interface)

- Thêm interface `ExecutionEngine` trong `server/internal/orchestrator/engine/` với 2 implementation:
  - `apiNativeEngine` — wrap hành vi hiện tại (delegate về llmrunner tool-loop), zero behavior change.
  - `cliEngine` — spawn generic CLI command trong sandbox container tại worktree của task.
- Engine được resolve per-task tại thời điểm worker nhận job (`orchestrator/worker.go`), không hard-code trong step.

### Issue 2: Generic CLI runner trong sandbox

- `cliEngine` chạy qua `sandbox.CommandRequest` (như `runSandboxStepInWorktree`, `orchestrator/sandbox.go:53`) với timeout dài hơn (configurable, default 30m) và network bridge bắt buộc (CLI cần gọi provider của nó).
- Command template per-project: `{command}` + `{args}` với placeholder `{prompt_file}`, `{workdir}` (ví dụ: `claude -p --output-format stream-json "$(cat {prompt_file})"`).
- Prompt được ghi thành file trong worktree (`.autocode/prompt.md`) thay vì truyền qua argv (tránh giới hạn độ dài + lộ secret qua process list).
- Preflight step: `command -v <cli>` trong container trước khi chạy; fail với message rõ ràng nếu CLI chưa cài trong image.
- Full stdout/stderr capture thành step logs; kết quả đánh giá bằng git diff của worktree (không parse output CLI).

### Issue 3: Settings model + API

- `Project`: thêm cột `execution_engine` (`api_native` | `cli`, default `api_native`) và `cli_engine_config` (jsonb: `{command, args, env, timeout_minutes}`) vào `server/pkg/models/project.go` + migration.
- `Task`: thêm `execution_engine` override (nullable — null = kế thừa project) vào `server/pkg/models/task.go` + `CreateTaskInput`.
- Env vars trong `cli_engine_config.env` được mã hóa/lưu như repo credentials hiện có; không bao giờ log giá trị.

### Issue 4: UI toggle

- Project settings (`web/src/components/projects/project-profile.tsx` + `web/src/lib/api/projects.ts`, `web/src/lib/types.ts`): section "Execution Engine" — radio API-native/CLI, khi chọn CLI hiện form command/args/env/timeout.
- Task creation dialog: dropdown "Engine" (Inherit from project / API-native / CLI), default Inherit.
- Task detail: badge hiển thị engine đã dùng cho task.

## Capabilities

### New Capabilities
- Chọn execution engine per-project với per-task override qua UI.
- Chạy generic CLI coding agent trong sandbox container tại worktree của task.
- Preflight validation cho CLI availability.

### Modified Capabilities
- `orchestrator/worker.go` resolve engine trước khi dispatch code steps.
- Project/Task model + create/update API nhận engine fields.

### Removed Capabilities
- Không có.

## Impact

| Area | Files Affected |
|------|----------------|
| Engine core (new) | `server/internal/orchestrator/engine/engine.go`, `api_native.go`, `cli.go`, `preflight.go` |
| Orchestrator | `server/internal/orchestrator/worker.go`, `sandbox.go`, `setup.go` |
| Models + migration | `server/pkg/models/project.go`, `task.go`, `server/internal/repository/migrations/*` |
| API | `server/internal/router/*` (project update, task create handlers) |
| Web | `web/src/lib/types.ts`, `web/src/lib/api/projects.ts`, `web/src/components/projects/project-profile.tsx`, task creation dialog, task detail badge |
