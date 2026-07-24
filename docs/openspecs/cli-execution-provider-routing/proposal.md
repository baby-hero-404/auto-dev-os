# Proposal: CLI Execution Provider Routing (thay thế CLI Engine Config đơn lẻ)

> **Supersedes**: `docs/openspecs/cli-engine-preset-setup/` (đã xoá — chưa implement dòng nào, toàn bộ REQ ở status "Not Started"). OpenSpec đó chỉ giải quyết "UI form đẹp hơn cho 1 config" — sau khi đối chiếu kiến trúc, bài toán thật là "Orchestrator chọn runtime nào cho Task", một tầng sâu hơn.

## Why

Form `CLIEngineConfigForm` (`web/src/components/projects/cli-engine-config-form.tsx`) phơi implementation detail (`Command`, `Args` với placeholder `{prompt_file}`, `Auth Check Command`, `Env`, `Timeout`) ra Project Settings. Nguyên nhân gốc không phải "UI xấu" — mà là **data model sai tầng**: `Project.ExecutionEngine` (`server/pkg/models/project.go:88`) là lựa chọn nhị phân `api_native | cli`, và `Project.CLIEngineConfig` chỉ giữ được **đúng 1** cấu hình CLI. Project buộc phải "biết" cách Claude Code/Codex hoạt động vì đó là nơi duy nhất lưu command/args.

Hệ quả kiến trúc, không chỉ UI:
- Không thể enable đồng thời nhiều provider (Claude Code + Codex + Claude API) cho 1 project.
- Không có priority/fallback — khi CLI hết quota, Task fail cứng, không tự chuyển sang provider khác.
- `web/src/app/ai-providers/components/ModelRoutingRules.tsx:XX` cố tình lọc `!provider.startsWith("cli:")` — CLI provider chưa bao giờ tham gia được cơ chế priority/is_active mà API provider đã có sẵn qua `ProviderModel` (`server/pkg/models/provider_model.go`) + `CredentialPoolService` cooldown (`server/internal/service/credential_pool.go`).

Đối chiếu 3 project reference đã khảo sát (`docs/references/infrastructure/DISCOVERY-llm-key-manager.md`, `DISCOVERY-9router.md`, `DISCOVERY-free-claude-code.md`): cả 3 đều tách rõ **Provider Profile** (data tĩnh: command/args/base-url — hiếm đổi) khỏi **Routing/Availability State** (runtime: priority, cooldown, enabled — đổi liên tục). Auto Code OS hiện gộp cả hai vào `CLIEngineConfig`, đây chính là lỗi thiết kế cần sửa.

Điểm chèn kiến trúc duy nhất hiện có cho vấn đề này đã tồn tại sẵn và rất hẹp:
- `engine.ResolveEngine(taskEngine, projectEngine)` (`server/internal/orchestrator/engine/engine.go:94`) — trả về 1 string nhị phân.
- `Orchestrator.resolveCLIEngineRunner(ctx, task)` (`server/internal/orchestrator/cli_engine_step.go:143`) — đọc đúng 1 `CLIEngineConfig`.
- Gọi bởi `step_registry.go:26` và `worker.go:309`.

Đây là **2 hàm, 2 điểm gọi** — thay thế chúng bằng một "Execution Router" không đòi hỏi viết lại `CLIEngineStep`/`LLMStep`/patch-retry-loop.

## What Changes

### Issue 1: CLI Profile registry — system-level, built-in (không phải bảng DB)
- File mới `server/pkg/models/cli_profiles.go`: `CLIProfile{Command, Args, AuthCheckCommand, TimeoutMinutes, CredentialProvider}`, map `CLIProfiles map[string]CLIProfile` với key `claude_code | openai_codex | antigravity`. Tương đương registry đã thiết kế ở OpenSpec cũ (Issue 1), giữ nguyên giá trị đã verify.
- File mới `web/src/lib/cliProfiles.ts` — mirror TS, cùng key ID.
- **Quyết định (đã chốt cùng user)**: KHÔNG có CRUD/DB cho CLI Profile ở phase này — org không tự thêm CLI tool mới ngoài 3 cái built-in. `custom` vẫn tồn tại như 1 loại `ExecutionProviderConfig{type:"cli", ref:"custom"}` mang theo command/args inline (giữ đường thoát cho advanced user, không xoá khả năng hiện tại).

