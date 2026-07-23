---
sources:
  - "server/**"
  - "server/internal/orchestrator/llmrunner/outputfilter/**"
  - "server/internal/orchestrator/llmrunner/toolloop.go"
verified: 2026-07-23
---

# 02. Context Pruning & Harness Independence

**Status:** 🟢 Implemented (audited 2026-07-12: PageRank buffing in `repomap/ranking.go`, binary-search pruning in `repomap/pruning.go`, and Harness Independence end-to-end in `steps/review.go` + `pkg/llm/router.go` — including the graceful-fallback edge case)
**Owner docs:** `docs/features/engineering/01-context-management.md` (shares the Repo Map / Context Engine this builds on)
**Code areas:** `server/internal/context/` (repomap, provider), `server/internal/prompts/` (PromptAssembler), AI Gateway (model review isolation)

**Mục tiêu:** Đặc tả hai cơ chế nâng cao nhằm tối ưu hóa bộ điều phối (Orchestrator) của Auto Code OS, được đúc kết từ kiến trúc của Aider và AI-SDLC:
1. **Thuật toán Cắt tỉa Token Nhị phân & Ưu tiên PageRank (Context Pruning & PageRank Personalization):** Tối ưu hóa việc nạp ngữ cảnh (Repo Map) sao cho luôn vừa vặn với ngân sách Token (Token Budget), tránh tràn RAM và rò rỉ ngữ cảnh (hallucination).
2. **Độc lập Mô hình (Harness Independence):** Cơ chế chống "điểm mù đồng thuận" (consensus bias) bằng cách ép buộc luồng Kiểm duyệt (Review) phải sử dụng một mô hình AI khác với mô hình đã sinh ra mã nguồn.

---

## 1. Feature 1: Binary Search Token Pruning & PageRank Personalization

### 1.1 Vấn đề
Việc gửi toàn bộ cây thư mục hoặc nội dung file cho LLM dẫn đến lãng phí Token, tăng chi phí và giảm độ chính xác. Giải pháp sử dụng AST Repo Map (bản đồ phụ thuộc mã nguồn không có thân hàm) là rất tốt, nhưng ta cần một cơ chế tự động ép dung lượng của Repo Map này luôn nằm trong một giới hạn Token an toàn (ví dụ: tối đa 1500 tokens).

### 1.2 Thiết kế Cơ chế
- **Ưu tiên ngữ cảnh (PageRank Buffing):** 
  - Context Engine sẽ tính toán tầm quan trọng của các file trong Repo Map dựa trên thuật toán PageRank trên đồ thị phụ thuộc (Dependency Graph).
  - **Buff theo Task:** Nếu một file được liệt kê rõ ràng trong mảng `FileDependencies` của một Task (do Agent Planner chỉ định), điểm PageRank của file đó sẽ được nhân hệ số cực lớn (vd: x50). Điều này ép hệ thống phải giữ lại AST của file đó và các file vệ tinh sát nhất với nó.
- **Cắt tỉa Nhị phân (Binary Search Token Pruning):**
  - Hệ thống định nghĩa một `TokenBudget` (Ngân sách Token) cho Repo Map.
  - Sử dụng thuật toán Tìm kiếm Nhị phân (Binary Search) để chọn top `N` node (AST Tags) có điểm PageRank cao nhất. 
  - Hệ thống sẽ render thử thành chuỗi văn bản, đếm số token. Nếu vượt budget, giảm `N` (upper_bound). Nếu thiếu budget, tăng `N` (lower_bound). Lặp lại cho đến khi số token tiệm cận sai số 10-15% so với budget đề ra.

---

## 2. Feature 2: Harness Independence (Model Review Isolation)

### 2.1 Vấn đề
Khi một LLM (ví dụ: `claude-3-5-sonnet`) vừa viết code, sau đó lại đóng vai trò là Reviewer để tự đánh giá code của chính nó, mô hình có xu hướng thiên vị (bias) và dễ dàng bỏ qua các lỗi logic tiềm ẩn do chung một mạng nơ-ron nhận thức.

### 2.2 Thiết kế Cơ chế
- **Cách ly Mô hình (Model Isolation):** 
  - Orchestrator (hoặc AI Gateway) khi điều phối bước `Review` sẽ kiểm tra mô hình đã được sử dụng ở bước `Code`.
  - Hệ thống sẽ cố gắng chọn một **mô hình khác** để làm Reviewer. Yêu cầu ở đây là **chỉ cần khác tên mô hình (Model ID)**, không bắt buộc phải khác nhà cung cấp (Provider). 
    - *Ví dụ hợp lệ:* Coder dùng `gpt-4o-mini`, Reviewer dùng `gpt-4o`.
    - *Ví dụ hợp lệ:* Coder dùng `claude-3-5-sonnet`, Reviewer dùng `gpt-4o`.
- **Cơ chế Dự phòng (Graceful Fallback):**
  - Trong trường hợp môi trường của người dùng chỉ cấu hình duy nhất một mô hình (hoặc các mô hình khác đều đang lỗi/hết quota), hệ thống sẽ không chặn (block) luồng làm việc.
  - **Fallback:** Hệ thống sẽ tự động hạ cấp cấu hình và dùng lại chính mô hình Coder để thực hiện bước Review, đảm bảo tiến độ công việc không bị gián đoạn. Ghi log cảnh báo nhẹ (Warning) cho người dùng về việc giảm chất lượng kiểm duyệt.

---

## 3. Feature 3: Tool-Output Filtering Pipeline

### 3.1 Vấn đề
Giới hạn duy nhất trước đây cho tool result là hard-cut 8000 ký tự (`maxToolResultChars`, `toolloop.go`) — không có lớp lọc nội dung nào. Build log 50KB bị cắt mù ở 8000 ký tự có thể mất chính dòng error; log lặp hàng nghìn dòng giống nhau chiếm chỗ context vô ích.

### 3.2 Thiết kế Cơ chế
Package `llmrunner/outputfilter/` — chuỗi filter thuần Go (không LLM), chạy **trước** hard-cut (hard-cut vẫn giữ làm safety net cuối):
1. **Dedup dòng lặp:** N dòng identical liên tiếp → 1 dòng + `[repeated N times]`.
2. **Error-priority truncation:** khi phải cắt, giữ ưu tiên dòng match error pattern (error/fail/panic/FAIL/✗/warning) + K dòng context quanh chúng + đầu/cuối output; cắt phần "im lặng" ở giữa trước.
3. **Path prefix compression:** đường dẫn tuyệt đối lặp lại nhiều lần → rút về relative sau lần đầu.
4. **ANSI/control strip:** mã màu terminal, carriage-return progress bar.

Mỗi tool khai báo **profile** riêng (metadata trong `tool/tools/*.go`, không đổi logic tool): `build/test` → error-priority mạnh; `git diff` → không dedup (mỗi dòng có nghĩa); `read file` → không filter (đã có bound riêng). Tool không khai báo dùng profile mặc định (strip ANSI + dedup). Mỗi lần filter chạy, log metric `outputfilter: in=X out=Y saved=Z%` để đo hiệu quả nén thực tế.

---

## 4. Lộ trình Triển khai (Implementation Phasing)
- **Phase 1:** Tích hợp logic PageRank Buffing vào AST Graph dựa trên `FileDependencies` của Task.
- **Phase 2:** Cài đặt vòng lặp Binary Search Pruning trong `ContextEngine` trước khi trả kết quả cho `PromptAssembler`.
- **Phase 3:** Cập nhật AI Gateway để hỗ trợ fallback logic khi chuyển đổi trạng thái từ `Coding` sang `Reviewing` (Harness Independence check).
