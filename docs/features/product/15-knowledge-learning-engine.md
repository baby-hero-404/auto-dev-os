---
sources:
  - "server/internal/service/learning.go"
  - "server/internal/service/memory.go"
  - "server/internal/service/memory_search.go"
  - "server/internal/repository/learned_skill.go"
  - "server/pkg/models/learned_skill.go"
verified: 2026-07-30
---

# 15. Knowledge & Learning Engine

**Status:** 🟢 Implemented  
**Owner docs:** `docs/ARCHITECTURE.md`  
**Code areas:** `server/internal/service/learning.go`, `server/internal/service/memory.go`, `server/internal/repository/learned_skill.go`  
**Acceptance criteria:** Agent có thể tự động học từ các PR đã merge, trích xuất kỹ năng mới, và lưu trữ vào memory. Khi gặp task tương tự, Agent tự động recall kỹ năng thông qua BM25/Vector search.

**Mục tiêu:** Xây dựng hệ thống bộ nhớ dài hạn (Long-term memory) và vòng lặp tự học (Self-Improving Learning Loop) để AI Agent ngày càng thông minh hơn sau mỗi task, giảm thiểu sai sót lặp lại.

---

## A. Self-Improving Learning Loop

Agent tự cải thiện qua vòng lặp học tập:

1.  **Task đạt `merged`:** Job history (steps, fixes, review feedback) được đưa qua một LLM call trích xuất (hàm `learning.DetectPatterns`).
2.  **Skill Extraction:** Đề xuất các `learned_skill` record (ví dụ: "cách chạy test ở repo X", "pattern sửa lỗi Y").
    - Autonomy `supervised`: Skill ở trạng thái `draft` chờ approve.
    - Autonomy `autonomous`: Active ngay lập tức.
3.  **Context Loading:** `context_load` tìm skill theo `trigger_keywords`/title khớp với task description, nạp top kết quả vào context với budget riêng (~2k tokens).
4.  **Usage Tracking:** Khi skill được sử dụng, hệ thống tự động cập nhật `usage_count` và `success_rate`.

## B. Knowledge Memory Search

Hệ thống cung cấp cơ sở hạ tầng tìm kiếm (Memory Search) tiên tiến (`memory_search.go`):
- **BM25 & Semantic Search:** Cho phép tìm kiếm chính xác các giải pháp trước đó dựa trên keywords và vector embeddings.
- **Anti-Loop Nudge:** Trong quá trình thực thi, hệ thống chèn system nudge tổng kết "những gì đã thử & thất bại", chống lặp vòng vô ích (fail ≥3 lần cùng tool/args).

## C. Cấu trúc Learned Skill

Một kỹ năng học được (`LearnedSkill`) lưu trữ trong bảng `learned_skills` bao gồm:
- `trigger_keywords[]`: Các từ khóa kích hoạt.
- `usage_count`: Số lần được sử dụng.
- `success_rate`: Tỷ lệ thành công khi sử dụng.
- `source_task_id`: Task gốc đã sinh ra kỹ năng này.
- `status`: `draft` | `active` | `archived`.