### Issue 2: `Project.ExecutionProviders` — danh sách có priority, thay cho `ExecutionEngine` nhị phân
- Field mới trên `Project`: `ExecutionProviders json.RawMessage` (jsonb, cột mới — **có migration**, khác OpenSpec cũ) chứa `[]ExecutionProviderConfig`:
  ```go
  type ExecutionProviderConfig struct {
      Type         string `json:"type"`                    // "api" | "cli"
      Ref          string `json:"ref"`                      // api: provider id (openai/anthropic/gemini); cli: profile key (claude_code/openai_codex/antigravity/custom)
      CredentialID string `json:"credential_id,omitempty"`  // optional pin; empty = let CredentialPoolService pick
      Priority     int    `json:"priority"`
      Enabled      bool   `json:"enabled"`
      CLIConfig    *CLIEngineConfig `json:"cli_config,omitempty"` // only when ref=="custom"
  }
  ```
- **Backward compat bắt buộc**: nếu `ExecutionProviders` rỗng (project cũ), Execution Router fallback nguyên xi hành vi hiện tại (`ExecutionEngine` + `CLIEngineConfig` cũ) — không migration data, không breaking change cho project đang chạy.
- `ExecutionEngine`/`CLIEngineConfig` cột cũ **giữ nguyên, không xoá** trong phase này (dùng làm fallback path); dọn dẹp là follow-up riêng sau khi `ExecutionProviders` đã chứng minh ổn định.

### Issue 3: Execution Router — thay `ResolveEngine` + `resolveCLIEngineRunner`
- Hàm mới `ResolveExecutionProvider(ctx, task, project) (*ResolvedExecutionProvider, error)` trong `server/internal/orchestrator/execution_router.go`:
  - Đọc `project.ExecutionProviders`, lọc `Enabled == true`, sort theo `Priority` tăng dần.
  - Với `type == "api"`: kiểm tra có `ProviderModel` active nào khớp `Ref` không (tái dùng `ProviderModelRepo`), và **không** có credential nào của provider đó đang `status == rate_limited`/trong cooldown toàn phần — nếu còn ít nhất 1 credential khả dụng thì coi là chọn được (không cần Router tự chọn credential, `CredentialPoolService` đã làm việc đó ở tầng LLM call).
  - Với `type == "cli"`: kiểm tra credential (`CredentialID` chỉ định, hoặc credential `cli:<ref>` đầu tiên khả dụng) có `status == active` — CLI dùng `ProviderCredential.Status`/`CooldownUntil` y hệt API (đã có sẵn field, chỉ chưa được đọc bởi CLI trước đây).
  - **Write-side (mới, xem design.md "CLI quota detection")**: vì CLI subprocess không trả HTTP status code và `Runtime.Run` là blocking call không streaming, output chỉ đọc được post-hoc sau khi process thoát — `RunCodeStep` chạy 1 bảng pattern cấu hình (`CLIQuotaRules`, theo từng CLI) để phát hiện quota/rate-limit trong output đã capture, rồi gọi `CooldownSetter.SetCooldown` cho đúng credential vừa dùng. Đây là phần trước đây field `Status`/`CooldownUntil` tồn tại nhưng chưa ai ghi cho CLI — REQ-006 giờ cover cả 2 chiều đọc/ghi.
  - Trả về **candidate đầu tiên khả dụng**; không tìm được candidate nào → lỗi rõ ràng `"no enabled execution provider is available"` (không còn silent fallback về api_native).
  - **Phạm vi đã chốt**: chọn 1 lần lúc Task bắt đầu (không mid-task switch). Nếu candidate đã chọn fail giữa chừng, Task fail — khi user bấm Retry, Router chạy lại và tự nhiên né candidate vừa cooldown.
- `resolveCLIEngineRunner` sửa thành gọi `ResolveExecutionProvider`; nếu kết quả `type == "cli"`, build `cliEngineRunner` từ `CLIProfiles[ref]` (hoặc `CLIConfig` inline nếu `ref == "custom"`) thay vì đọc thẳng `project.CLIEngineConfig`. `step_registry.go:26` và `worker.go:309` gọi qua đúng 1 điểm thay đổi này — không sửa `CLIEngineStep`, `LLMStep`, hay patch-retry-loop.

