# Tasks: Handler/API Layer Cleanup

## P0 — security-adjacent

### Task 1.1: Xoá JWT-query-param fallback
- [x] Re-verify `grep -rn "[?&]token=" web/src` rỗng.
- [x] Xoá fallback trong `AuthMiddleware` (`auth.go`).
- [x] `handler_test.go`/`auth_test.go`: thêm test request chỉ có `?token=` (không header) → 401.
- Satisfies: REQ-R01

## P1 — dead routes + duplicate logic

### Task 1.2: Xoá route alias
- [x] Re-verify `grep -rn "restart\|/close" web/src` không match route này.
- [x] Xoá 2 dòng route trong `router.go`.
- [x] `router_test.go` (nếu có test liệt kê route list): cập nhật.
- Satisfies: REQ-R02

### Task 1.3: Quyết định + xử lý `NewSecretService`
- [x] Điều tra kỹ hơn trước khi hỏi: `NewSecretService(secretRepo, cfg.Auth.JWTSecret)` nội bộ chỉ gọi lại `NewSecretCipher(keyMaterial)` — **đúng cipher construction đã được validate 4 dòng phía trên** (biến `secretCipher` dùng thật cho `credentialPoolSvc`) — rồi discard kết quả. Đây không phải quyết định sản phẩm, chỉ là gọi trùng lặp 100% một validation đã chạy — xoá thẳng, không cần hỏi.
- [x] Xoá dòng gọi `NewSecretService` + biến `secretRepo` (chỉ tồn tại để phục vụ dòng đó) trong `cmd/api/main.go`. `internal/service/secrets.go` (SecretService/SecretRepo) giữ nguyên — vẫn là code sẵn sàng cho tính năng Secrets chưa wiring handler, chỉ bỏ validation trùng ở boot.
- Satisfies: (không có REQ riêng — hoá ra không cần xác nhận sản phẩm, chỉ là redundancy thuần)

### Task 1.4: Gộp `tokenClaims`/`service.TokenClaims`
- [x] Sửa `internal/middleware/auth.go` theo design.md.
- [x] `go build ./...`, `go test ./internal/middleware/...`.
- Satisfies: REQ-M01

### Task 1.5: Extract `requireTaskStatus` trong `pr.go`
- [x] Thêm helper, thay 2 chỗ lặp trong `Approve`/`Reject`.
- [x] `pr_test.go`: chạy lại test hiện có, xác nhận behavior không đổi.
- Satisfies: (refactor, không có REQ riêng)

## Self-review checklist

| REQ | Task |
|---|---|
| REQ-R01 | 1.1 |
| REQ-R02 | 1.2 |
| REQ-M01 | 1.4 |
