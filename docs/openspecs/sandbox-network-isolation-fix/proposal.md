# Proposal: Sandbox Network Isolation Fix

## Why

`DockerRuntime.resolveNetworkMode` (`server/internal/sandbox/docker.go:77-82`) — hàm resolve network mode cho MỌI sandbox container (task execution, CLI-auth terminal, CLI-test terminal) — có bug đặt tên sai lệch hoàn toàn với hành vi thật:

```go
func (r *DockerRuntime) resolveNetworkMode(requested string) string {
	if requested == NetworkModeBridge || (requested == NetworkModeDefault && !r.config.DisableNetworking) {
		return "host"
	}
	return NetworkModeNone
}
```

`NetworkModeBridge` (hằng số `"bridge"`, `sandbox.go:17`) — tên gọi ngụ ý container chạy trên Docker bridge network (NAT'd, cô lập khỏi host) — trên thực tế được resolve thành literal Docker network mode `"host"`: container **chia sẻ thẳng network namespace của host** — không NAT, không cô lập, container thấy mọi interface/service đang chạy trên `127.0.0.1` của host y hệt process chạy trực tiếp trên máy.

Verify: mọi call site yêu cầu `NetworkModeBridge` — `orchestrator/sandbox.go:74`, `cli_spec_step.go:69`, `cli_engine_step.go:66`, `handler/cli_terminal.go:59` (tức toàn bộ step chạy code trong sandbox + terminal xác thực CLI tương tác) — đều nhận host networking thay vì bridge. Đây không phải regression mới: `git log -S` cho thấy hành vi này tồn tại từ trước cả khi `resolveNetworkMode` được tách thành hàm riêng (commit `9020028` chỉ refactor logic đã có sẵn, không đổi behavior).

Hệ quả kiến trúc, không chỉ naming:
- `validateExecutionPolicy` (`policy.go:27-33`) gate `SecretEnv` dựa trên `resolvedNetworkMode != NetworkModeNone` — comment/logic ở đây giả định "bridge" là mức độ exposure thấp hơn "host", nhưng trong thực tế 2 cái là **cùng một network mode** — gate này không tạo ra khác biệt bảo vệ nào giữa "task cần network hạn chế" và "task chạy full host network".
- CLI-auth/CLI-test terminal (`internal/handler/cli_terminal.go`) — nơi user gõ lệnh tương tác trong sandbox để xác thực Claude Code/Codex/Antigravity — cũng chạy với host networking, dù đây là bề mặt user-facing nhạy cảm nhất (chạy lệnh do người dùng gõ trực tiếp).

Đây là bug bảo mật/cô lập thật, không phải "code cũ nhưng vẫn đúng" — mức độ ưu tiên cao hơn các OpenSpec dọn dead-code khác trong đợt audit này.

## What Changes

### Issue 1: `resolveNetworkMode` trả về network mode cô lập thật cho `NetworkModeBridge`/default-with-networking
- Thay `"host"` bằng cách tạo/dùng 1 Docker bridge network riêng cho sandbox (không phải bridge mặc định `docker0` dùng chung — dùng network do `DockerRuntime` tự quản lý, tên cố định vd `auto-code-os-sandbox-bridge`, tạo lười lúc `NewDockerRuntime`/`Prewarm` nếu chưa có) — container attach vào network này thay vì `NetworkMode("host")`.
- `NetworkModeNone` giữ nguyên hành vi (không đổi).
- Xem design.md để quyết định giữa 3 phương án cụ thể (Docker default bridge / custom bridge network / giữ host nhưng đổi tên hằng số cho đúng sự thật) — **cần user xác nhận hướng nào trước khi code**, vì đây là thay đổi hành vi runtime có thể ảnh hưởng tool nào đang ngầm dựa vào việc gọi `localhost` của host (nếu có).

### Issue 2: Đồng bộ lại comment/naming cho đúng thực tế trong lúc sửa
- Comment trên `resolveNetworkMode` (`docs/...` dòng 72-76) hiện mô tả đúng ý định thiết kế (cô lập) nhưng sai thực tế — cập nhật lại sau khi Issue 1 xong để mô tả đúng hành vi mới.

## Capabilities

### Modified Capabilities
- `DockerRuntime.resolveNetworkMode` — network mode `"bridge"`/default giờ thật sự cô lập, không còn là alias của `"host"`.

### Removed Capabilities
- Sandbox container (task execution + CLI terminal) không còn thấy trực tiếp host network/localhost services trừ khi `NetworkModeNone` cũng không dùng (tức là trừ khi có nhu cầu rõ ràng khác, nằm ngoài phạm vi OpenSpec này).

## Impact

| Area | Files Affected |
|------|----------------|
| Backend sandbox | `server/internal/sandbox/docker.go`, `server/internal/sandbox/policy.go` (nếu comment cần cập nhật theo hành vi mới) |
| Backend tests | `server/internal/sandbox/docker_test.go` (nếu có test integration với Docker thật — kiểm tra có bị skip trong CI hay không trước khi thêm test mới) |
| Vận hành | Cần xác nhận không có workflow nào hiện tại (vd tool cần gọi `host.docker.internal` hoặc `127.0.0.1` của host) đang ngầm phụ thuộc vào host networking trước khi merge — xem design.md "Trade-offs" |
