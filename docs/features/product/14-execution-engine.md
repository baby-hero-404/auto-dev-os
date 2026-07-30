---
sources:
  - "server/internal/orchestrator/engine/**"
  - "server/internal/orchestrator/steps/cli_analyze.go"
  - "server/internal/orchestrator/steps/cli_spec.go"
  - "server/internal/orchestrator/steps/cli_implement.go"
  - "server/internal/workflow/step.go"
  - "server/pkg/models/project.go"
  - "server/pkg/models/task.go"
  - "server/internal/service/credential_pool.go"
  - "web/src/components/projects/execution-providers-list.tsx"
  - "web/src/app/ai-providers/components/GlobalRoutingPanel.tsx"
verified: 2026-07-30
---

# 14. Execution Engine (Pluggable API-Native / CLI)

**Status:** 🟢 Implemented
**Owner docs:** `docs/ARCHITECTURE.md`; `docs/features/product/08-workflow-engine.md` for DAG selection
**Code areas:** `server/internal/orchestrator/engine/` (`engine.go`, `cli.go`, `preflight.go`) for the CLI engine implementation; API-native tool-loop lives in `server/internal/orchestrator/llmrunner/toolloop.go` driven per-step from `server/internal/orchestrator/steps/*.go`, `server/internal/orchestrator/worker.go`, `server/internal/orchestrator/execution_router.go`, `server/internal/orchestrator/sandbox.go`, `server/pkg/models/project.go`, `server/pkg/models/task.go`, `web/src/components/projects/execution-providers-list.tsx` (exports `CLIEngineConfigForm`)
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
- **Per-Run Auth Directory & Context Injection:** Thông tin đăng nhập CLI (sandbox CLI credentials) được staging vào một thư mục auth riêng biệt (per-run auth directory) trong mỗi lần chạy. Hệ thống cũng tự động inject các context file trực tiếp vào CLI runtime environment.
- **Artifact Versioning & Re-authentication:** Các artifact do CLI tạo ra được versioning theo từng attempt. Hỗ trợ theo dõi trạng thái re-authentication của credential.

## B. Settings Model

| Model | Field | Ghi chú |
|:------|:------|:--------|
| `Project` | `execution_engine` | `api_native` \| `cli`, mặc định `api_native` |
| `Project` | `cli_engine_config` | jsonb: `{command, args, env, timeout_minutes}` |
| `Task` | `execution_engine` | Override nullable — null = kế thừa từ project |

Biến môi trường trong `cli_engine_config.env` được mã hoá/lưu như credential hiện có (§05) — không bao giờ ghi log giá trị.

**UI:** Project Settings có section "Execution Engine" (radio API-native/CLI, hiện form command/args/env/timeout khi chọn CLI — `CLIEngineConfigForm` trong `execution-providers-list.tsx`). Task creation dialog không có selector riêng — task luôn kế thừa routing từ `Project.execution_providers`/`execution_engine` (xem B2). Task detail hiển thị badge engine đã dùng.

## B2. Execution Provider Routing (ưu tiên hơn Execution Engine)

`Project.execution_providers` (jsonb, `[]ExecutionProviderConfig`) thay thế field nhị phân `execution_engine` bằng một danh sách provider ưu tiên (`{type: api|cli, ref, credential_id?, priority, enabled, cli_config?}`). `Orchestrator.ResolveExecutionProvider` (`server/internal/orchestrator/execution_router.go`) chọn candidate `enabled=true` có priority thấp nhất và credential còn khả dụng (không `cooldown`/`rate_limited`); nếu danh sách rỗng, fallback **byte-identical** về `execution_engine`/`cli_engine_config` cũ (không breaking change cho project chưa migrate). Việc resolve diễn ra **một lần khi Task bắt đầu** — không failover giữa chừng task đang chạy.

`ref` cho `type=cli` là một trong `claude_code | openai_codex | antigravity | custom`, ánh xạ tới `CLIProfile` registry built-in (`server/pkg/models/cli_profiles.go`) — chỉ `custom` mới cần `cli_config` inline và **bắt buộc** `credential_id` (không có provider cố định để auto-map).

**Quota detection (write-side):** Do CLI chạy blocking trong sandbox, phát hiện quota/rate-limit dựa trên pattern-matching stdout+stderr đã capture sau khi tiến trình kết thúc (`server/internal/orchestrator/engine/cli_quota.go`, bảng `CLIQuotaRules` keyed theo `ProfileRef`, có fallback `"*"`). Khi khớp, `CredentialPoolService.SetCooldown` ghi `cooldown_until` (mặc định 1 phút) lên `ProviderCredential` tương ứng, để lần resolve tiếp theo tự động fallthrough qua candidate priority thấp hơn.

**UI:** Project Settings có thêm section "Execution Providers" (`execution-providers-list.tsx`) — danh sách cố định 7 hàng (Anthropic/OpenAI/Gemini API + Claude Code/OpenAI Codex/Antigravity/Custom CLI), mỗi hàng có checkbox Enabled, nút ▲/▼ đổi priority, và dropdown "CLI Authentication Profile" (bắt buộc cho Custom CLI, mặc định "Auto" cho 3 preset CLI). Nếu org chưa có credential `cli:*` nào, dropdown hiện link "Authenticate a CLI provider" trỏ sang trang AI Providers. Credential đang cooldown (do quota exceeded) được đánh dấu "— on cooldown" trong dropdown, và nếu credential đang chọn đang cooldown thì hiện cảnh báo kèm thời điểm hết cooldown (`cooldown_until`).

**Global Routing (`Organization.default_execution_providers`):** cùng cấu trúc/UI với Execution Providers (`GlobalRoutingPanel.tsx` trên trang AI Providers, admin-only), nhưng là fallback priority list ở **org-wide**, dùng cho project chưa tự cấu hình `execution_providers` riêng (xem thứ tự ưu tiên ở `execution_router.go`). **Auto-enable:** mỗi khi org thêm một `ProviderCredential` mới cho Anthropic/OpenAI/Gemini/Claude CLI/Codex CLI/Antigravity CLI, `CredentialPoolService.Create` (`server/internal/service/credential_pool.go`) tự động bật `enabled=true` cho row tương ứng trong `default_execution_providers` — không cần admin vào Global Routing bấm Save thủ công. Priority của row giữ nguyên vị trí cố định sẵn có (không reorder); nếu org chưa từng lưu Global Routing lần nào, credential đầu tiên sẽ tự scaffold đủ 7 row (theo đúng thứ tự UI) rồi chỉ bật row đó. Custom CLI (`ref=custom`) không bao giờ được auto-enable vì cần `command`/`credential_id` cấu hình tay (`models.AutoEnableExecutionProviderRow`, `server/pkg/models/project.go`).

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
