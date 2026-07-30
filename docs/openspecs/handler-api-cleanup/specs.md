# Specs: Handler/API Layer Cleanup

## Removed Requirements

### REQ-R01: JWT-query-param auth fallback bị xoá
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN 1 request tới route dùng `AuthMiddleware` mang JWT trong `Authorization` header
- THEN xác thực thành công như hôm nay (không đổi)

**Scenario: Request chỉ mang `?token=` không có header**
- WHEN 1 request tới route dùng `AuthMiddleware` chỉ có `?token=<jwt>` trong query, không có `Authorization` header
- THEN bị từ chối 401 (trước đây sẽ pass) — đây là thay đổi hành vi có chủ đích, verify trước khi merge rằng không frontend/tool nào còn dựa vào query-param

### REQ-R02: `/tasks/{taskID}/restart`, `/tasks/{taskID}/close` bị xoá
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN gọi `POST /tasks/{taskID}/retry` hoặc `POST /tasks/{taskID}/cancel`
- THEN hành vi không đổi (route chính, không phải alias, vẫn còn nguyên)

**Scenario:**
- WHEN gọi `POST /tasks/{taskID}/restart` hoặc `POST /tasks/{taskID}/close` sau khi xoá
- THEN trả 404 (route không còn tồn tại) — chấp nhận được vì đã verify 0 caller thật

## Modified Requirements

### REQ-M01: `middleware.tokenClaims` hợp nhất với `service.TokenClaims`
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN `InjectClaimsFromJWT` parse 1 JWT hợp lệ (không verify signature — đây là unverified pre-parse, giữ nguyên mục đích cũ)
- THEN kết quả claims giống hệt hôm nay, chỉ khác là dùng chung struct `service.TokenClaims` thay vì bản copy riêng

## Not in scope (giữ nguyên có chủ đích)
- `handler.authClaimsKey`/`contextKey` (auth.go:11-13) — key context riêng cho claims **đã verify** (post-`AuthMiddleware`), khác tầng tin cậy với `tokenClaims` (pre-verify) — không gộp 2 cái này, đây là tách biệt có chủ đích.
