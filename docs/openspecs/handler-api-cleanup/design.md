# Design: Handler/API Layer Cleanup

## Issue 1: JWT-query-param fallback

`internal/handler/auth.go` (trong `AuthMiddleware`, quanh dòng 72-75) — xoá đoạn:
```go
// Fallback to query param for WebSocket connections
if tokenStr == "" {
	tokenStr = r.URL.Query().Get("token")
}
```
giữ nguyên phần đọc `Authorization` header phía trên. Trước khi xoá: `grep -rn "[?&]token=" web/src` phải rỗng (đã verify lúc audit — re-verify lại 1 lần nữa ngay trước khi merge vì frontend có thể đổi giữa lúc này).

## Issue 2: Route alias

`router.go`: xoá 2 dòng
```go
r.Post("/tasks/{taskID}/restart", workflowH.Retry)
r.Post("/tasks/{taskID}/close", workflowH.Cancel)
```
Không đổi `workflowH.Retry`/`workflowH.Cancel` — vẫn wired ở `/retry`/`/cancel`.

## Issue 3: `NewSecretService`

`cmd/api/main.go:106`:
```go
if _, err := service.NewSecretService(secretRepo, cfg.Auth.JWTSecret); err != nil {
	return fmt.Errorf(...)
}
```
Nếu quyết định (a) — không cần secrets feature: thay bằng validate cipher trực tiếp (không qua service wrapper), vd gọi thẳng hàm cipher-construction mà `NewSecretService` đang dùng nội bộ, hoặc xoá check này luôn nếu cipher construction không thể fail theo cách cần catch sớm. Nếu quyết định (b) — giữ cho tương lai: đổi comment cho rõ "constructed here only to validate cipher config at boot; no handler wires this yet" để người đọc sau không tưởng nhầm là đã có route.

## Issue 4: Gộp logic lặp

### `tokenClaims` → `service.TokenClaims`
`internal/middleware/auth.go`: xoá struct `tokenClaims` riêng, `InjectClaimsFromJWT` unmarshal thẳng vào `service.TokenClaims` (import `internal/service`, đã verify không tạo cycle: `grep -rln "internal/middleware" internal/service/` rỗng).

### `pr.go` — `requireTaskStatus` helper
```go
func requireTaskStatus(w http.ResponseWriter, task *models.Task, allowed ...string) bool {
	for _, s := range allowed {
		if task.Status == s {
			return true
		}
	}
	writeError(w, http.StatusBadRequest, "task is not awaiting PR review (status: "+task.Status+")")
	return false
}
```
`Approve`/`Reject` gọi `if !requireTaskStatus(w, task, models.TaskStatusHumanReview, models.TaskStatusPrReady) { return }` thay vì lặp lại `if`.

## Trade-offs

- **Không đổi WS ticket mechanism** — Issue 1 chỉ xoá fallback không còn dùng, không đụng tới cơ chế ticket đang hoạt động tốt.
- **Ticket-consume→WS-upgrade helper dùng chung giữa `cli_auth.go`/`cli_test_handler.go`** — để ngoài phạm vi task bắt buộc (tasks.md đánh dấu optional) vì rủi ro thấp nhưng lợi ích cũng thấp, không đáng tách riêng review.
