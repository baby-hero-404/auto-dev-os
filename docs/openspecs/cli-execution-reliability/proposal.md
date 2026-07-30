# Proposal: CLI Execution Reliability & Tracing

> **Evidence Source**: Task thực tế `cfeacf66-c111-47d9-a0bc-7b7cee85dda4` (project "finance management") — fail ở step `cli_analyze` sau 3 lần retry.
> **Follow-up to**: [`definition-of-ready-gate/`](../definition-of-ready-gate/) — task 1.5b của OpenSpec đó bị đánh dấu "skipped" với ghi chú *"CLI-mode DI: fallback khi LLMClient unavailable... agent CLI là black-box, không có cơ chế clarification nào tồn tại trong CLI flow hiện tại... để lại cho khi CLI-mode clarification thực sự được yêu cầu."* OpenSpec này hiện thực hoá đúng phần bị hoãn đó.

## Why

### Phần 1 — Bug đã trace được từ task thật

Query trực tiếp `workflow_artifacts` (artifact `cli_output`, lưu qua `checkpoint.SaveArtifact` ở `cli_spec_step.go:143`) cho task `cfeacf66-...` cho ra output thật của cả 3 lần retry:

```
Not logged in · Please run /login
```

Root cause: project rơi vào org default execution provider (`organizations.default_execution_providers`), candidate `claude_code` (`server/pkg/models/cli_profiles.go:19-25`) được chọn, nhưng credential `cli:claude` đã mất hiệu lực đăng nhập. `Preflight` (`engine/cli.go:138-193`) vẫn PASS vì `auth_check_command: "claude --version"` — lệnh này chỉ in version, exit code luôn `0` bất kể trạng thái login, không thực sự kiểm tra auth.

Hệ quả kép:
1. **Tốn 3 lần retry vô ích** (~14s, nhưng về nguyên tắc là tiền lệ xấu — cùng pattern sẽ tốn tới 90 phút nếu timeout dài hơn) cho một lỗi permanent, không bao giờ tự khỏi bằng cách retry.
2. **Lý do thật không thấy được ở nơi đầu tiên người vận hành sẽ tra**: `server/.data/logs/<task_id>.jsonl` chỉ có message chung `"cli exited with status 1"` (`engine/cli.go:333`), vì `cli_spec_step.go:140` luôn log level `"info"` với message `"%s: cli engine finished (success=%v)"` bất kể `success=false` — output thật chỉ nằm trong DB artifact, phải query trực tiếp `workflow_artifacts` mới thấy.

### Phần 2 — Gap chưa được xử lý: CLI hỏi lại giữa chừng (clarifying question)

CLI (`claude`/`codex`/`antigravity`) chạy hoàn toàn non-interactive trong sandbox: `CI=1` được set (`engine/cli.go:290`), không có stdin/tty đính kèm (comment tại `engine/cli.go:135-137`: *"the sandbox runtime never opens stdin/tty for spawned commands"*). Hệ thống hiện tại **không có cơ chế nào phân biệt** 3 tình huống sau khi nhìn vào output:
- CLI đang chạy việc hợp lệ, chỉ là lâu.
- CLI thật sự lỗi (crash, exception).
- CLI đang **hỏi lại** người dùng một câu hỏi làm rõ (clarifying question) hoặc chờ xác nhận, và vì không có stdin nên nó chỉ... đứng im hoặc thoát non-zero.

`loop_detector.go:16-23` chỉ bắt dòng lặp ≥10 lần trong cửa sổ 50 dòng chứa `error/failed/exception/traceback/panic/retry` — một câu hỏi in ra 1 lần duy nhất không match pattern nào cả. Kết quả: task chạy tới hết `defaultCLITimeout` (30 phút, `engine/cli.go:28`) × 3 lần retry trước khi fail, không có thông tin gì hữu ích ngoài "exited with status N" hoặc timeout.

