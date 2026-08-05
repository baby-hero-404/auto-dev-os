# Specs: CLI Session Continuity

## Added Requirements

### REQ-000: Investigation xác nhận giả định cốt lõi (làm trước, không đoán)
> ❌ Status: Not Started

**Scenario: Xác nhận cấu trúc lưu session thật của claude**
- WHEN chạy `claude -p "..." --output-format json` với `HOME` trỏ vào 1 thư mục test cố định
- THEN ghi lại: session transcript nằm ở path nào, tên file có đoán trước được không, `session_id` trong JSON output có khớp với tên file/thư mục sinh ra không

**Scenario: Xác nhận `--resume <session_id>` hoạt động xuyên container**
- WHEN copy nguyên thư mục `HOME` từ Scenario 1 sang 1 container/process mới (không dùng lại process cũ), rồi chạy `claude --resume <session_id> -p "tiếp tục" --output-format json` với `HOME` trỏ vào bản copy đó
- THEN xác nhận claude thực sự nối tiếp đúng ngữ cảnh (không tạo session mới, không lỗi "session not found")

**Scenario: Xác nhận agy có cơ chế tương đương**
- WHEN chạy `agy -p "..." --output-format json`
- THEN ghi lại: output JSON có field ID session tương đương `session_id` của claude không; nếu không, `antigravity` dùng chiến lược "cô lập HOME theo task + `-c`" giống `openai_codex` thay vì ID-based

### REQ-001: HOME của container được cô lập theo task + provider, persistent qua các lần retry
> ❌ Status: Not Started

**Scenario: Bind-mount HOME riêng cho từng task**
- WHEN `RunCodeStep` chạy cho task X với provider `claude_code`
- THEN `$HOME` container được bind-mount từ `server/.data/workspaces/<task-X-id>/home/claude_code/` trên host — không phải thư mục ephemeral trong container

**Scenario: 2 task khác nhau không chia sẻ HOME**
- WHEN task A và task B (khác ID, có thể cùng project, cùng provider) đều chạy
- THEN mỗi task ghi vào thư mục HOME riêng của mình, không đọc/ghi chéo

**Scenario: HOME bị dọn cùng lúc với workspace khi task hoàn tất**
- WHEN task hoàn tất và cơ chế cleanup-on-completion hiện có chạy
- THEN thư mục `home/<provider>/` (nằm trong workspace của task) bị dọn theo, không cần thêm logic cleanup riêng

### REQ-002: Capture session ID từ output CLI hỗ trợ ID-based resume
> ❌ Status: Not Started

**Scenario: claude trả JSON có `session_id`**
- WHEN 1 lần chạy `claude_code` thành công hoặc bị kill nhưng có in JSON output trước đó
- THEN `session_id` được parse và lưu vào checkpoint (`SaveArtifact` hoặc field mới trên `CodeStepResult`, gắn với `task_id + step_id`)

**Scenario: Không tìm thấy session_id — không lỗi, chỉ không resume được**
- WHEN output không chứa JSON hợp lệ hoặc thiếu field `session_id`
- THEN hệ thống fallback về hành vi hiện tại (chạy lại từ đầu prompt gốc), không crash

### REQ-003: Retry cùng step sau kill/timeout dùng cờ resume thay vì chạy lại từ đầu
> ❌ Status: Not Started

**Scenario: Step bị `idleTimeoutHit`/`killed`, retry lần kế tiếp cùng step**
- WHEN `worker.go` phát hiện lần chạy trước của **cùng step, cùng task** kết thúc với `idleTimeoutHit=true` hoặc `killed=true`, và có session ID/HOME đã lưu từ lần đó
- THEN lần retry gọi CLI với cờ resume phù hợp provider (`--resume <id>` cho claude, `--conversation <id>` cho agy nếu REQ-000 xác nhận có, `resume --last` cho codex) thay vì prompt gốc từ đầu

**Scenario: Retry do lỗi khác (không phải kill/timeout) — không resume**
- WHEN lần chạy trước fail vì lý do khác (auth invalid, exit code lỗi logic, quota) — không phải `idleTimeoutHit`/`killed`
- THEN retry chạy lại bình thường từ prompt gốc, không dùng cờ resume (resume vào 1 conversation đã đi sai hướng không có ích, có thể khiến model lặp lại sai lầm)

**Scenario: Chuyển sang step khác — không resume**
- WHEN task chuyển từ `cli_analyze` sang `cli_implement` (step khác, không phải retry cùng step)
- THEN `cli_implement` luôn chạy conversation mới, không mang theo session của `cli_analyze`

**Scenario: Không có session ID/HOME khả dụng (vd sau restart server, hoặc REQ-002 không tìm thấy ID)**
- WHEN không có state nào để resume
- THEN retry rơi về hành vi hiện tại (chạy lại từ đầu), ghi log rõ lý do (không silently fail)

## Removed Requirements
- Không có.
