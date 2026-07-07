# Phân Tích Cơ Chế Quản Lý Ngữ Cảnh (Context Management) của Aider

## 1. Source Snapshot
- **Source Repository:** `references/aider`
- **Reviewed Paths:** `aider/repomap.py`, `aider/queries/`
- **Snapshot Date:** 2026-07-06
- **Scope:** Phân tích cơ chế "Repo Map" - giải pháp tối ưu ngữ cảnh siêu nén của Aider.
- **Confidence:** High (Phân tích từ mã nguồn gốc).
- **Status:** Proposed (Nghiên cứu ứng dụng cho Auto Code OS).

## 2. Vấn đề cốt lõi
Khi làm việc với một dự án lớn, việc gửi toàn bộ nội dung file cho AI sẽ dẫn đến:
1. **Vượt quá Context Window** của LLM.
2. **Loãng thông tin (Hallucination)** do AI bị ngộp trong hàng ngàn dòng code không liên quan.
3. **Tốn kém chi phí Token** cực lớn.

## 3. Giải pháp "Repo Map" của Aider (Technical Deep Dive)
Thay vì ném toàn bộ codebase cho AI, Aider xây dựng một **"Bản đồ cấu trúc xương" (Skeleton Map)**. AI sẽ nhìn thấy toàn bộ tên class, tên hàm, nhưng **phần thân hàm (body) sẽ bị ẩn đi**.

Dưới đây là thuật toán 5 bước trích xuất từ lõi `repomap.py`:

### Bước 1: Trích xuất Cú pháp & Caching (AST Tagging)
- **Tốc độ:** Aider dùng `diskcache.Cache` (dựa trên SQLite) để lưu kết quả AST theo `mtime` (thời gian sửa file). Nếu file không đổi, nó bỏ qua bước parse AST để tối ưu tốc độ.
- **Tree-sitter:** Dùng `tree_sitter` để quét mã nguồn và lọc ra 2 loại Tag: `name.definition.` (`def` - nơi khai báo) và `name.reference.` (`ref` - nơi gọi hàm).
- **Fallback Lexer Hacker:** Khúc này cực hay! Nếu tree-sitter chỉ tìm thấy `def` mà không tìm thấy `ref` (do một số bộ parser ngôn ngữ bị kém), Aider tự động fallback dùng thư viện `pygments` (Lexer) để trích xuất mù (blind extract) toàn bộ các biến `Token.Name` làm `ref` để vá lỗi.

### Bước 2: Xây dựng Đồ thị phụ thuộc (MultiDiGraph)
Aider dùng thư viện **NetworkX** (`nx.MultiDiGraph`) để tạo đồ thị có hướng. Nếu file `A` gọi hàm `foo()` được khai báo ở file `B`, nó tạo 1 cạnh từ `A -> B`.
**Công thức tính Trọng số (Weight) siêu việt:**
- Trọng số cơ bản = $\sqrt{\text{số lần gọi}}$. Dùng căn bậc hai để tránh việc 1 hàm bị gọi 100 lần áp đảo hoàn toàn hàm bị gọi 1 lần (Scale down high frequency).
- **Hệ số nhân (Multiplier) thông minh:**
  - Nếu hàm tên dài >= 8 ký tự và có định dạng `snake_case/camelCase`: **Nhân 10** (Vì các hàm tên độc lạ mang nhiều ngữ cảnh hơn các hàm chung chung như `get()`).
  - Nếu tên hàm bắt đầu bằng `_` (private method): **Nhân 0.1** (Giảm độ quan trọng).
  - Nếu hàm được định nghĩa ở quá 5 nơi (generic name): **Nhân 0.1**.
  - Nếu file gọi hàm đang nằm trong khung Chat hiện tại: **Nhân 50**!

### Bước 3: Thuật toán PageRank Personalization
Aider chạy thuật toán **PageRank** (`nx.pagerank`) lên đồ thị vừa tạo.
- Điểm khởi tạo (Personalization score) của mỗi file mặc định là `100 / tổng_số_file`.
- Nếu người dùng nhắc đến tên file (`mentioned_fnames`) hoặc nhắc đến một biến (`mentioned_idents`) trong cửa sổ Chat, file đó lập tức được buff điểm cực mạnh.
- PageRank sẽ làm nhiệm vụ lan truyền điểm số từ các file đang chat sang các file core/utils vệ tinh. 

### Bước 4: Cắt tỉa Token bằng Binary Search
Giả sử LLM chỉ cho phép 1024 tokens cho Repo Map. Aider làm sao để lấy đúng 1024 tokens?
Nó dùng **Tìm kiếm nhị phân (Binary Search)** (từ dòng `676-704` trong code). Nó thử bốc Top N Tags cao điểm nhất, render thử ra chữ, rồi đếm Token. Nếu thiếu thì tăng `lower_bound`, nếu thừa thì giảm `upper_bound`. Vòng lặp dừng khi số Token tiệm cận sai số dưới 15% (`ok_err = 0.15`).

### Bước 5: Render AST siêu nén (`grep_ast`)
Aider dùng `TreeContext` từ thư viện `grep_ast` với tham số `child_context=False`. Nó in ra kết quả cuối cùng giữ nguyên thụt lề (indent) nhưng xóa rỗng ruột:
```python
# utils.py:
class Security:
    def hash_password(password):
        ...
```

## 4. Kế hoạch Tích hợp cho Auto Code OS
Nếu AI-SDLC quản lý ngữ cảnh theo tư duy của *Project Manager* (Băm nhỏ task), thì Aider quản lý ngữ cảnh theo tư duy của *Kỹ Sư Hệ Thống* (Nén kiến trúc code).

**Kết luận:** 
Auto Code OS phải sở hữu tính năng này! Kế hoạch triển khai:
1. **Thay vì tự build:** Chúng ta có thể đóng gói logic `repomap.py` của Aider thành một **Context Provider Microservice** hoặc dùng qua CLI.
2. **Khi Agent bắt đầu task:** Agent gửi danh sách các file nó định sửa, hệ thống sẽ trả về 1000 tokens bản đồ Repo Map. Nhờ đó, Agent tự biết phải import cái gì, gọi hàm nào mà không cần dò mìn (Blind guess) như trước.
