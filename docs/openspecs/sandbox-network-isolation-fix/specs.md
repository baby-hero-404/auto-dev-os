# Specs: Sandbox Network Isolation Fix

## Modified Requirements

### REQ-001: `NetworkModeBridge` cô lập container khỏi host network namespace
> ✅ Status: Fully Implemented (Option A: Docker default bridge)

**Scenario: Task chạy code với `NetworkMode=bridge`**
- WHEN 1 sandbox container được tạo với `req.NetworkMode == sandbox.NetworkModeBridge`
- THEN container **không** chia sẻ network namespace với host (không literal Docker `NetworkMode("host")`) — `docker inspect <container> --format '{{.HostConfig.NetworkMode}}'` không trả về `"host"`
- AND container vẫn có network egress ra internet (NAT qua bridge) — không phải `NetworkModeNone`, các tool cần tải package/gọi API bên ngoài (`npm install`, `claude login`, gọi LLM API) vẫn hoạt động
- AND container **không** thấy được service đang lắng nghe trên `127.0.0.1`/`localhost` của host

**Scenario: `NetworkModeDefault` khi `DisableNetworking=false`**
- WHEN `req.NetworkMode == sandbox.NetworkModeDefault` (chuỗi rỗng) và `DockerConfig.DisableNetworking == false`
- THEN hành vi giống hệt `NetworkModeBridge` ở trên (cô lập nhưng có egress) — đây là default khi không disable networking, không đổi ý nghĩa "default = có mạng", chỉ đổi *cách* có mạng

**Scenario: `NetworkModeNone` không đổi**
- WHEN `req.NetworkMode == sandbox.NetworkModeNone`, hoặc `DisableNetworking == true`
- THEN container hoàn toàn không có network — hành vi y hệt hôm nay, không có gì thay đổi ở nhánh này

**Scenario: CLI-auth/CLI-test terminal cũng được cô lập**
- WHEN user mở terminal tương tác qua `internal/handler/cli_terminal.go` (auth hoặc test flow) — request luôn dùng `NetworkModeBridge` (`cli_auth.go`/`cli_test_handler.go`)
- THEN terminal session chạy trong container cô lập network giống mọi task khác — không có ngoại lệ riêng cho terminal

## Removed Requirements
- Không có.