### Issue 4: UI Project Settings — "Execution Providers" thay cho CLI Engine form
- `web/src/components/projects/cli-engine-config-form.tsx` (form cũ, phơi command/args) → thay bằng `execution-providers-list.tsx`: mỗi provider (Claude API/OpenAI API/Gemini API/Claude Code/OpenAI Codex/Antigravity/Custom CLI) là 1 dòng với checkbox Enabled + ▲▼ Priority — cùng pattern UX với `ModelRoutingRules.tsx`, không phải phát minh UI mới.
- Field command/args/env chỉ hiện khi user chọn "Custom CLI" (giữ nguyên form raw hiện có cho case này, không đổi).
- Dòng "Custom CLI" có thêm dropdown "CLI Authentication Profile" **bắt buộc chọn** (không có `CredentialProvider` cố định để tự map như 3 preset) — dòng preset có dropdown tương tự nhưng **optional** (mặc định "Auto"), ghi vào `ExecutionProviderConfig.credential_id` (field đã tồn tại từ Issue 2, chỉ thiếu UI để set nó).
- `ModelRoutingRules.tsx` bỏ điều kiện lọc `!provider.startsWith("cli:")` là **hệ quả tự nhiên**, không phải việc cần làm riêng — CLI provider giờ tham gia đúng vòng đời `ProviderCredential` (status/cooldown) như API.

## Capabilities

### New Capabilities
- `CLIProfiles` registry (system-level, Go + TS, không DB).
- `Project.ExecutionProviders` — danh sách provider có priority/enabled, thay thế lựa chọn nhị phân.
- `ResolveExecutionProvider` — Execution Router chọn provider khả dụng đầu tiên theo priority, tái dùng `ProviderCredential.Status`/`CooldownUntil` cho cả API lẫn CLI.
- UI "Execution Providers" list trên Project Settings.

### Modified Capabilities
- `resolveCLIEngineRunner`: đọc qua Router thay vì đọc thẳng `CLIEngineConfig`.
- CLI credential (`cli:*`) giờ tham gia cooldown/status check giống API credential (trước đây field tồn tại nhưng không được đọc cho CLI).

### Removed Capabilities
- Không có gì bị xoá cứng ở phase này — `ExecutionEngine`/`CLIEngineConfig` giữ làm fallback, dọn dẹp là follow-up.

## Impact

| Area | Files Affected |
|------|----------------|
| Backend models | `server/pkg/models/cli_profiles.go` (mới), `server/pkg/models/project.go` (thêm `ExecutionProviders`, `ExecutionProviderConfig`) |
| Backend migration | `server/migration/` (mới — cột `execution_providers jsonb` trên `projects`, **migration thật**, khác OpenSpec cũ) |
| Backend orchestrator | `server/internal/orchestrator/execution_router.go` (mới), `server/internal/orchestrator/cli_engine_step.go` (sửa `resolveCLIEngineRunner`, `RunLLMStep` gọi cooldown), `server/internal/orchestrator/engine/cli.go` (gọi `detectQuotaExceeded`, field mới trên `CodeStepResult`/`CLIEngineConfig`), `server/internal/orchestrator/engine/cli_quota.go` (mới — `CLIQuotaRules`) |
| Backend tests | `server/pkg/models/cli_profiles_test.go`, `server/internal/orchestrator/execution_router_test.go`, `server/internal/orchestrator/engine/cli_quota_test.go` (mới) |
| Frontend types | `web/src/lib/types.ts` (thêm `ExecutionProviderConfig`, `execution_providers?`) |
| Frontend registry | `web/src/lib/cliProfiles.ts` (mới) |
| Frontend UI | `web/src/components/projects/execution-providers-list.tsx` (mới, thay `cli-engine-config-form.tsx`), `web/src/components/projects/project-profile.tsx` |
| Frontend UI (hệ quả) | `web/src/app/ai-providers/components/ModelRoutingRules.tsx` (bỏ filter `cli:`) |
| Docs | `docs/features/` (Project Settings / Execution Providers) |
