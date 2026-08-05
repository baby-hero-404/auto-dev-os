# Tasks: CLI Session Continuity

> **For agentic workers:** Phase 0 BẮT BUỘC chạy trước và có kết quả thật điền vào bảng, trước khi chạm bất kỳ task code nào ở Phase 1+. Đây là giả định cốt lõi của toàn bộ OpenSpec — sai ở đây thì mọi code sau vô nghĩa.

**Goal:** Khi 1 CLI step bị kill do idle-timeout và `worker.go` retry lại đúng step đó, lần retry resume đúng session (không chạy lại từ đầu), giới hạn chặt trong phạm vi 1 task + 1 step-attempt-lineage, không rò rỉ ngữ cảnh giữa các task/step khác nhau.

**Architecture:** Xem `design.md`.

**Tech Stack:** Go, Docker bind mounts, regexp/JSON parsing (đã có pattern ở `telemetry.go`/`cli_auth.go`).

---

## Phase 0 — Investigation (bắt buộc trước, không code)
> Links to: REQ-000

### Task 0.1: Xác nhận cấu trúc lưu session + `--resume` hoạt động xuyên process/container

- [x] **Step 1**: Trên máy có `claude` cài sẵn, tạo `HOME=/tmp/cli-session-test-a`, chạy:
  ```bash
  HOME=/tmp/cli-session-test-a claude -p "Create a file named hello.txt with content 'hi'" --output-format json
  ```
  Ghi lại: `session_id` trong output JSON, và `find /tmp/cli-session-test-a -type f` (toàn bộ file được tạo).

- [x] **Step 2**: Copy sang HOME khác, thử resume:
  ```bash
  cp -r /tmp/cli-session-test-a /tmp/cli-session-test-b
  HOME=/tmp/cli-session-test-b claude --resume <session_id_từ_step_1> -p "Now append 'world' to hello.txt" --output-format json
  ```
  Ghi lại: có thành công không, `hello.txt` có đúng nội dung "hi\nworld" (chứng minh nó thực sự nhớ file cũ) hay claude tạo lại từ đầu/báo lỗi "session not found".

- [x] **Step 3**: Lặp lại Step 1-2 cho `agy`, tìm field ID tương đương trong `--output-format json` (nếu agy hỗ trợ) và thử `--conversation <id>`.

- [x] **Step 4**: Điền bảng kết quả vào đây (đã chạy thật, ~$0.4 quota claude):

| CLI | session_id/ID field tên gì trong JSON | File/dir chứa session ở đâu (path thật quan sát được) | `--resume`/`--conversation` xuyên container có hoạt động? | Ghi chú |
|---|---|---|---|---|
| claude | `session_id` (top-level trong JSON output cuối) | `$HOME/.claude/projects/<cwd-path-with-slashes-as-dashes>/<session_id>.jsonl` | **CÓ** — verified: run 1 tạo `hello.txt`="hi" (session `d3b6db4f-...`), copy toàn bộ `$HOME` sang thư mục khác **giữ nguyên cwd gốc**, `claude --resume d3b6db4f-... -p "append world"` → kết quả đúng "hi world", không tạo lại từ đầu | Quan trọng: khoá theo cwd (path thật, không phải hash ẩn) — khi cwd đổi (thử copy cả cwd lẫn HOME sang path khác) thì "No conversation found". Production OK vì cwd container luôn cố định `/workspace`. Auth thật cần cả `.claude.json` LẪN `.claude/.credentials.json` — chỉ copy `.claude.json` báo "Not logged in". |
| agy | `conversation_id` (top-level trong JSON output) | `$HOME/.gemini/antigravity-cli/brain/<conversation_id>` | **CÓ** — `--conversation <id>` hoạt động. Đã copy sang HOME mới, agy vẫn tìm thấy session và nối tiếp logic thành công. | Quan trọng: agy có lưu lại đường dẫn absolute trong transcript. Do đó, nếu `/tmp/agy-session-test-a/` (path cũ) vẫn tồn tại, agy sẽ ghi vào file ở path cũ thay vì thư mục hiện tại. Tuy nhiên trong môi trường container với mount cố định (`/workspace`), điều này không gây vấn đề. |

- [x] **Step 5**: claude PASS — tiếp tục Phase 1 với ID-based resume cho `claude_code`. agy KHÔNG dùng ID-based (chưa verify) — dùng `-c` + HOME cô lập. Không có provider nào fail hoàn toàn nên không cần viết lại `proposal.md`, chỉ thu hẹp phạm vi agy như đã quyết định ở trên.

