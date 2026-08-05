# Proposal: CLI Session Continuity (resume after mid-step kill)

> **Follow-up to**: conversation trace in this session — bắt đầu từ câu hỏi "CLI checkpoint có đúng chưa" (đã xác nhận CLI steps chỉ lưu `cli_output`/`cli_prompt` artifact, không có `patch`/`diff` — xem ghi chú riêng, KHÔNG thuộc scope OpenSpec này), rồi rẽ sang "`-c`/`--continue` có dùng để chia nhỏ CLI task theo phase không".

## Why

### CLI hiện tại luôn chạy lại từ đầu, kể cả khi chỉ bị kill giữa chừng vì timeout

Mỗi `RunCodeStep` (`engine/cli.go:255`) là 1 container Docker mới (`docker.go:402`, `docker.go:635`), bị `ContainerRemove(..., Force: true)` ngay sau khi chạy xong (`docker.go:422,660`). `$HOME` bên trong container (`sandbox.SandboxHomeDir = "/home/agent"`, `sandbox.go:27`) **không được bind-mount** — chỉ `/workspace` và (nếu có) logs dir được mount (`docker.go:205-222`). Khi 1 step bị `idleTimeoutHit`/`killed` (`cli.go`'s `CodeStepResult` switch) và `worker.go` retry lại cùng step, container mới hoàn toàn không còn dấu vết gì của lần chạy trước — CLI phải bắt đầu lại từ prompt gốc, tốn lại toàn bộ token/thời gian đã dùng cho phần việc dở dang.

### `-c`/`--continue` của cả 3 CLI đều tồn tại, nhưng bị chặn bởi chính kiến trúc ephemeral-HOME nói trên

Đã xác nhận qua `docs/guides/*-cli-headless.md` (đã cài đặt sẵn, không phải tài liệu suy đoán):
- **claude**: `-c`/`--continue` (tiếp tục gần nhất) VÀ `--resume <session_id>` (resume đúng session — `claude-cli-headless.md:119,282`). `--output-format json` (đã là arg mặc định của `claude_code` profile, `cli_profiles.go`) in kèm `"session_id": "abc123"` trong object JSON cuối cùng (`claude-cli-headless.md:100`).
- **agy** (antigravity): `-c`/`--continue` VÀ `--conversation <id>` (resume đúng ID, `antigravity-cli-headless.md:72-74`).
- **codex**: chỉ có `codex exec resume --last` (`codex-cli-headless.md:68-70`) — **không có** cờ resume theo ID cụ thể trong tài liệu đã xác nhận.

Cả `-c`/`--continue` (mọi CLI) lẫn `codex ... resume --last` đều tra cứu "session gần nhất" dựa trên state lưu dưới `$HOME` (theo cwd, luôn cố định là `/workspace` trong container) — **không phải theo ID**. Vì `$HOME` không tồn tại giữa 2 lần chạy (container bị xoá), 2 cờ này vô nghĩa ở kiến trúc hiện tại: CLI tìm session ở `$HOME` trắng tinh, không thấy gì.

`--resume <session_id>`/`--conversation <id>` (claude, agy) có lợi thế lớn hơn: resume **đúng session theo ID tường minh**, không phụ thuộc "gần nhất trong cwd này" — nghĩa là ngay cả khi nhiều task cùng ghi state vào chung 1 nơi, mình vẫn chọn đúng session cần resume vì có ID cụ thể, không đoán theo cwd. Nhưng vẫn cần state vật lý của session đó (transcript) tồn tại trên đĩa lúc container mới khởi động — tức vẫn cần 1 bind-mount HOME nào đó persistent qua các lần retry.

### Rủi ro đã loại: share session theo project

Trong phần thảo luận trước khi viết OpenSpec này, đã cân nhắc và **loại bỏ** phương án share theo project (mount 1 thư mục session dùng chung cho mọi task cùng project): vì cwd trong container luôn là `/workspace` bất kể task, share ở bất kỳ phạm vi nào rộng hơn "1 task, 1 step-attempt-lineage" đều khiến `-c`/`resume --last` (không có ID) có thể pick nhầm session của 1 task/step khác đã chạy gần đây hơn trên cùng thư mục — lỗi âm thầm (model tiếp tục nhầm việc), không phải lỗi crash rõ ràng.

### Tiền lệ có sẵn trong code: round-trip file theo path cố định (`CredentialFiles`/`UpdatedCredentialFiles`)

`sandbox.CommandRequest.CredentialFiles` (`sandbox.go:42-48`) đã giải quyết một bài toán tương tự ở quy mô nhỏ hơn: vài file credential cố định (vd `.claude.json`) được mount ghi-được vào container, và sau khi chạy xong, `docker.go:502-525` đọc lại nội dung, trả `UpitedCredentialFiles` nếu đổi — dùng để đồng bộ token đã refresh trong lúc CLI chạy trở lại DB. Cơ chế này chứng minh round-trip file theo path cố định đã hoạt động ổn định trong runtime — nhưng nó giả định **path đích cố định biết trước**. Session transcript của claude/agy nằm dưới `~/.claude/projects/<cwd-hash>/<session-id>.jsonl` — path phụ thuộc runtime (hash của cwd, ID sinh ra lúc chạy), không cố định biết trước như `.claude.json` — nên không thể tái dùng y nguyên cơ chế `CredentialFiles`, cần 1 bind-mount thư mục (giống mô hình `LogsHostDir`, `docker.go:213-222`) thay vì 1 file cố định.

## What Changes (đề xuất, cần investigation xác nhận trước khi code — xem `tasks.md` Phase 0)

1. **Phạm vi**: chỉ áp dụng resume khi retry **cùng 1 step, cùng 1 task**, và chỉ khi lần chạy trước bị `idleTimeoutHit`/`killed` (không áp dụng khi chuyển sang step khác, hay khi lỗi là do logic/code sai — resume vào 1 conversation đã sai hướng không giúp ích). Việc thu hẹp phạm vi này matching với câu hỏi đã hỏi trước đó trong hội thoại ("Chỉ khi retry cùng step bị timeout/kill") — chưa có phản hồi phủ định từ người dùng.
2. **Lưu trữ**: bind-mount thư mục `server/.data/workspaces/<task-id>/home/<provider-ref>/` vào `$HOME` container, lồng trong workspace của task để tự động bị dọn bởi cơ chế cleanup-on-completion đã có (commit `3ef54f8`) — không cần thêm logic dọn dẹp riêng. `<provider-ref>` = `CLIEngineConfig.ProfileRef` đã có sẵn (`claude_code`/`openai_codex`/`antigravity`/`custom`).
3. **Chọn cờ resume theo provider** (ưu tiên ID-based khi CLI hỗ trợ, vì an toàn hơn "gần nhất"):
   - `claude_code`: capture `session_id` từ JSON output cuối (mở rộng `parseCLITelemetry`/`cliTelemetry`, `telemetry.go`), lưu vào checkpoint; lần retry cùng step dùng `--resume <session_id>` thay vì chạy lại prompt gốc.
   - `antigravity` (agy): tương tự, dùng `--conversation <id>` nếu agy's `--output-format json` cũng in ID tương đương (**cần investigation, chưa xác nhận field name** — xem Phase 0).
   - `openai_codex`: không có cờ ID-based đã xác nhận — dùng `resume --last`, chấp nhận rủi ro "gần nhất" (đã giảm thiểu vì HOME đã cô lập theo task).
4. **Investigation trước khi code** (Phase 0, `tasks.md`): xác nhận thật sự trên máy có CLI cài sẵn — (a) cấu trúc thư mục thật `~/.claude/projects/...` sau 1 lần chạy `-p --output-format json`, (b) `--resume <session_id>` có hoạt động đúng khi thư mục đó được copy sang 1 container hoàn toàn mới hay không (không đoán — đây là giả định cốt lõi của toàn bộ OpenSpec, phải verify trước khi viết code thật), (c) agy's JSON output có field ID tương đương `session_id` không.

## Explicitly Out of Scope

- **Patch/diff artifact cho CLI steps** (gap về khôi phục workspace khi bị dọn/tái tạo — đã tìm thấy trong hội thoại trước OpenSpec này) — vấn đề khác, giải quyết bằng cơ chế khác (`SaveArtifact(..., "patch", ...)`), không liên quan tới session-continue. Không đưa vào đây để tránh 1 OpenSpec gánh 2 vấn đề không phụ thuộc nhau.
- Share session giữa các task khác nhau (kể cả cùng project) — đã loại ở phần Why.
- Resume xuyên suốt nhiều step khác nhau trong cùng task (vd `cli_analyze` continue sang `cli_implement`) — mỗi step vẫn là 1 conversation độc lập, đúng ngữ nghĩa hiện tại của hệ thống (mỗi step 1 nhiệm vụ khác nhau).

## Capabilities

### New Capabilities
- Session-ID capture cho claude (và agy nếu Phase 0 xác nhận có field tương đương) từ JSON output cuối.
- Bind-mount HOME per-task+provider, cô lập, tự dọn theo workspace lifecycle.
- Resume flag injection khi `worker.go` retry đúng step vừa bị kill/timeout.

### Modified Capabilities
- `engine/telemetry.go` — thêm field session ID vào `cliTelemetry`/`CLITelemetry`.
- `engine/cli.go` — mount HOME, chọn cờ resume theo `ProfileRef` khi là lần retry-sau-kill.
- `sandbox.CommandRequest`/`docker.go` — thêm field mount HOME (mô hình theo `LogsHostDir`).
- `worker.go` — truyền tín hiệu "đây là retry sau kill của step X" xuống `RunCodeStep`.

### Removed Capabilities
- Không có.

## Impact

| Area | Files Affected |
|------|----------------|
| Sandbox | `server/internal/sandbox/sandbox.go`, `server/internal/sandbox/docker.go` |
| CLI engine | `server/internal/orchestrator/engine/cli.go`, `server/internal/orchestrator/engine/telemetry.go` |
| Orchestrator | `server/internal/orchestrator/worker.go`, `server/internal/orchestrator/cli_spec_step.go` |
| Docs | `docs/guides/claude-cli-headless.md`/`antigravity-cli-headless.md` (cập nhật sau investigation nếu phát hiện field/behavior mới) |
| Tests | `server/internal/orchestrator/engine/telemetry_test.go`, `cli_test.go`, `server/internal/sandbox/docker_test.go` |
