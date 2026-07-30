# Specs: CLI Execution Reliability & Tracing

## Added Requirements

### REQ-001: Log lỗi CLI step ra jsonl kèm lý do thật
> ✅ Status: Implemented

**Scenario: CLI step fail — log phải chứa lý do, không chỉ "cli exited with status N"**
- WHEN một CLI step (`cli_analyze`/`cli_spec`/`cli_implement`) chạy xong với `res.Success=false`
- THEN entry ghi vào `server/.data/logs/<task_id>.jsonl` có `level="error"` và `message` chứa `res.Error` cùng tối đa 2000 ký tự cuối của `res.Output` (đã redact secret)
- AND không cần query DB (`workflow_artifacts`) mới biết được lý do fail

**Scenario: CLI step thành công — hành vi log không đổi**
- WHEN `res.Success=true`
- THEN log vẫn ở level "info" với message "cli engine finished (success=true)" — không đổi hành vi hiện tại cho case thành công

### REQ-002: Lỗi auth-invalid được phân loại permanent, không đốt hết retry
> ✅ Status: Implemented

**Scenario: Credential không hợp lệ (auth) — fail nhanh, không retry 3 lần**
- WHEN output của CLI khớp `CLIAuthInvalidRules[ref]` (vd chứa "Not logged in", "Please run /login")
- THEN step fail ngay với lỗi đánh dấu permanent (wrap `engine.ErrConfigInvalid` hoặc tương đương) — state machine không lặp lại 3 lần retry cho cùng 1 credential chắc chắn sẽ fail y hệt
- AND log (REQ-001) hiển thị rõ đây là lỗi auth, không phải lỗi code/logic chung chung

**Scenario: Lỗi khác (không phải auth-invalid) — hành vi retry không đổi**
- WHEN output không khớp bất kỳ pattern nào trong `CLIAuthInvalidRules`
- THEN hành vi retry 3 lần hiện tại giữ nguyên, không bị ảnh hưởng

**Scenario: Quota-exceeded vẫn được ưu tiên đúng thứ tự nếu cả 2 cùng match**
- WHEN output vừa khớp `detectQuotaExceeded` vừa khớp `detectAuthInvalid` (hiếm, nhưng cần thứ tự rõ ràng)
- THEN auth-invalid được ưu tiên báo cáo (vì quota là transient/cooldown còn auth-invalid là permanent — permanent phải thắng để không cooldown nhầm rồi vẫn retry vô ích)

### REQ-003: `auth_check_command` — investigation trước, code sau
> ✅ Status: Implemented (claude_code, openai_codex) — antigravity giữ nguyên, không tìm được lệnh phù hợp

**Scenario: Xác nhận lệnh check-login thật của từng CLI**
- WHEN thực hiện investigation task cho `claude`/`codex`/`antigravity`
- THEN ghi kết quả (lệnh tồn tại hay không, có side-effect gì) vào `tasks.md`/`design.md` trước khi sửa `cli_profiles.go`
- AND nếu không tìm được lệnh phù hợp cho 1 CLI cụ thể, giữ nguyên `auth_check_command` hiện tại cho CLI đó và ghi rõ lý do (không đoán mò)

### REQ-004: Prompt CLI step cấm hỏi lại giữa chừng
> ✅ Status: Implemented

**Scenario: CLI agent gặp yêu cầu mơ hồ**
- WHEN `cli_analyze`/`cli_spec`/`cli_implement` chạy với instruction đã có thêm đoạn "Do not ask clarifying questions"
- THEN agent được hướng dẫn tự đưa ra giả định hợp lý nhất và ghi chú vào Risks (cli_analyze) hoặc Implementation Notes (cli_spec/cli_implement) thay vì dừng lại chờ trả lời

### REQ-005: Detector nhận diện CLI đang "chờ trả lời" (awaiting-input)
> ✅ Status: Implemented

**Scenario: Output kết thúc bằng pattern hỏi/xác nhận**
- WHEN dòng cuối cùng (non-empty) của combined output khớp 1 trong `awaitingInputPatterns` (vd kết thúc bằng "(y/n)", "Do you want to...?")
- THEN `detectAwaitingInput` trả `true`, `CodeStepResult.AwaitingInput = true`

**Scenario: Output không kết thúc bằng pattern hỏi — không có false positive giữa chừng**
- WHEN 1 dòng giữa transcript (không phải dòng cuối) chứa "?" hoặc từ khoá tương tự nhưng đó chỉ là log bình thường
- THEN `detectAwaitingInput` không bị trigger (chỉ check dòng cuối cùng sau khi process đã kết thúc)