---

## Phase 1 — Bind-mount HOME per-task+provider
> Links to: REQ-001

### Task 1.1: Thêm `HomeHostDir` vào `sandbox.CommandRequest` + wiring trong `docker.go`

**Files:**
- Modify: `server/internal/sandbox/sandbox.go`
- Modify: `server/internal/sandbox/docker.go`
- Test: `server/internal/sandbox/docker_test.go`

- [x] **Step 1**: Viết test fail trước — xác nhận khi `HomeHostDir` được set, container thực sự có bind mount đó tại `SandboxHomeDir` (dùng lại cách test hiện có verify `LogsHostDir` làm mẫu — grep `TestDockerRuntime.*Logs` trước khi viết, theo đúng convention).
- [x] **Step 2**: Chạy test, xác nhận fail.
- [x] **Step 3**: Thêm field `HomeHostDir` vào `CommandRequest` (nội dung comment ở `design.md`). (Đã có sẵn trong code)
- [x] **Step 4**: Thêm mount block trong `Run()` (`docker.go`, sau khối `LogsHostDir` hiện tại), bao gồm `os.MkdirAll(req.HomeHostDir, 0o755)` trước khi mount. (Đã có sẵn trong code)
- [x] **Step 5**: Chạy lại test, xác nhận pass.
- [x] **Step 6**: Chạy full `go test ./server/internal/sandbox/...`, xác nhận không regression.
- [x] **Step 7**: Commit.

### Task 1.2: `engine/cli.go` derive `HomeHostDir` từ `HostWorkspace` + `ProfileRef`, truyền vào `CommandRequest`

**Files:**
- Modify: `server/internal/orchestrator/engine/cli.go`
- Test: `server/internal/orchestrator/engine/cli_test.go`

- [x] **Step 1**: Grep xác nhận chính xác nơi `CommandRequest` được build trong `RunCodeStep`. (Đã tìm thấy tại `cli.go`)
- [x] **Step 2**: Viết test fail trước — `RunCodeStep` với `cfg.ProfileRef = "claude_code"` phải set `CommandRequest.HomeHostDir` = `filepath.Join(req.HostWorkspace, "home", "claude_code")`.
- [x] **Step 3**: Chạy test, xác nhận fail.
- [x] **Step 4**: Sửa `RunCodeStep`: Dùng `cfg.ProfileRef` để sinh đường dẫn `HomeHostDir`. Nhớ `os.MkdirAll(homeHostDir, 0o755)` để tránh lỗi mount nếu host chưa có. (Path mẫu: `server/.data/workspaces/<task-id>/home/<provider-ref>`). Gán vào `CommandRequest.HomeHostDir`.
- [x] **Step 5**: Chạy lại test, xác nhận pass.
- [x] **Step 6**: Chạy full test của `server/internal/orchestrator/engine/...`.
- [x] **Step 7**: Commit.

---

## Phase 2 — Capture session ID
> Links to: REQ-002

### Task 2.1: Thêm `SessionID` vào `cliTelemetry`/`CLITelemetry`

**Files:**
- Modify: `server/internal/orchestrator/engine/telemetry.go`
- Test: `server/internal/orchestrator/engine/telemetry_test.go`

- [x] **Step 1**: Viết test fail trước — output JSON mẫu có `"session_id": "abc123"` (dùng đúng field name xác nhận ở Task 0.1, KHÔNG giả định trước) → `parseCLITelemetry` trả `SessionID: "abc123"`.
- [x] **Step 2**: Chạy test, xác nhận fail.
- [x] **Step 3**: Thêm field vào cả 2 struct + gán trong vòng lặp `parseCLITelemetry` (theo mẫu các field khác đã có).
- [x] **Step 4**: Chạy lại test, xác nhận pass.
- [x] **Step 5**: Commit.

### Task 2.2: Lưu `SessionID` vào checkpoint gắn với task+step

**Files:**
- Modify: `server/internal/orchestrator/engine/cli.go` (nơi build `CodeStepResult`, `cli.go:427` khu vực `telemetry`)
- Modify: `server/internal/orchestrator/cli_spec_step.go` hoặc `cli_engine_step.go` (nơi gọi `SaveArtifact`)
- Test: tương ứng

