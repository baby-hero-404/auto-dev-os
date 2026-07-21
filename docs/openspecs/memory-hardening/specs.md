# Specs: Memory Hardening

## Added Requirements

### REQ-001: Decay sweep chạy định kỳ
> ❌ Status: Not Started

**Scenario:**
- WHEN server chạy quá interval cấu hình (default 6h)
- THEN `ApplyDecay()` được gọi ít nhất 1 lần (log evidence)
- AND memory rows stale bị giảm `decay_score`; rows dưới ngưỡng TTL bị archive/xóa
- AND sweep lỗi không làm crash worker (log error, chờ tick sau)

### REQ-002: Secret redaction mở rộng
> ❌ Status: Not Started

**Scenario:**
- WHEN tool output chứa `AKIA...`, `AIza...`, JWT 3-segment, `npm_...`, `ghs_/ghu_/github_pat_...`, hoặc `Bearer <token>`
- THEN giá trị bị thay bằng `[REDACTED]` trước khi ghi vào memory
- AND corpus test ≥1 mẫu thật (đã vô hiệu) cho mỗi loại pass

### REQ-003: Embedder circuit breaker
> ❌ Status: Not Started

**Scenario:**
- WHEN `Embed()` fail N lần liên tiếp (default 5)
- THEN breaker open trong M phút (default 2), các call trong khoảng đó fail-fast không gọi HTTP
- AND `Search` khi breaker open vẫn trả kết quả (BM25 + graph, bỏ vector stream)
- AND `RecordObservation` khi breaker open vẫn lưu memory (vector null, đánh dấu cần backfill)

**Scenario:**
- WHEN breaker half-open và 1 call thành công
- THEN breaker đóng lại, vector stream hoạt động bình thường

### REQ-004: MMR diversity dedup
> ❌ Status: Not Started

**Scenario:**
- WHEN search trả top-N sau rrfMerge và tồn tại 2 kết quả cosine similarity > 0.95
- THEN MMR (λ=0.7) chọn lại từ top-2N sao cho kết quả cuối không chứa cặp near-duplicate đó (trừ khi không đủ ứng viên)
- AND thứ hạng kết quả đầu tiên (most relevant) không đổi

## Modified Requirements

### REQ-M01: Search degradation không đổi contract
> ❌ Status: Not Started

**Scenario:**
- WHEN vector stream bị bỏ (breaker open)
- THEN response schema của Search không đổi; caller không cần biết degradation xảy ra

## Removed Requirements
- Không có.
