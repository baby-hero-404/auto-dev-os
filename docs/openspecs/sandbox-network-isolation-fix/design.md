# Design: Sandbox Network Isolation Fix

## Root cause

```go
// server/internal/sandbox/docker.go:77
func (r *DockerRuntime) resolveNetworkMode(requested string) string {
	if requested == NetworkModeBridge || (requested == NetworkModeDefault && !r.config.DisableNetworking) {
		return "host"   // <-- literal Docker network mode string, không phải bridge thật
	}
	return NetworkModeNone
}
```
`"host"` được truyền thẳng vào `container.NetworkMode(resolvedNetworkMode)` (`docker.go:217`, `:309`) — Docker hiểu đúng nghĩa đen: container share network namespace với host daemon, không NAT, không cô lập.

## Phương án sửa (cần chọn trước khi code — xem "Open question" cuối file)

### Phương án A — Docker default bridge (khuyến nghị)
```go
func (r *DockerRuntime) resolveNetworkMode(requested string) string {
	if requested == NetworkModeBridge || (requested == NetworkModeDefault && !r.config.DisableNetworking) {
		return "bridge" // Docker's own default bridge network — NAT'd egress, no host-namespace sharing
	}
	return NetworkModeNone
}
```
Đơn giản nhất — Docker daemon đã có sẵn network `bridge` mặc định (`docker0`), không cần tạo/quản lý network riêng. Container vẫn egress internet qua NAT bình thường. Nhược điểm: mọi sandbox container (của mọi task, mọi org) cùng nằm trên 1 bridge network mặc định → về lý thuyết có thể thấy nhau qua IP nội bộ nếu Docker's inter-container communication (`icc`) bật (mặc định Docker bật `icc=true` trên bridge mặc định). Với threat model hiện tại (mỗi sandbox chạy code của 1 task, không multi-tenant network segmentation là yêu cầu đã có), đây là cải thiện lớn so với host networking, dù chưa hoàn hảo.

### Phương án B — Custom bridge network riêng cho sandbox, `icc=false`
Tạo network riêng lúc `NewDockerRuntime`/`Prewarm` nếu chưa tồn tại:
```go
_, err := r.client.NetworkCreate(ctx, "auto-code-os-sandbox-bridge", network.CreateOptions{
	Driver: "bridge",
	Options: map[string]string{"com.docker.network.bridge.enable_icc": "false"},
})
```
rồi dùng tên network này thay vì `"bridge"`. Cô lập tốt hơn A (container của các task khác nhau không thấy nhau), nhưng thêm state cần quản lý (network lifecycle, cleanup, race lúc nhiều `DockerRuntime` instance cùng tạo network lần đầu — cần `IfNameExists`-style idempotent create).

### Phương án C — Giữ `"host"` nhưng đổi tên hằng số + comment cho đúng sự thật
Không đổi hành vi runtime, chỉ sửa `NetworkModeBridge` → đổi tên thành thứ trung thực (vd `NetworkModeHost`) và cập nhật mọi call site + comment liên quan (`policy.go`'s exposure-gating logic) để không còn giả định sai là có cô lập. **Không khuyến nghị** — đây là "sửa lời nói dối bằng cách thừa nhận nó" thay vì sửa lỗ hổng cô lập thật; chỉ hợp lý nếu có lý do vận hành cụ thể cần host networking (vd sandbox cần gọi service khác đang chạy trên host qua `127.0.0.1` — chưa thấy bằng chứng nào trong codebase cho nhu cầu này, xem "Not flagged" trong audit report gốc).

**Khuyến nghị: Phương án A** cho phase này (đóng gap cô lập lớn nhất — thoát host namespace — với thay đổi tối thiểu), Phương án B là follow-up tự nhiên nếu multi-tenant network isolation giữa các task trở thành yêu cầu rõ ràng.

## Security & Risk Mitigation

| Risk | Mitigation |
|---|---|
| Đổi từ host → bridge có thể phá vỡ 1 workflow đang ngầm dựa vào việc gọi `127.0.0.1`/host service từ trong sandbox (chưa phát hiện case nào qua audit, nhưng chưa chứng minh được là không có) | Trước khi merge: chạy full test suite tương tác thật với Docker runtime (không chỉ `StubRuntime`), thử `make sandbox-build` + chạy 1 task thật end-to-end (backend code step + CLI auth terminal) để xác nhận không có lệnh nào fail vì thiếu quyền truy cập host |
| `validateExecutionPolicy`'s `SecretEnv` gate dựa trên `resolvedNetworkMode != NetworkModeNone` — logic này không đổi ý nghĩa sau fix (bridge vẫn "có network" nên vẫn bị gate) | Không cần sửa `policy.go` logic, chỉ cập nhật comment nếu đang mô tả sai |

## Trade-offs

- **Không làm Phương án B (custom network + icc=false) ngay trong lần sửa này** — ưu tiên đóng gap nghiêm trọng nhất (host-namespace escape) trước, tránh gộp 1 fix an toàn cấp thiết với 1 thay đổi hạ tầng (network lifecycle management) rủi ro cao hơn, review lâu hơn.

## Open question — cần user xác nhận trước khi code
1. Chọn Phương án A hay B? (mặc định đề xuất A nếu không có phản hồi khác)
2. Có cần giữ 1 cờ config (`SANDBOX_NETWORK_MODE=host`) để rollback nhanh nếu phát hiện breakage sau khi deploy, hay chấp nhận thẳng bridge không cần escape hatch?
