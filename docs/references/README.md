# 📚 Danh Mục Tài Liệu Tham Khảo & Học Hỏi Từ Các Dự Án Mã Nguồn Mở

Thư mục này chứa các phân tích chi tiết về 12 dự án mã nguồn mở và tài liệu kỹ thuật trong thư mục `resources/`, đóng vai trò định hình kiến trúc phát triển cho **Auto Code OS**.

## 📁 Sơ Đồ Cấu Trúc Restructure Mới (`docs/references/`)

Dưới đây là cấu trúc thư mục tài liệu tham khảo đã được sắp xếp lại gọn gàng, chia nhỏ theo từng dự án:

```
docs/references/
├── README.md                           # File tổng hợp chung (Bản này)
├── 9router.md                          # Local AI Gateway, RTK Token Saver
├── ai-sdlc.md                          # Chu trình SDLC tự chủ, Cross-Harness Review
├── agentmemory.md                      # Bộ nhớ 4 tầng, RAG 3 luồng (RRF), Ebbinghaus curve
├── openspec.md                         # Phát triển song song, 3-way Spec Merge
├── multica.md                          # CLI/Daemon, Workspace Garbage Collection
├── openclaw.md                         # Docker Sandbox, Multi-channel Inbox
├── superpowers.md                      # Quy trình TDD (Red-Green-Refactor)
├── hermes-agent.md                     # Vòng lặp tự học (Closed learning loop), RPC Sub-agents
├── free-claude-code.md                 # API Proxy CLI, Remote code control
├── prompt-base.md                      # JIT Skill Loading, Minimal Viable Context
├── antigravity-awesome-skills.md       # Thư viện 1,470+ Kỹ năng (Seed Data)
└── llm-key-manager.md                  # Mã hóa vault, Effective Score key routing
```

---

## 📊 Bảng Tổng Hợp So Sánh & Bài Học Kinh Nghiệm

| Dự Án | Tính Năng Hay Nhất | Bài Học Lớn Nhất Cho Auto Code OS | Tài Liệu Chi Tiết |
|---|---|---|---|
| **Multica** | CLI & Local Daemon nhận task qua Websockets | Tự động dọn dẹp Workspace cũ (GC) để tránh đầy đĩa | [Xem chi tiết](file:///home/ubuntu/my_projects/auto_code_os/docs/references/multica.md) |
| **OpenClaw** | Control plane định tuyến kênh chat, Docker Sandbox | Xây dựng Sandbox Docker phân quyền bảo mật cao | [Xem chi tiết](file:///home/ubuntu/my_projects/auto_code_os/docs/references/openclaw.md) |
| **AI-SDLC** | Vòng lặp SDLC kín, Review chéo (Cross-Harness) | Thiết lập chỉ số Tự chủ (Autonomy) và chặn cost LLM | [Xem chi tiết](file:///home/ubuntu/my_projects/auto_code_os/docs/references/ai-sdlc.md) |
| **9router** | Nén token RTK, Fallback model 3 tầng | Nén terminal log của test run trước khi gửi AI | [Xem chi tiết](file:///home/ubuntu/my_projects/auto_code_os/docs/references/9router.md) |
| **OpenSpec** | Phân rã spec thành sub-tasks, Merge spec song song | Dùng Fingerprint Hash để chống xung đột ghi đè spec | [Xem chi tiết](file:///home/ubuntu/my_projects/auto_code_os/docs/references/openspec.md) |
| **AgentMemory** | Bộ nhớ 4 lớp (Working, Episodic, Semantic, Procedural) | Tìm kiếm RAG 3 luồng tích hợp RRF (Vector + BM25) | [Xem chi tiết](file:///home/ubuntu/my_projects/auto_code_os/docs/references/agentmemory.md) |
| **Superpowers** | Cưỡng chế pipeline, TDD Red-Green-Refactor | Bắt AI tạo test case fail (RED) trước khi sửa code | [Xem chi tiết](file:///home/ubuntu/my_projects/auto_code_os/docs/references/superpowers.md) |
| **Hermes Agent** | Tự động đúc kết Skill mới sau task khó | Vòng lặp tự học (Self-Improving) viết playbook | [Xem chi tiết](file:///home/ubuntu/my_projects/auto_code_os/docs/references/hermes-agent.md) |
| **Free Claude Code** | API Proxy ghi đè endpoint cục bộ | Bọc (wrap) các công cụ AI CLI tiêu chuẩn vào sandbox | [Xem chi tiết](file:///home/ubuntu/my_projects/auto_code_os/docs/references/free-claude-code.md) |
| **Prompt Base** | Nạp Skill JIT (Just-In-Time) | Tối ưu prompt động, tránh framework tax | [Xem chi tiết](file:///home/ubuntu/my_projects/auto_code_os/docs/references/prompt-base.md) |
| **Awesome Skills** | 1,470+ Kỹ năng đóng gói bằng Markdown di động | Dùng làm seed data nạp sẵn vào Database PostgreSQL | [Xem chi tiết](file:///home/ubuntu/my_projects/auto_code_os/docs/references/antigravity-awesome-skills.md) |
| **LLM Key Manager** | Vault client bảo mật, thuật toán Effective Score | Chạy công thức Effective Score chọn API Key tối ưu | [Xem chi tiết](file:///home/ubuntu/my_projects/auto_code_os/docs/references/llm-key-manager.md) |

---

## 🛠️ Tổng Hợp Sự Cố Patching & Giải Pháp Khắc Phục (Patching Remediation)

*(Thông tin tổng hợp và mở rộng từ báo cáo sự cố `llm-patching-issue-report.md` cũ)*

### Vấn Đề
Khi AI sinh file patch (unified diff) để áp dụng vào codebase thông qua lệnh `git apply`, hệ thống rất dễ gặp lỗi crash luồng (`exit code 2`) do:
1. Thiếu thông tin dòng trống hoặc whitespace không khớp.
2. Metadata header của diff hunk bị lệch số dòng.
3. Không thể tự phục hồi khiến cả task bị hủy bỏ lãng phí token trước đó.

### Giải Pháp Đã Thử Nghiệm & Đề Xuất Cho Auto Code OS
1. **Cơ chế Phục Hồi Nhẹ (Graceful Degradation):** Khi `git apply` lỗi, hệ thống không crash mà đánh dấu là *Sự kiện có thể khôi phục*. Task sẽ gửi file diff bị lỗi sang bước `ReviewerAgent` để AI tự chỉnh sửa lại các hunk bị lệch.
2. **Hybrid Patch Engine:** Xây dựng một module Go có khả năng tự động chuyển đổi giữa 2 cơ chế:
    *   *Ưu tiên 1:* **Unified Diff** (`git apply` nhanh, chính xác nếu diff chuẩn).
    *   *Ưu tiên 2 (Fallback):* **Search & Replace Block Matching** (Fuzzy matching). AI chỉ cần sinh ra khối text cũ và khối text mới. Hệ thống Go sẽ tự động tìm kiếm khối cũ trong codebase (cho phép sai lệch nhỏ về whitespace) và thay thế bằng khối mới.
3. **Patch Validator:** Chạy bộ kiểm duyệt cú pháp diff trước khi thực hiện ghi xuống ổ đĩa, đảm bảo độ an toàn cho mã nguồn.