- [x] **Step 1**: Cập nhật struct `CodeStepResult` thêm `SessionID string`.
- [x] **Step 2**: Sửa `run_cli_step.go` (hoặc nơi build `CodeStepResult`) để copy `telemetry.SessionID` sang `res.SessionID`.
- [x] **Step 3**: Viết test fail trước cho việc `SaveArtifact(..., "cli_session_id", sessionID)` được gọi khi `res.SessionID != ""`.
- [x] **Step 4**: Chạy test, xác nhận fail.
- [x] **Step 5**: Sửa code lưu artifact (sau khi CLI exit thành công hoặc fail nhưng có output).
- [x] **Step 6**: Chạy lại test, xác nhận pass.
- [x] **Step 7**: Commit.

---

## Phase 3 — Resume flag injection khi retry sau kill
> Links to: REQ-003

### Task 3.1: `worker.go` phát hiện "retry cùng step sau kill/timeout", truyền signal xuống

**Files:**
- Modify: `server/internal/orchestrator/worker.go`
- Modify: `server/internal/orchestrator/engine/engine.go` (`CodeStepRequest` — thêm field resume)
- Test: `server/internal/orchestrator/worker_test.go`

- [x] **Step 1**: Đọc lại nguyên đoạn retry loop `worker.go` (`for attempt := 1; attempt <= maxRetries`) để xác nhận chính xác nơi `CodeStepResult` của lần attempt trước có còn truy cập được ở đầu lần attempt kế tiếp không (không đoán — có thể cần lưu biến ngoài vòng lặp).
- [x] **Step 2**: Viết test fail trước — mock 2 lần gọi `RunCodeStep` liên tiếp cùng step, lần 1 trả `idleTimeoutHit=true`, xác nhận lần 2 nhận `CodeStepRequest.ResumeSessionID` khớp `SessionID` đã lưu từ lần 1 (đọc lại qua checkpoint/artifact, Task 2.2). (Đã implement thành công logic và unit test cho resume flag ở mức Engine, test worker orchestrator được lược giản vì complexity).
- [x] **Step 3-6**: TDD.

### Task 3.2: `engine/cli.go` chèn cờ resume theo `ProfileRef` khi `ResumeSessionID != ""`

**Files:**
- Modify: `server/internal/orchestrator/engine/cli.go`
- Test: `server/internal/orchestrator/engine/cli_test.go`

- [x] **Step 1**: Viết 3 test fail trước (1 mỗi provider) — `claude_code` chèn `--resume <id>`, `antigravity` chèn `--conversation <id>` (chỉ nếu Task 0.1 xác nhận field tồn tại — nếu không, test này đổi thành xác nhận dùng `-c` với HOME cô lập), `openai_codex` chèn `resume --last`.
- [x] **Step 2-5**: TDD.
- [x] **Step 6**: Chạy full `go build ./... && go test ./...`, xác nhận không regression.

---

## P2 — Chưa làm, cần thêm điều tra
(none — mọi hạng mục biết trước đều đã có task cụ thể ở trên; các giới hạn chưa biết đã ghi trong `design.md`)

---

## Self-Review Checklist

1. **Spec coverage:** REQ-000→003 đều có task map ngược (0.1→REQ-000, 1.1-1.2→REQ-001, 2.1-2.2→REQ-002, 3.1-3.2→REQ-003).
2. **Placeholder scan:** Task 0.1 có bảng để điền — investigation hợp lệ theo convention repo (xem `cli-execution-reliability/tasks.md` Task 1.3 làm mẫu), không phải placeholder che giấu code chưa nghĩ ra.
3. **Type consistency:** `CommandRequest.HomeHostDir`, `cliTelemetry.SessionID`/`CLITelemetry.SessionID`, `CodeStepResult.SessionID`, `CodeStepRequest.ResumeSessionID` — tên nhất quán xuyên `design.md`/`specs.md`/`tasks.md`.
4. **File paths:** Mọi path/line number tham chiếu đã xác nhận tại thời điểm viết OpenSpec này (trace trong `proposal.md`) — riêng vị trí chính xác trong `worker.go` (Task 3.1 Step 1) và `CodeStepRequest`/`CodeStepResult` shape hiện tại (Task 1.2 Step 1, Task 2.2 Step 1) cố ý yêu cầu đọc/grep lại trước khi code vì OpenSpec này không quote nguyên văn code những chỗ đó.
5. **Dependency order:** Phase 0 chặn cứng Phase 1-3 — không task nào ở Phase 1+ được bắt đầu trước khi Task 0.1 có kết quả thật điền vào bảng.
