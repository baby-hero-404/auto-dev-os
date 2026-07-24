# Specs: CLI Execution Provider Routing

## Added Requirements

### REQ-001: CLI Profile registry (system-level, built-in)
> ❌ Status: Not Started

**Scenario:**
- WHEN `models.CLIProfiles["claude_code"]` được đọc
- THEN nó trả về `Command="claude"`, `Args=["-p", "--dangerously-skip-permissions", "{prompt_file}"]`, `AuthCheckCommand="claude --version"`, `TimeoutMinutes=30`, `CredentialProvider="cli:claude"`
- AND `models.CLIProfiles["openai_codex"]`, `["antigravity"]` tồn tại với giá trị đã verify thủ công (kế thừa từ OpenSpec cũ, không tự suy đoán lại)

**Scenario: Key không tồn tại**
- WHEN `models.CLIProfiles["not_a_real_key"]` được truy cập
- THEN trả `(zero value, false)` qua hàm `ProfileOrEmpty(key string) (CLIProfile, bool)` — không panic

### REQ-002: `Project.ExecutionProviders` — field mới, có migration, jsonb
> ❌ Status: Not Started

**Scenario:**
- WHEN client gửi `PUT /projects/:id` với `execution_providers = [{"type":"cli","ref":"claude_code","priority":1,"enabled":true},{"type":"api","ref":"anthropic","priority":2,"enabled":true}]`
- THEN `Project.ExecutionProviders` (cột `jsonb`, migration mới thêm cột) lưu đúng mảng
- AND `GET /projects/:id` trả về đúng thứ tự `priority` đã lưu (không tự sắp xếp lại trong storage layer — sort chỉ xảy ra ở Router)

**Scenario: Validate item không hợp lệ**
- WHEN 1 phần tử trong `execution_providers` có `type` khác `"api"`/`"cli"`, hoặc `type=="cli"` mà `ref` không thuộc `{claude_code, openai_codex, antigravity, custom}`, hoặc `ref=="custom"` mà thiếu `cli_config.command`
- THEN API trả lỗi 400 rõ ràng chỉ đúng phần tử/field sai (không silently drop)

### REQ-003: Backward compat — project cũ (chưa có `execution_providers`) chạy y hệt hành vi hôm nay
> ❌ Status: Not Started

**Scenario:**
- WHEN 1 project được tạo trước tính năng này, có `execution_engine="cli"`, `cli_engine_config={"command":"claude",...}`, và `execution_providers` rỗng/không tồn tại
- THEN `ResolveExecutionProvider` trả về đúng candidate tương đương config cũ (`type:"cli", ref:"custom", cli_config: <cli_engine_config cũ>`) — không yêu cầu user migrate thủ công, không cần chạy script backfill

**Scenario: Project mới hoàn toàn, chưa từng cấu hình gì**
- WHEN project có `execution_engine="api_native"` (default), `execution_providers` rỗng
- THEN `ResolveExecutionProvider` trả về candidate mặc định tương đương hành vi `api_native` hiện tại (không đổi behavior cho project chưa từng đụng vào CLI)

### REQ-004: Execution Router chọn candidate đầu tiên theo priority còn khả dụng
> ❌ Status: Not Started

**Scenario:**
- WHEN `execution_providers = [{type:cli, ref:claude_code, priority:1, enabled:true}, {type:api, ref:anthropic, priority:2, enabled:true}]`, và credential `cli:claude` đang `status=="active"`
- THEN `ResolveExecutionProvider` trả về candidate CLI (`claude_code`) — không xét tới candidate priority 2

**Scenario: Candidate ưu tiên cao nhất đang rate-limited**
- WHEN candidate CLI priority 1 có credential `status=="rate_limited"` hoặc `CooldownUntil` còn trong tương lai
- THEN Router bỏ qua candidate đó, thử candidate priority 2 (`api/anthropic`) — kiểm tra provider đó còn ≥1 credential khả dụng (không rate-limited toàn bộ)
- AND trả về candidate priority 2 nếu hợp lệ

**Scenario: `enabled:false` bị bỏ qua vô điều kiện**
- WHEN candidate priority 1 có `enabled:false`
- THEN Router bỏ qua candidate đó kể cả khi credential của nó đang khả dụng — không có candidate nào override được `enabled:false`

**Scenario: Không còn candidate nào khả dụng**
- WHEN tất cả candidate `enabled:true` đều có credential rate-limited/cooldown, hoặc `execution_providers` toàn `enabled:false`
- THEN `ResolveExecutionProvider` trả lỗi `"no enabled execution provider is available"` — Task fail với message này, không fallback âm thầm về provider ngẫu nhiên

### REQ-005: Chọn 1 lần lúc Task bắt đầu — không mid-task switch
> ❌ Status: Not Started

**Scenario:**
- WHEN Task đã bắt đầu chạy với candidate CLI `claude_code` đã chọn, và giữa chừng credential đó chuyển sang `rate_limited`
- THEN Task hiện tại **không** tự động chuyển sang candidate khác — Task fail (hoặc theo cơ chế retry step hiện có của `patch_retry_loop`, không đổi)
- AND khi user retry Task (tạo lượt chạy mới), `ResolveExecutionProvider` chạy lại từ đầu và tự nhiên bỏ qua candidate đang cooldown

### REQ-006: CLI credential tham gia cooldown/status check giống API credential
> ❌ Status: Not Started

