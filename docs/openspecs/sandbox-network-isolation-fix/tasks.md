# Tasks: Sandbox Network Isolation Fix

## P0 — decision gate

### Task 0.1: Chốt phương án sửa
- [x] User xác nhận Phương án A/B/C (design.md "Open question") trước khi động code — đây là thay đổi hành vi runtime, không phải cleanup an toàn tuyệt đối.
- Satisfies: (gate cho mọi task dưới)

## P0 — fix

### Task 1.1: Sửa `resolveNetworkMode`
- [x] Implement theo phương án đã chốt (design.md).
- [x] Cập nhật comment trên `resolveNetworkMode` cho đúng hành vi mới.
- Satisfies: REQ-001

### Task 1.2: Verify thật với Docker runtime (không chỉ `StubRuntime`)
- [x] `make sandbox-build` (rebuild image nếu cần).
- [x] Chạy 1 task thật (code_backend hoặc tương đương) end-to-end với `SANDBOX_RUNTIME=docker`, xác nhận vẫn: cài được dependency (`npm install`/`go mod`), gọi được LLM API/CLI login (egress internet vẫn hoạt động).
- [x] Mở CLI-auth terminal (`ai-providers` page) thật, xác nhận `claude login`/tương tự vẫn hoàn tất được (đã test luồng này trong session hôm nay với host networking — so sánh lại sau khi đổi sang bridge).
- [x] `docker inspect <container> --format '{{.HostConfig.NetworkMode}}'` trong lúc container đang chạy, xác nhận không còn `"host"`.
- Satisfies: REQ-001 (verification)

### Task 1.3: Unit test cho `resolveNetworkMode`
- [x] `docker_test.go` (hoặc file test mới nếu chưa có unit test cho hàm này): test 3 nhánh (`NetworkModeBridge`, `NetworkModeDefault` với `DisableNetworking` true/false, `NetworkModeNone`) trả đúng giá trị theo phương án đã chọn — không cần Docker daemon thật cho unit test này (test hàm thuần, không gọi `client.ContainerCreate`).
- Satisfies: REQ-001

## Self-review checklist

| REQ | Task |
|---|---|
| REQ-001 | 0.1, 1.1, 1.2, 1.3 |
