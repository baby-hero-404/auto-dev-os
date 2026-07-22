# Specs: Pluggable Execution Engine

## Added Requirements

### REQ-001: Engine interface with two implementations
> ✅ Status: Implemented — `engine.ExecutionEngine` (api_native passthrough + cli), dispatched by swapping the `steps.LLMRunner` given to code_backend/code_frontend/fix; api_native path is byte-for-byte the pre-existing code path (zero code touched in `runPatchRetryLoop`/`llmrunner`)

**Scenario:**
- WHEN worker nhận job cho task có engine resolve = `api_native`
- THEN toàn bộ pipeline chạy y hệt hành vi hiện tại (zero regression, cùng DAG, cùng steps)
- AND không có code path CLI nào được chạm tới

**Scenario:**
- WHEN task có engine resolve = `cli`
- THEN worker dispatch qua `cliEngine` thay vì llmrunner tool-loop cho các code steps

### REQ-002: Engine resolution (project default + task override)
> ✅ Status: Implemented — `engine.ResolveEngine` + `Orchestrator.resolveCLIEngineRunner`, unit + orchestrator-level tests

**Scenario:**
- WHEN task có `execution_engine = null` và project có `execution_engine = "cli"`
- THEN engine resolve = `cli`
- AND WHEN task có `execution_engine = "api_native"` (override) thì resolve = `api_native` bất kể project setting

### REQ-003: Generic CLI spawn trong sandbox worktree
> ✅ Status: Implemented — prompt written to `.autocode/prompt.md` (base64-encoded through a single `bash -lc` script, avoiding argv escaping limits), cwd = resolved worktree container path, network = bridge/none mirroring the project's networking policy (same as other sandbox steps), timeout from `cli_engine_config.timeout_minutes` (default 30m), `{prompt_file}`/`{workdir}` substituted per-arg, env values only ever appear inside the sandbox command (never logged — `o.log`/artifacts only see redacted stdout/stderr)

**Scenario:**
- WHEN `cliEngine` chạy cho task T với config `{command: "claude", args: ["-p", "{prompt_file}"]}`
- THEN prompt được ghi vào `.autocode/prompt.md` trong worktree của T
- AND command được chạy trong sandbox container với cwd = worktree, network = bridge, timeout = config (default 30m)
- AND placeholder `{prompt_file}`/`{workdir}` trong args được thay bằng đường dẫn container thực
- AND env vars từ `cli_engine_config.env` được inject, không xuất hiện trong bất kỳ log nào

### REQ-004: Preflight check
> ✅ Status: Implemented — `cliEngine.Preflight` runs `command -v <cmd>` in the sandbox before any real spawn; nonzero exit → `cli engine: command "<cmd>" not found in sandbox`, no spawn attempted

**Scenario:**
- WHEN CLI command không tồn tại trong sandbox image
- THEN step fail sớm với message `cli engine preflight failed: command "<cmd>" not found in sandbox image`
- AND không có lần spawn thật nào diễn ra

### REQ-004b: Auth preflight (chống treo vì token hết hạn)
> ✅ Status: Implemented — `auth_check_command` runs with 30s timeout + `CI=1`/no-stdin before the real spawn (nonzero/timeout → fail with re-auth message), and the real spawn also always sets `CI=1`. When both `auth_check_command` and `env` are empty, `Preflight` now returns a warning string (surfaced via `cliEngineRunner` as a `warn`-level job log) instead of silently skipping the check; execution still proceeds.

**Scenario:**
- WHEN `cli_engine_config` có `auth_check_command` (vd `claude auth status`) được khai báo
- THEN preflight chạy nó (timeout 30s) trước spawn thật; exit ≠0 hoặc timeout → step fail `cli auth preflight failed — token may be expired; re-authenticate or provide API key via env`
- AND sandbox spawn thật luôn chạy với env chống interactive login (`CI=1`, stdin đóng) — CLI chờ browser login sẽ chết theo timeout thay vì treo vô hạn

