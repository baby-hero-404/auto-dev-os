---
sources:
  - "server/internal/handler/cli_auth.go"
  - "server/cmd/minttoken/main.go"
verified: 2026-07-30
---

# 11. Multi-Channel Interaction (Remote Coding Sessions)

**Status:** 🟡 In Progress (Websocket terminal implemented; chat integrations planned)  
**Owner docs:** `docs/ARCHITECTURE.md`  
**Code areas:** `server/internal/handler/cli_auth.go`, `server/internal/handler`, `server/cmd/minttoken`  
**Blocking decisions:** First chat channel to support (Discord vs Slack).  
**Acceptance criteria:** Developer can create tasks, receive progress, approve/reject actions from authenticated remote sessions, and securely access CLI via WebSocket terminal.

**Mục tiêu:** Cho phép nhà phát triển giao việc và nhận báo cáo từ AI **mọi lúc mọi nơi** — thông qua Terminal (WebSocket), Discord, Telegram, Slack, hoặc voice note. Không cần mở dashboard, chỉ cần nhắn tin hoặc thao tác qua command line an toàn.

---

## Secure CLI & Remote Terminal (Implemented)

Các tính năng nền tảng cho truy cập từ xa đã được triển khai thông qua giao thức WebSocket:

*   **WebSocket Ticket-Based Authentication:** Truy cập remote terminal và CLI được bảo mật qua cơ chế ticket-based auth trên WebSocket. Để test cục bộ, có thể dùng công cụ `minttoken` (`server/cmd/minttoken/main.go`) sinh JWT token hợp lệ cho admin.
*   **PTY Terminal Resizing:** Hỗ trợ mô phỏng terminal PTY đầy đủ (bao gồm tự động resize) qua CLI auth handler, mang lại trải nghiệm mượt mà khi kết nối remote.
*   **Terminal WS Reconnect:** Cơ chế tự động kết nối lại (reconnect resilience) khi WebSocket bị gián đoạn, đảm bảo không mất session đang chạy.
*   **RBAC & Verified Auth Claims:** Các hành động từ xa được phân quyền chặt chẽ. Hệ thống luôn tra cứu `verified auth claims` để kiểm tra quyền truy cập (RBAC) trước khi cho phép thực thi lệnh qua CLI terminal.

---

## Tính Năng Planned (Chat & Voice)

*   **Chatbot đa kênh:** Tích hợp Discord, Telegram, Slack — tạo thành Multi-channel Inbox. Một lệnh chat = một task AI.
*   **Streaming tiến độ:** Cập nhật tiến độ task trực tiếp vào kênh chat — "Agent đang viết code...", "Test passed ✅", "PR created 🔗".
*   **Can thiệp & phê duyệt:** Approve/reject PR ngay trong chat. Ví dụ: `/approve task-123` hoặc `/reject task-123 "fix error handling"`.
*   **Voice notes:** Chuyển ghi chú giọng nói thành text để AI xử lý — brainstorm bằng giọng nói, nhận code bằng PR.

---

**Dự án tham khảo:**

| Dự án | Lý do tham khảo |
|:------|:----------------|
| OpenClaw | Multi-channel gateway, sandboxing |
| Free Claude Code | Drop-in proxy, chat-to-task pattern |