Với flow API-native (LLM-based `analyze` step), vấn đề này **đã được giải quyết**: `analyze.go:684-751` phát hiện `clarification_questions` (structured output từ LLM), lưu vào `task.Clarifications` (`models.ClarificationRound`), set `SpecStatus = TaskSpecStatusClarificationRequired`, và pause qua `workflow.PauseError`. `TaskService.Clarify` (`service/task.go:215-250`) nhận câu trả lời, ghi thêm 1 round, resume task. Nhưng cơ chế này phụ thuộc vào LLM trả JSON có cấu trúc — **CLI agent là black-box, không có structured output tương đương**, nên toàn bộ cơ chế này chưa từng áp dụng được cho `cli_analyze`/`cli_spec`/`cli_implement`.

Tham khảo cách các reference project xử lý vấn đề tương tự (`docs/references/*/DISCOVERY-*.md`):
- **ai-sdlc**: chặn từ gốc bằng Definition-of-Ready gate trước khi dispatch — chính là spec đã có (`definition-of-ready-gate/`), nhưng chỉ áp dụng cho flow API-native.
- **multica**: có state rõ ràng (`queued`/`deferred`/`blocked`) + pause/resume qua "blocker" comment.
- **free-claude-code**: có `FailureKind` enum phân loại lý do dừng thay vì string-match rời rạc.
- **aider**: ngay cả dự án trưởng thành này cũng chỉ tự-quyết cho lỗi format/parse (cap 3 lần), mọi thứ khác đều `confirm_ask()` — xác nhận đây là bài toán khó, chưa có giải pháp hoàn chỉnh ở bất kỳ đâu, chỉ có cách giảm thiểu.

## What Changes

### Issue 1: Log lỗi CLI step ra jsonl log kèm lý do thật (P0)
- `cli_spec_step.go:140`: khi `!res.Success`, log level `"error"` với message kèm `res.Output`/`res.Error` (đã redact qua `redactSecrets`, có sẵn) — không chỉ log "info" chung chung.

### Issue 2: Phân loại lỗi auth-invalid là permanent, không đốt retry (P0)
- File mới `engine/cli_auth.go`, mirror đúng cấu trúc `CLIQuotaRules`/`detectQuotaExceeded` (`cli_quota.go`) nhưng cho pattern auth-invalid ("Not logged in", "Please run /login", "please authenticate", ...), keyed theo profile ref.
- `RunCodeStep` (`engine/cli.go`): khi `detectAuthInvalid(...)` match, đánh dấu lỗi permanent (giống tinh thần `ErrConfigInvalid`, `cli.go:23`) để caller (`RunCLIStep`, `cli_spec_step.go`) không burn hết 3 lần retry cho lỗi sẽ lặp lại y hệt mỗi lần.

### Issue 3: `auth_check_command` thực sự kiểm tra login state (P0, best-effort)
- Rà lại `auth_check_command` của từng profile trong `cli_profiles.go` — hiện tại `claude --version`/`codex --version`/`antigravity --version` đều chỉ check binary chạy được, không check auth. Cần xác nhận lệnh thật (có thể không tồn tại 1 lệnh "check-only" cho mọi CLI) — xem `design.md` cho hướng tiếp cận cụ thể và giới hạn đã biết.

### Issue 4: Cấm CLI agent hỏi lại giữa chừng qua system prompt (P1)
- `server/internal/prompts/steps/cli_analyze.md`, `cli_spec.md`, `cli_implement.md`: thêm instruction rõ ràng — không được dừng lại hỏi người dùng; nếu gặp điều mơ hồ, tự đưa ra giả định hợp lý nhất, tiếp tục, và ghi chú giả định vào section Risks (đã có)/Implementation Notes (đã có ở `cli_implement.md`).