**Scenario:**
- WHEN không có `auth_check_command` và không có env key nào
- THEN preflight log warning khuyến nghị cấu hình một trong hai (không chặn)

### REQ-004c: Loop-kill monitor
> ✅ Status: Implemented — frequency-in-window ring buffer (50-line window, ≥10 repeats of a normalized error line triggers kill), only lines matching `errorLinePatterns` count toward the window. **Trade-off**: `sandbox.Runtime.Run` is a single blocking call with no line-by-line callback, so this runs as a post-hoc pass over the fully-captured output rather than a true live-stream kill (the process itself always runs to completion or timeout; only the reported step result is short-circuited to "killed"). Extending `sandbox.Runtime` for real streaming was out of scope for this slice.

**Scenario:**
- WHEN một dòng error (normalized) xuất hiện ≥10 lần trong 50 dòng error gần nhất (frequency-in-window — bắt được cả stack trace nhiều dòng lặp `A B C D E A B C D E...`, không yêu cầu liên tiếp)
- THEN process bị kill sớm, step fail `cli killed: repeated error loop detected ("<error line>" ×10)` — không đợi hết 30m timeout
- AND dòng không match error pattern (vd progress) không vào cửa sổ đếm, KHÔNG trigger kill
- AND cùng error xuất hiện 9 lần rồi CLI tự thoát loop → không kill (không false positive trên retry hợp lệ)

### REQ-005: Output capture & result semantics
> ✅ Status: Implemented — full stdout/stderr captured, redacted, and saved as a step artifact; exit code + loop-kill decide `CodeStepResult.Success`; nonzero exit surfaces the redacted failure in the error message and leaves the worktree untouched for retry/debug. On a successful exit, `cliEngineRunner.RunLLMStep` now explicitly checks `git status --porcelain` (via `repoutil.GetChangedFiles`) before reporting success back to `runPatchRetryLoop`: zero changed files fails the step (`"cli engine: run completed but produced no file changes"`) unless `cli_engine_config.allow_noop` is set, for genuinely read-only/inspection CLI configs.

**Scenario:**
- WHEN CLI process kết thúc exit code 0
- THEN full stdout/stderr được lưu thành step logs
- AND kết quả của step được xác định bằng git diff của worktree (có thay đổi → tiếp tục checkpoint/PR; không thay đổi → step fail "cli produced no changes")

**Scenario:**
- WHEN CLI process exit khác 0 hoặc timeout
- THEN step fail, stderr tail được đưa vào error message, worktree giữ nguyên để retry/debug

### REQ-006: Settings persistence + API
> ✅ Status: Implemented — migration + model fields (Group 1), PATCH validation (engine enum, command required when cli) and env-value masking on GET, round-trip tests

**Scenario:**
- WHEN PATCH project với `execution_engine: "cli"` và `cli_engine_config`
- THEN giá trị được validate (engine thuộc enum; command không rỗng khi engine=cli) và lưu
- AND GET project trả về config nhưng env var **values** bị mask (`"***"`)

### REQ-007: UI toggle
> ✅ Status: Implemented — Project settings UI section with Execution Engine select & CLI config form (env key-value masking), Task create dialog with engine selector (Inherit/API-native/CLI), and Task detail hero title block with engine badge.

**Scenario:**
- WHEN user mở project settings
- THEN thấy section Execution Engine với radio + form CLI config (chỉ hiện khi chọn CLI)
- AND WHEN tạo task mới thì có dropdown Engine (Inherit/API-native/CLI) default Inherit
- AND task detail hiển thị badge engine đã dùng

## Modified Requirements

### REQ-M01: Worker dispatch
> ✅ Status: Implemented — `stepRunners()` resolves the engine once per job build via `resolveCLIEngineRunner`, and swaps `codeStepLLM` for `code_backend`/`code_frontend`/`fix` only; all other steps unaffected, engine never changes mid-job

**Scenario:**
- WHEN job được claim trong `orchestrator/worker.go`
- THEN engine được resolve một lần và gắn vào job context; mọi code step của job dùng cùng engine (không đổi giữa chừng)

## Removed Requirements
- Không có.