**Scenario:**
- WHEN credential `provider=="cli:codex"` có `status=="rate_limited"` hoặc `CooldownUntil` còn trong tương lai
- THEN `ResolveExecutionProvider` coi candidate `type:cli, ref:openai_codex` là KHÔNG khả dụng — dùng đúng field `ProviderCredential.Status`/`CooldownUntil` đã tồn tại, không cần bảng cooldown riêng cho CLI

**Scenario: CLI process kết thúc với output báo hết quota — hệ thống tự đánh dấu cooldown (write-side, trước đây chưa tồn tại)**
- WHEN `cliEngine.RunCodeStep` chạy xong (blocking, không streaming — chỉ có captured `stdout`/`stderr` + exit code sau khi process thoát), và output/exit code khớp 1 rule trong `CLIQuotaRules` (bảng pattern cấu hình, xem design.md — ví dụ exit code `429`, hoặc regex case-insensitive trên output như `"rate limit"`, `"usage limit reached"`, `"quota exceeded"`, `"try again later"`, mỗi CLI có bộ pattern riêng vì format lỗi khác nhau giữa Claude Code/Codex/Antigravity)
- THEN `CodeStepResult.QuotaExceeded=true` được set (field mới trên `CodeStepResult`, `server/internal/orchestrator/engine/cli.go`)
- AND `cliEngineRunner.RunLLMStep` (`server/internal/orchestrator/cli_engine_step.go`), sau khi nhận `res.QuotaExceeded==true`, gọi `CooldownSetter.SetCooldown(ctx, credentialID, "", time.Now().Add(cliCooldownDuration))` cho đúng credential đã dùng để chạy step đó (`credentialID` phải được `resolveCLIEngineRunner`/Router truyền xuống `cliEngineRunner`, không suy đoán lại)
- AND step vẫn trả lỗi như bình thường (`res.Success==false` → step fail) — việc set cooldown không thay đổi kết quả step hiện tại, chỉ ảnh hưởng lần `ResolveExecutionProvider` **tiếp theo** (đúng REQ-005: không mid-task switch)

**Scenario: Output không khớp rule nào — không set cooldown nhầm**
- WHEN CLI process fail vì lý do khác (compile error, network lỗi tạm thời không phải quota, bug trong code sinh ra) và output không khớp bất kỳ pattern nào trong `CLIQuotaRules`
- THEN `QuotaExceeded` vẫn là `false`, không gọi `SetCooldown` — tránh false-positive khoá nhầm 1 credential còn tốt

### REQ-007: UI Execution Providers list thay thế form raw
> ❌ Status: Not Started

**Scenario:**
- WHEN user mở Project Settings → Execution Providers, thấy danh sách: Claude API / OpenAI API / Gemini API / Claude Code / OpenAI Codex / Antigravity / Custom CLI
- THEN mỗi dòng có checkbox Enabled + nút ▲▼ đổi Priority — không có field `command`/`args`/`auth_check` hiển thị cho 3 CLI preset (chỉ "Custom CLI" mới mở form raw đầy đủ)
- AND lưu thành `execution_providers` đúng REQ-002

**Scenario: dòng "Custom CLI" phải chọn CLI Authentication Profile (credential) — không có provider cố định để tự map**
- WHEN user chọn "Custom CLI" (bung form raw `command`/`args`/`auth_check`/`env`/`timeout`)
- THEN form hiển thị thêm 1 dropdown "CLI Authentication Profile" liệt kê các credential có `provider` bắt đầu bằng `cli:` (cùng nguồn dữ liệu với dropdown credential đã có trong `cli-engine-config-form.tsx` hôm nay) — bắt buộc chọn 1 (không để trống), lưu vào `ExecutionProviderConfig.CredentialID`
- AND với 3 dòng preset (Claude Code/OpenAI Codex/Antigravity), dropdown credential là **optional** — nếu để trống, Router tự chọn credential `active` đầu tiên có `provider==CLIProfiles[ref].CredentialProvider` (hành vi mặc định hôm nay); nếu chọn (pin), Router chỉ dùng đúng credential đó, không tự chọn công cụ khác dù đang cooldown (fail rõ ràng thay vì âm thầm rơi xuống priority thấp hơn)

### REQ-008: `ModelRoutingRules.tsx` không còn loại trừ `cli:` provider
> ❌ Status: Not Started

**Scenario:**
- WHEN user mở trang Model Routing Rules (AI Providers)
- THEN danh sách provider tab bao gồm cả `cli:claude`/`cli:codex`/`cli:antigravity` bên cạnh `openai`/`anthropic`/`gemini` — filter `!provider.startsWith("cli:")` bị xoá

## Modified Requirements

### REQ-M01: `resolveCLIEngineRunner` gọi qua Execution Router
> ❌ Status: Not Started

**Scenario:**
- WHEN `Orchestrator.resolveCLIEngineRunner(ctx, task)` được gọi và `ResolveExecutionProvider` trả về candidate `type:"api"` (không phải cli)
- THEN hàm trả `nil` (giữ đúng contract cũ: nil nghĩa là "không dùng CLI runner, để `LLMStep` xử lý") — `step_registry.go`/`worker.go` không cần sửa gì thêm ngoài việc gọi hàm này

## Removed Requirements
- Không có requirement nào bị xoá — `ExecutionEngine`/`CLIEngineConfig` (cột cũ) tiếp tục hoạt động làm fallback path (REQ-003), dọn dẹp cột cũ là phase sau, ngoài phạm vi OpenSpec này.
