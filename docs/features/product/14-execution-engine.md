---
sources:
  - "server/internal/orchestrator/engine/**"
  - "server/internal/orchestrator/steps/cli_analyze.go"
  - "server/internal/orchestrator/steps/cli_spec.go"
  - "server/internal/orchestrator/steps/cli_implement.go"
  - "server/internal/workflow/step.go"
  - "server/pkg/models/project.go"
  - "server/pkg/models/task.go"
  - "web/src/components/projects/cli-engine-config-form.tsx"
verified: 2026-07-23
---

# 14. Execution Engine (Pluggable API-Native / CLI)

**Status:** 🟢 Implemented
**Owner docs:** `docs/ARCHITECTURE.md`; `docs/features/product/08-workflow-engine.md` for DAG selection
**Code areas:** `server/internal/orchestrator/engine/` (`engine.go`, `api_native.go`, `cli.go`, `preflight.go`), `server/internal/orchestrator/worker.go`, `server/internal/orchestrator/sandbox.go`, `server/pkg/models/project.go`, `server/pkg/models/task.go`, `web/src/components/projects/cli-engine-config-form.tsx`
**Acceptance criteria:** Project/task can select `api_native` or `cli` execution engine; CLI engine spawns a configured coding-agent binary as a subprocess in the task worktree, preflight-checks its availability, and evaluates results by git diff instead of parsing stdout.

**Mục tiêu:** Cho phép Auto Code OS chạy task bằng một trong hai cơ chế thực thi: **API-native** (server tự giữ tool-loop, gọi LLM trực tiếp qua Gateway — §01) hoặc **CLI (Subprocess)** — spawn một CLI coding agent có sẵn của người dùng (Claude Code, Codex CLI, aider…) như tiến trình con trong worktree cô lập. Mục tiêu chính: cho phép user tận dụng subscription CLI sẵn có thay vì trả token qua API key riêng, đồng thời giảm gánh nặng bảo trì tool-loop cho path này.

---

## A. Engine Abstraction

Interface `ExecutionEngine` (`server/internal/orchestrator/engine/engine.go`) có 2 implementation, được resolve **per-task** tại thời điểm worker nhận job (`orchestrator/worker.go`) — không hard-code trong step:

| Engine | Hành vi |
|:-------|:--------|
| `apiNativeEngine` | Wrap hành vi hiện tại — delegate về `llmrunner` tool-loop. Zero behavior change so với trước khi có engine abstraction. |
| `cliEngine` | Spawn CLI command trong sandbox container tại worktree của task, dùng `sandbox.CommandRequest` (cùng cơ chế `runSandboxStepInWorktree`, `orchestrator/sandbox.go`). |

**CLI runner:**
- Command template per-project (`{command} {args}` với placeholder `{prompt_file}`, `{workdir}`, ví dụ `claude -p --output-format stream-json "$(cat {prompt_file})"`).
- Prompt được ghi thành file trong worktree (`.autocode/prompt.md`) — không truyền qua argv, tránh giới hạn độ dài và lộ secret qua process list.
- **Preflight step** (`preflight.go`): `command -v <cli>` trong container trước khi chạy; fail rõ ràng nếu CLI chưa cài trong image.
- Timeout dài hơn API-native (configurable, mặc định 30 phút) và network bridge bắt buộc (CLI cần tự gọi provider của nó).
- Full stdout/stderr được capture thành step logs; kết quả được đánh giá bằng **git diff của worktree**, không parse output CLI.

## B. Settings Model

| Model | Field | Ghi chú |
|:------|:------|:--------|
| `Project` | `execution_engine` | `api_native` \| `cli`, mặc định `api_native` |
| `Project` | `cli_engine_config` | jsonb: `{command, args, env, timeout_minutes}` |
| `Task` | `execution_engine` | Override nullable — null = kế thừa từ project |

Biến môi trường trong `cli_engine_config.env` được mã hoá/lưu như credential hiện có (§05) — không bao giờ ghi log giá trị.

**UI:** Project Settings có section "Execution Engine" (radio API-native/CLI, hiện form command/args/env/timeout khi chọn CLI — `cli-engine-config-form.tsx`). Task creation dialog có dropdown Engine (Inherit/API-native/CLI, mặc định Inherit). Task detail hiển thị badge engine đã dùng.

## C. CLI Spec-First Pipeline

Vì CLI agent đã tự có tool-loop, context loading, planning và self-review bên trong nó, DAG API-native (context_load → analyze → plan → code → merge → review → fix → test → pr) không phù hợp khi `execution_engine = cli`. Thay vào đó, `BuildWorkflow` chọn workflow definition thứ hai — `cli_spec_first` (`server/internal/workflow/step.go`) — theo engine đã resolve của task:

```
cli_analyze → cli_spec → cli_implement → cli_mr
```

| Step | Vai trò |
|:-----|:--------|
| **cli_analyze** | CLI được prompt phân tích repo + task description, ghi `.autocode/analysis.md` (tech stack, files liên quan, risks). Server đọc file này lưu vào `task.Analysis`. |
| **cli_spec** | CLI authoring một OpenSpec set vào `docs/openspecs/<task-slug>/` trong worktree (4 files theo đúng convention của chính Auto Code OS). Server parse `proposal.md` + `tasks.md` để hiển thị UI; gate approve (tuỳ autonomy setting của project) trước khi sang implement. |
| **cli_implement** | CLI được prompt implement theo spec set, tick checkbox trong `tasks.md` khi xong. Kết quả đánh giá bằng git diff. |
| **cli_mr** | Tái dùng PR step hiện có (`orchestrator/steps/pr.go`) — push branch + tạo PR; spec set nằm trong diff nên reviewer thấy cả spec lẫn code. |

Task detail có tab/panel "Spec" render `proposal.md` + checkbox `tasks.md` (đọc từ worktree qua endpoint riêng). Khi autonomy = supervised, nút Approve/Request-changes hiện trên spec trước khi `cli_implement` được dispatch.

---

**Dự án tham khảo:**

| Dự án | Lý do tham khảo |
|:------|:----------------|
| Multica | `SubagentSpawner` interface pattern cho pluggable hóa harness |
| ai-sdlc | Shell-spawn/SDK/mock spawner implementations; spec-first pipeline cho black-box agent |
| OpenSpec | Convention 4-file spec set (proposal/specs/design/tasks) dùng làm hợp đồng thực thi cho CLI agent |
