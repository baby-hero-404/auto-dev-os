---
sources:
  - "server/internal/orchestrator/engine/**"
  - "server/internal/orchestrator/steps/cli_analyze.go"
  - "server/internal/orchestrator/steps/cli_spec.go"
  - "server/internal/orchestrator/steps/cli_implement.go"
  - "server/internal/orchestrator/steps/cli_implement_track.go"
  - "server/internal/orchestrator/steps/cross_review.go"
  - "server/internal/mcpcontext/**"
  - "server/cmd/mcp-context/main.go"
  - "server/internal/sandbox/activity.go"
  - "server/internal/sandbox/docker.go"
  - "server/internal/sandbox/sandbox.go"
  - "server/internal/workflow/step.go"
  - "server/pkg/models/project.go"
  - "server/pkg/models/task.go"
  - "server/pkg/models/cli_profiles.go"
  - "server/internal/service/credential_pool.go"
  - "web/src/components/projects/execution-providers-list.tsx"
  - "web/src/app/ai-providers/components/GlobalRoutingPanel.tsx"
verified: 2026-08-04
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

## C. CLI Orchestrator Pipeline

Vì CLI agent đã tự có tool-loop, context loading, planning và self-review bên trong nó, DAG API-native không phù hợp khi `execution_engine = cli`. `BuildWorkflow` chọn một trong hai DAG definitions dựa trên kết quả phân tích của `cli_analyze` (populated qua `workflow.ErrGraphChanged` re-dispatch):

**Single-track:**
```
cli_analyze → cli_spec → cli_implement → cross_review → cli_mr
```

**Parallel-track (FE+BE detected by `CLIAnalysisNeedsParallelTracks`):**
```
cli_analyze → cli_spec → cli_implement_backend ┐
                        → cli_implement_frontend ┘ → merge → cross_review → cli_mr
```

| Step | Vai trò |
|:-----|:--------|
| **cli_analyze** | CLI phân tích repo + task description, ghi `.autocode/analysis.md` (captured qua `CaptureFiles` sentinel encoding). Xác định single- vs dual-track. |
| **cli_spec** | CLI authoring 4 OpenSpec files vào `docs/openspecs/<task-slug>/`. Human approval gate theo autonomy setting. |
| **cli_implement (/ _backend / _frontend)** | CLI implement theo spec. Role-specific prompt qua `PromptBuilder.LoadRolePrompt` (cùng `AgentRole` resolution với API-native flow). BE/FE tracks chạy trong worktree riêng (`feature/<task-id>-be` / `-fe`). |
| **cross_review** | LLM-based reviewer (`AgentRoleReviewer`) độc lập harness (Harness Independence — loại trừ provider của coder). Fail → `ErrCrossReviewFixLoop` re-dispatch `cli_implement` với violations prepended. Cycle limit + repeat-violation → escalate `human_review`. |
| **merge** | Reuse `MergeStep` để merge `be`/`fe` branches vào integration branch. |
| **cli_mr** | Tái dùng `PRStep` push/PR logic verbatim. |

**Context MCP Server (`server/cmd/mcp-context`):**
Stdio-only MCP server bundled trong sandbox image. Expose 6 tools — `repo.search`, `ast.query`, `dependency.impact`, `skill.search`, `architecture.query`, `quality.check` — là thin wrappers over `internal/context/*`. Auto-wired per CLI profile (`claude --mcp-config`, Codex `config.toml`, Antigravity `mcp_config.json`). Dynamic context invalidation: `Provider.IndexAll` (mtime-respecting) đảm bảo mid-session edits được reflect ngay.

**Observability:**
- Real-time log streaming (`Follow:true` concurrent với `ContainerWait`), logs → `server/.data/workspaces/<task-id>/logs/cli_<step_id>_run.log`.
- MCP server: `mcp-server.log` + `mcp-trace.jsonl` (JSON-RPC request/response pairs per round-trip, gated bởi `--trace` flag hoặc `AUTOCODE_MCP_TRACE=1`).
- Telemetry (cost/duration/tokens) parse từ `--output-format json` CLI output, accumulated vào `workflow_jobs.total_cost_usd`/`total_duration_ms`/`total_tokens_used` (migration 000023, additive `gorm.Expr` update).

**Resilience & Security:**
- Smart idle timeout (15 min default, `AUTOCODE_CLI_IDLE_TIMEOUT_MINUTES`) + streaming loop detector (200-line ring buffer, 10× repetition) → `ContainerKill(SIGKILL)` ngay khi phát hiện.
- Sandbox cache mounts fixed sang `/home/agent/` (non-root UID 1000).

## D. Sandbox Manager v2 (Per-Runtime Container Orchestration)

Sandbox Manager v2 (`server/internal/sandbox/`) nâng cấp cơ chế Sandbox từ hình thức Docker Image đơn khối (monolith) lên mô hình Orchestration đa môi trường theo runtime dự án:

1. **Auto-Detection (`detector.go`):** Đọc các marker files trong repo (`package.json` → Node, `requirements.txt`/`pyproject.toml` → Python, `go.mod` → Go, `pom.xml` → Java, `pubspec.yaml` → Flutter) để tự động nhận diện Runtime môi trường mà không cần khởi động container.
2. **Dynamic Registry & Manifests (`registry.go`):** Nạp cấu hình từ các file `manifest.yaml` trong `server/internal/sandbox/runtimes/*/manifest.yaml`. Mỗi manifest định nghĩa:
   - `image`: Docker Image tương ứng (vd. `autocode/node:latest`, `autocode/go:latest`, `autocode/python:latest`).
   - `cache`: Danh sách bind-mounts đệm toàn cục (`~/.npm`, `~/.cache/pip`, `~/go/pkg/mod`, `~/.pub-cache`).
   - `setup` & `healthcheck`: Các lệnh chuẩn bị môi trường và kiểm tra sức khỏe container trước khi giao việc cho CLI Agent.
3. **Multi-Version Best Practices:**
   - **Go Runtime:** Thiết lập `ENV GOTOOLCHAIN=auto` cho phép Go SDK tự động tải và chuyển đổi đúng phiên bản Go mà `go.mod` yêu cầu.
   - **Python Runtime:** Tích hợp `uv` giúp khởi tạo virtualenvs và quản lý dependency siêu tốc.
   - **Layered Dockerfile Build:** Quản lý qua target `make sandbox-images` trong `Makefile`.
- Credentials injected qua `container.Config.Env`, không bao giờ interpolated vào `cmd.Args`. `redactSecrets` scrub toàn bộ log/checkpoint output (4 credential shapes: Anthropic/OpenAI/Gemini/GitHub).
- Context cancellation: `ContainerKill(SIGKILL)` ngay lập tức khi job bị cancel, không cần chờ 5s SIGTERM grace.
- Partial retry: backend track checkpoint survive → chỉ frontend track bị re-run (Phase 5 checkpoint resume).

---

**Dự án tham khảo:**

| Dự án | Lý do tham khảo |
|:------|:----------------|
| Multica | `SubagentSpawner` interface pattern cho pluggable hóa harness |
| ai-sdlc | Shell-spawn/SDK/mock spawner implementations; spec-first pipeline cho black-box agent |
| OpenSpec | Convention 4-file spec set (proposal/specs/design/tasks) dùng làm hợp đồng thực thi cho CLI agent |