### REQ-006: Pause/resume CLI step khi cần clarification, tái dùng infra đã có
> ✅ Status: Implemented

**Scenario: CLI step phát hiện awaiting-input**
- WHEN `out.AwaitingInput = true` được trả về từ `RunCLIStep`
- THEN step tương ứng (`cli_analyze.go`/`cli_spec.go`/`cli_implement.go`) append 1 `models.ClarificationRound` vào `task.Clarifications`, set `SpecStatus = TaskSpecStatusClarificationRequired`, và trả `workflow.PauseError{Step: <step hiện tại>, Reason: "workflow paused for human task clarification (cli)"}`
- AND task hiển thị lên UI giống hệt cách clarification của flow API-native đã hiển thị (tái dùng UI, không xây mới)

**Scenario: Người dùng trả lời, task resume đúng step đã pause (không phải luôn về cli_analyze)**
- WHEN `TaskService.Clarify` được gọi cho 1 task đang `clarification_required` mà bị pause ở `cli_implement`
- THEN task resume và chạy lại `cli_implement` (không nhảy về `cli_analyze`), instruction của lần chạy lại có kèm câu trả lời

**Scenario: Flow API-native (analyze.go) không bị ảnh hưởng**
- WHEN `TaskService.Clarify` được gọi cho 1 task bị pause ở step `analyze` (flow cũ, không đổi)
- THEN hành vi y hệt hôm nay — resume về `TaskStatusAnalyzing` như hiện tại, mọi test case cũ của `Clarify`/`analyze.go` vẫn pass

### REQ-007: Xác nhận flag non-interactive thật của từng CLI (manual verification)
> ❌ Status: Not Started (Investigation)

**Scenario: Chạy thử từng CLI trong sandbox không có tty**
- WHEN chạy `claude`/`codex`/`antigravity` với `CI=1`, không stdin, trong 1 tình huống cố ý mơ hồ
- THEN quan sát và ghi lại: CLI có tự treo chờ tty, hay tự thoát/tự quyết — kết quả ghi vào `tasks.md`, dùng để tinh chỉnh `awaitingInputPatterns` (REQ-005) nếu cần

### REQ-008: `antigravity` profile chạy được trong sandbox headless
> ⚠️ Status: Superseded — xem ghi chú bên dưới

**[SUPERSEDED]** Investigation gốc (dưới đây, giữ lại cho lịch sử) kết luận `antigravity` là GUI binary cần `xvfb-run`. Một nhánh làm việc khác song song (đã merge vào `master`, xem `docker/Dockerfile.sandbox` và `server/pkg/models/cli_profiles.go` hiện tại) phát hiện ra bằng chứng mạnh hơn: binary headless thật tên là **`agy`** (không phải `antigravity`), có tài liệu riêng tại `docs/guides/antigravity-cli-headless.md` với các flag đã xác nhận trực tiếp từ `agy --help`. `agy` không cần display — không cần `xvfb-run`. Profile hiện tại dùng `Command: "agy"`, `Args: ["--dangerously-skip-permissions", "-p", promptFileInstruction]` (thứ tự flag quan trọng: `-p` phải ngay trước prompt vì `agy` dùng Go stdlib `flag`, xác nhận từ output thật của 1 lần chạy bị lỗi). `docker/Dockerfile.sandbox` không còn cài `xvfb`/`xauth` (đã gỡ, không cần nữa).

<details>
<summary>Investigation gốc (superseded, giữ để tham khảo)</summary>

**Scenario: antigravity là GUI binary (VSCode fork), cần display để khởi động**
- WHEN `resolveCLICandidate` build config cho candidate `ref: "antigravity"`
- THEN `Command = "xvfb-run"`, `Args` chứa subcommand thật `chat --mode agent` (không phải `run --yes` — subcommand không tồn tại, verified qua `antigravity --help`/`antigravity run --help` chạy thật)
- AND `docker/Dockerfile.sandbox` cài `xvfb`+`xauth` để `xvfb-run` khả dụng trong container

**Scenario: AuthCheckCommand không đổi, không cần display**
- WHEN Preflight chạy `auth_check_command`
- THEN vẫn dùng `antigravity --version` trực tiếp (không qua `xvfb-run`) vì lệnh này không cần mở cửa sổ

</details>

## Removed Requirements
- Không có.
