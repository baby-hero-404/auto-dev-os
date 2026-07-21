# Specs: Pluggable Execution Engine

## Added Requirements

### REQ-001: Engine interface with two implementations
> ❌ Status: Not Started

**Scenario:**
- WHEN worker nhận job cho task có engine resolve = `api_native`
- THEN toàn bộ pipeline chạy y hệt hành vi hiện tại (zero regression, cùng DAG, cùng steps)
- AND không có code path CLI nào được chạm tới

**Scenario:**
- WHEN task có engine resolve = `cli`
- THEN worker dispatch qua `cliEngine` thay vì llmrunner tool-loop cho các code steps

### REQ-002: Engine resolution (project default + task override)
> ❌ Status: Not Started

**Scenario:**
- WHEN task có `execution_engine = null` và project có `execution_engine = "cli"`
- THEN engine resolve = `cli`
- AND WHEN task có `execution_engine = "api_native"` (override) thì resolve = `api_native` bất kể project setting

### REQ-003: Generic CLI spawn trong sandbox worktree
> ❌ Status: Not Started

**Scenario:**
- WHEN `cliEngine` chạy cho task T với config `{command: "claude", args: ["-p", "{prompt_file}"]}`
- THEN prompt được ghi vào `.autocode/prompt.md` trong worktree của T
- AND command được chạy trong sandbox container với cwd = worktree, network = bridge, timeout = config (default 30m)
- AND placeholder `{prompt_file}`/`{workdir}` trong args được thay bằng đường dẫn container thực
- AND env vars từ `cli_engine_config.env` được inject, không xuất hiện trong bất kỳ log nào

### REQ-004: Preflight check
> ❌ Status: Not Started

**Scenario:**
- WHEN CLI command không tồn tại trong sandbox image
- THEN step fail sớm với message `cli engine preflight failed: command "<cmd>" not found in sandbox image`
- AND không có lần spawn thật nào diễn ra

### REQ-004b: Auth preflight (chống treo vì token hết hạn)
> ❌ Status: Not Started

**Scenario:**
- WHEN `cli_engine_config` có `auth_check_command` (vd `claude auth status`) được khai báo
- THEN preflight chạy nó (timeout 30s) trước spawn thật; exit ≠0 hoặc timeout → step fail `cli auth preflight failed — token may be expired; re-authenticate or provide API key via env`
- AND sandbox spawn thật luôn chạy với env chống interactive login (`CI=1`, stdin đóng) — CLI chờ browser login sẽ chết theo timeout thay vì treo vô hạn

**Scenario:**
- WHEN không có `auth_check_command` và không có env key nào
- THEN preflight log warning khuyến nghị cấu hình một trong hai (không chặn)

### REQ-004c: Loop-kill monitor
> ❌ Status: Not Started

**Scenario:**
- WHEN một dòng error (normalized) xuất hiện ≥10 lần trong 50 dòng error gần nhất (frequency-in-window — bắt được cả stack trace nhiều dòng lặp `A B C D E A B C D E...`, không yêu cầu liên tiếp)
- THEN process bị kill sớm, step fail `cli killed: repeated error loop detected ("<error line>" ×10)` — không đợi hết 30m timeout
- AND dòng không match error pattern (vd progress) không vào cửa sổ đếm, KHÔNG trigger kill
- AND cùng error xuất hiện 9 lần rồi CLI tự thoát loop → không kill (không false positive trên retry hợp lệ)

### REQ-005: Output capture & result semantics
> ❌ Status: Not Started

**Scenario:**
- WHEN CLI process kết thúc exit code 0
- THEN full stdout/stderr được lưu thành step logs
- AND kết quả của step được xác định bằng git diff của worktree (có thay đổi → tiếp tục checkpoint/PR; không thay đổi → step fail "cli produced no changes")

**Scenario:**
- WHEN CLI process exit khác 0 hoặc timeout
- THEN step fail, stderr tail được đưa vào error message, worktree giữ nguyên để retry/debug

### REQ-006: Settings persistence + API
> ❌ Status: Not Started

**Scenario:**
- WHEN PATCH project với `execution_engine: "cli"` và `cli_engine_config`
- THEN giá trị được validate (engine thuộc enum; command không rỗng khi engine=cli) và lưu
- AND GET project trả về config nhưng env var **values** bị mask (`"***"`)

### REQ-007: UI toggle
> ❌ Status: Not Started

**Scenario:**
- WHEN user mở project settings
- THEN thấy section Execution Engine với radio + form CLI config (chỉ hiện khi chọn CLI)
- AND WHEN tạo task mới thì có dropdown Engine (Inherit/API-native/CLI) default Inherit
- AND task detail hiển thị badge engine đã dùng

## Modified Requirements

### REQ-M01: Worker dispatch
> ❌ Status: Not Started

**Scenario:**
- WHEN job được claim trong `orchestrator/worker.go`
- THEN engine được resolve một lần và gắn vào job context; mọi code step của job dùng cùng engine (không đổi giữa chừng)

## Removed Requirements
- Không có.
