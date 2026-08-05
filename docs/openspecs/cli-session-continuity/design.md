# Design: CLI Session Continuity

## Kiến trúc tổng quan

```
worker.go (retry loop)
  │  biết: step vừa fail có idleTimeoutHit/killed=true không (đã có trong CodeStepResult)
  │  nếu có + đây là retry cùng step → truyền "resume=true" xuống
  ▼
cli_spec_step.go / cli_engine_step.go (RunCLIStep)
  ▼
engine/cli.go (RunCodeStep)
  │  đọc session ID đã lưu từ checkpoint lần trước (nếu có)
  │  build args: chèn --resume <id> / --conversation <id> / resume --last
  │  build CommandRequest với HomeHostDir = workspace/<task>/home/<provider>/
  ▼
sandbox/docker.go (Run)
  │  bind-mount HomeHostDir -> $HOME container (mới, mô hình theo LogsHostDir)
  │  container chạy, ghi transcript vào $HOME thật (persistent trên host)
  ▼
sau khi chạy xong: parse JSON output cuối → session_id mới (có thể khác id cũ nếu CLI tạo session mới)
  │  lưu lại vào checkpoint cho lần retry kế tiếp (nếu còn cần)
```

## Thư mục lưu trữ

```
server/.data/workspaces/<task-id>/
  ├── logs/                    (đã có)
  ├── home/
  │   └── <provider-ref>/      (mới — mount vào $HOME container)
  └── ... (workspace code — đã có)
```

`<provider-ref>` = `CLIEngineConfig.ProfileRef` (`claude_code`/`openai_codex`/`antigravity`/`custom`). Tách theo provider trong cùng 1 task vì 1 task có thể fallback qua nhiều provider khác nhau giữa các lần retry (`execution_router.go`'s multi-candidate resolution) — không muốn HOME của claude lẫn với HOME của codex nếu task đổi provider giữa chừng.

Lồng trong `workspaces/<task-id>/` (không phải root riêng) để thừa hưởng cleanup-on-completion hiện có — không cần thêm TTL/cron riêng.

## Thay đổi cụ thể theo file

### `server/internal/sandbox/sandbox.go`
Thêm field vào `CommandRequest` (cạnh `LogsHostDir`, comment theo đúng phong cách đã có):
```go
// HomeHostDir, if set, is bind-mounted read-write at the container's HOME
// path (SandboxHomeDir), persisting whatever a CLI writes there (session
// transcripts, local state) across separate Run() calls for the same
// task+provider — unlike the container's own writable layer, which is
// destroyed on ContainerRemove. Runtimes that don't support bind mounts may
// ignore it (session-resume then silently degrades to a fresh session).
HomeHostDir string
```

### `server/internal/sandbox/docker.go`
Thêm 1 mount block ngay sau khối `LogsHostDir` hiện có (`docker.go:213-222`), cùng mô hình `mount.TypeBind`, `os.MkdirAll` trước khi mount.

### `server/internal/orchestrator/engine/telemetry.go`
```go
type cliTelemetry struct {
    ...
    SessionID *string `json:"session_id"`
}

type CLITelemetry struct {
    ...
    SessionID string
}
```
Field optional, permissive — CLI không in field này thì `SessionID` rỗng, không lỗi (đúng tinh thần "permissive superset" đã ghi trong comment file này).

### `server/internal/orchestrator/engine/cli.go`
- `RunCodeStep` nhận biết "đây là resume attempt" qua field mới trên `CodeStepRequest` (vd `ResumeSessionID string`, `IsRetryAfterKill bool` — tên chính xác quyết định lúc code, sau khi xác nhận shape `CodeStepRequest` hiện tại).
- Khi resume: chèn cờ vào `args` TRƯỚC khi apply `{prompt_file}`/`{workdir}` template hiện có, theo bảng cờ mỗi provider (xem `proposal.md` mục 3). Nếu prompt gốc vẫn cần truyền (claude/agy resume vẫn nhận 1 prompt mới cho turn tiếp theo, không phải prompt rỗng) thì instruction nên là 1 câu ngắn kiểu "Continue exactly where you left off and finish the task described in {prompt_file}." — KHÔNG lặp lại nguyên văn prompt gốc (đã nằm trong session).
- Build `CommandRequest.HomeHostDir` từ `req.HostWorkspace`-derived path (tương tự cách `logsHostDir` đang được derive ở `cli.go:368`).

### `server/internal/orchestrator/worker.go`
Retry loop (`for attempt := 1; attempt <= maxRetries; attempt++`) cần biết kết quả `idleTimeoutHit`/`killed` của **lần attempt ngay trước, cùng step** để quyết định có truyền resume signal cho lần kế tiếp không. Cần đọc lại đoạn code quanh chỗ `CodeStepResult` được nhận trong vòng lặp này (chưa đọc chi tiết trong OpenSpec này — task đầu của Phase 1 trong `tasks.md` là grep+đọc để xác nhận vị trí chính xác trước khi sửa, theo đúng convention "không đoán" của repo).

## Giới hạn đã biết, chấp nhận

- **codex không có ID-based resume đã xác nhận** → dùng `resume --last`, đúng nghĩa "gần nhất trong HOME này" — vẫn an toàn vì HOME đã cô lập theo task+provider (REQ-001), chỉ có 1 "gần nhất" khả dĩ trong đó.
- Nếu Phase 0 (REQ-000) phát hiện `--resume`/`--conversation` KHÔNG hoạt động xuyên container như kỳ vọng (vd session transcript tham chiếu thêm state khác ngoài file, hoặc gắn với machine ID), toàn bộ thiết kế cần xét lại — đây là lý do Phase 0 đứng trước mọi task code khác trong `tasks.md`.
- Không resume xuyên step khác nhau, không resume khi lỗi không phải kill/timeout — xem `specs.md` REQ-003 cho lý do.