### Issue 5: Detector phân loại "đang hỏi lại" khác với "đang lỗi" (P1)
- Mở rộng theo mô hình `loop_detector.go`/`cli_quota.go`: thêm rule-set pattern nhận diện câu hỏi/xác nhận đặc trưng (`(y/n)`, `Do you want to`, `Please confirm`, `?` ở cuối dòng cuối cùng của output khi process đã kết thúc, ...).

### Issue 6: Pause/resume khi phát hiện "đang hỏi lại", tái dùng hạ tầng clarification đã có (P1)
- Tái dùng `models.TaskSpecStatusClarificationRequired`, `models.ClarificationRound`, `task.Clarifications`, `workflow.PauseError` — **không** tạo task status mới. `cli_analyze.go`/`cli_spec.go`/`cli_implement.go`: khi detector ở Issue 5 match, trả `workflow.PauseError` với reason tương tự `"workflow paused for human task clarification"` thay vì để step fail.
- `TaskService.Clarify` (`service/task.go:215-250`) hiện hard-code resume về `TaskStatusAnalyzing` — cần tổng quát hoá để resume đúng bước đã pause (CLI có thể pause ở `cli_implement`, không phải luôn `cli_analyze`) thay vì luôn nhảy về đầu flow.

### Issue 7: Xác nhận flag non-interactive thật (P1, verification task — không phải code)
- Ghi vào `design.md` như một hạng mục cần verify thủ công: `CI=1` có thực sự đủ để mọi CLI (`claude`, `codex`, `antigravity`) không bao giờ block chờ tty không, hay cần thêm flag riêng.

### Issue 8: Worktree reset giữa các lần retry (P2 — cần điều tra thêm trước khi viết task cụ thể)
- Chưa xác nhận được liệu git worktree có bị "bẩn" giữa các lần retry tự động trong cùng 1 task khi 1 lần treo dở dang bị kill. Ghi nhận là rủi ro cần điều tra riêng, **không** đưa vào `tasks.md` của OpenSpec này cho tới khi có bằng chứng cụ thể (tránh viết task suy đoán).

## Capabilities

### New Capabilities
- `engine/cli_auth.go`: detector lỗi auth-invalid (permanent), theo mô hình `cli_quota.go`.
- Detector "stuck-awaiting-input" cho CLI output (Issue 5).
- Pause/resume cho CLI mode dựa trên clarification, tái dùng field/status đã có (Issue 6).

### Modified Capabilities
- `cli_spec_step.go` — logging khi step fail (Issue 1), pause khi cần clarification (Issue 6).
- `engine/cli.go` — phân loại lỗi permanent vs retriable (Issue 2).
- `cli_profiles.go` — auth_check_command (Issue 3, best-effort).
- Prompt templates 3 CLI step (Issue 4).
- `TaskService.Clarify` — tổng quát hoá resume target (Issue 6).

### Removed Capabilities
- Không có.

## Impact

| Area | Files Affected |
|------|----------------|
| CLI engine | `server/internal/orchestrator/engine/cli.go`, `server/internal/orchestrator/engine/cli_auth.go` (mới), `server/internal/orchestrator/engine/loop_detector.go` |
| CLI profiles | `server/pkg/models/cli_profiles.go` |
| Sandbox image | `docker/Dockerfile.sandbox` (REQ-008: `xvfb`/`xauth` for antigravity) |
| Orchestrator steps | `server/internal/orchestrator/cli_spec_step.go`, `server/internal/orchestrator/steps/cli_analyze.go`, `server/internal/orchestrator/steps/cli_spec.go`, `server/internal/orchestrator/steps/cli_implement.go` |
| Prompts | `server/internal/prompts/steps/cli_analyze.md`, `server/internal/prompts/steps/cli_spec.md`, `server/internal/prompts/steps/cli_implement.md` |
| Service | `server/internal/service/task.go` (`Clarify`) |
| Tests | `server/internal/orchestrator/engine/cli_test.go`, `cli_auth_test.go` (mới), `server/internal/orchestrator/cli_spec_step_test.go`, `server/internal/service/task_test.go` |
