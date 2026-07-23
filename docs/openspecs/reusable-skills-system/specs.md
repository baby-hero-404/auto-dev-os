# Specs: Reusable Skills System

## Added Requirements

### REQ-001: Extraction sau merged
> ✅ Status: Done

**Scenario:**
- WHEN task chuyển `merged`
- THEN extraction chạy trên job history, tạo 0-2 skill records
- AND supervised → `draft`; autonomous → `active`
- AND extraction fail không ảnh hưởng task status (best-effort, log)

### REQ-002: Skill loading
> ✅ Status: Done

**Scenario:**
- WHEN context_load chạy cho task mới có description match trigger_keywords của skill active
- THEN top-3 skills theo score được thêm vào context trong section riêng, tổng ≤2k tokens
- AND không match → section vắng mặt, context như cũ

### REQ-003: Usage tracking
> ✅ Status: Done

**Scenario:**
- WHEN task có skill loaded đạt merged (hoặc failed)
- THEN `usage_count` +1 và `success_rate` cập nhật theo kết quả

### REQ-004: Mid-task nudge
> ✅ Status: Done

**Scenario:**
- WHEN tool-loop đạt iteration 15, 30, …
- THEN 1 message nudge tóm tắt tools đã fail (tên + số lần) được chèn trước call tiếp theo
- AND WHEN cùng tool+args fail ≥3 lần THEN nudge nêu đích danh và đề nghị đổi cách tiếp cận

### REQ-005: Skills UI
> ✅ Status: Done (backend only — CRUD API implemented; UI page deferred, see implementation notes)

**Scenario:**
- WHEN user mở trang Skills của project
- THEN thấy list (title, status, usage, success rate, source task link), edit/activate/deactivate/approve draft được

## Modified Requirements

### REQ-M01: DetectPatterns tương thích
> ✅ Status: Done

**Scenario:**
- WHEN pipeline learning cũ chạy
- THEN hành vi hiện có của DetectPatterns không mất (extraction là lớp thêm, không thay thế)

## Removed Requirements
- Không có.
