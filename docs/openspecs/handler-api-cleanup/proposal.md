# Proposal: Handler/API Layer Cleanup

## Why

Audit `internal/handler`, `internal/middleware`, `cmd/api`/`cmd/migrate`/`cmd/rollout-gate` phát hiện: 1 vestige bảo mật nhẹ (JWT trong query string, không còn caller thật), 2 route alias không ai gọi, 1 service khởi tạo nhưng không handler nào dùng tới, và vài chỗ logic lặp lại giữa các handler nên gộp.

## What Changes

### Issue 1: Xoá JWT-query-param fallback trong `AuthMiddleware` (ưu tiên cao nhất — an toàn, không chỉ dead code)
- `internal/handler/auth.go:72-75` — fallback đọc `?token=` từ query string cho WebSocket. Từ khi có cơ chế ws-ticket (`18bc081`), 2 route WS terminal (`cli-auth/terminal`, `cli-test/terminal`) đã nằm **ngoài** nhóm `AuthMiddleware` và dùng ticket riêng. Không còn caller nào dùng query-param JWT (verify: `grep -rn "[?&]token=" web/src` không có kết quả). Giữ lại là rủi ro: JWT lộ trong URL (log server, proxy, browser history) không cần thiết.
- Xoá đoạn fallback, chỉ còn đọc từ header `Authorization`.

### Issue 2: Xoá 2 route alias chết
- `router.go:274` `/tasks/{taskID}/restart` (alias của `/retry`) — 0 caller (`web/src` chỉ gọi `/retry`).
- `router.go:278` `/tasks/{taskID}/close` (alias của `/cancel`) — 0 caller (`web/src` chỉ gọi `/cancel`).
- Xoá cả 2 route, giữ nguyên `/retry`/`/cancel`.

### Issue 3: `NewSecretService` khởi tạo nhưng không dùng
- `cmd/api/main.go:106` tạo `service.NewSecretService(...)` chỉ để validate cipher constructor, kết quả discard (`if _, err := ...`), không handler/route nào dùng service này.
- Quyết định: (a) nếu secrets-management là tính năng dự kiến chưa ship — giữ nguyên nhưng đổi thành 1 lệnh validate cipher trực tiếp, không tạo cả service object gây hiểu nhầm là đã wiring; hoặc (b) nếu không còn trong roadmap — xoá `NewSecretService`/`internal/service/secrets.go` luôn. **Cần xác nhận với người biết roadmap trước khi chọn.**

### Issue 4: Gộp logic lặp lại (refactor thuần, không đổi hành vi)
- `internal/middleware/auth.go`'s `tokenClaims` trùng 100% field với `internal/service/auth.go`'s `TokenClaims` — gộp về dùng chung 1 struct (không có import cycle, đã verify).
- `internal/handler/pr.go`'s `Approve`/`Reject` lặp lại guard `task.Status != HumanReview && != PrReady` — extract `requireTaskStatus` helper.
- `internal/handler/cli_auth.go`/`cli_test_handler.go`'s ticket-consume→WS-upgrade boilerplate — cân nhắc gộp thành helper dùng chung (không bắt buộc, mức độ ưu tiên thấp hơn 3 issue trên).

## Capabilities

### Removed Capabilities
- JWT-in-query-param auth fallback.
- `/tasks/{taskID}/restart`, `/tasks/{taskID}/close` routes.

### Modified Capabilities
- `middleware.tokenClaims` hợp nhất với `service.TokenClaims`.

## Impact

| Area | Files Affected |
|------|----------------|
| Backend handler | `server/internal/handler/auth.go`, `server/internal/handler/router.go`, `server/internal/handler/pr.go` |
| Backend middleware | `server/internal/middleware/auth.go` |
| Backend entrypoint | `server/cmd/api/main.go` |
| Backend service | `server/internal/service/secrets.go` (tuỳ quyết định Issue 3) |
| Frontend | Không đổi — `web/src` đã không gọi các route/param bị xoá (verify trước khi merge) |
