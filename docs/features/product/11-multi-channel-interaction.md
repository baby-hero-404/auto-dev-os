---
sources:
  - "server/**"
---

# 11. Multi-Channel Interaction (Remote Coding Sessions)

**Status:** ⚪ Deferred (until core features stabilize)  
**Owner docs:** Create a dedicated plan before implementation.  
**Code areas:** `server/internal/handler`, `server/internal/service`, future integrations for Discord/Telegram/Slack  
**Blocking decisions:** First channel to support, auth model for chat commands, approval semantics for remote actions.  
**Acceptance criteria:** Developer can create tasks, receive progress, approve/reject actions, and inspect PR status from an authenticated remote chat session.

> **⚠️ Deferred:** Feature này được hoãn có chủ đích. Ưu tiên hiện tại là hoàn thiện các feature cốt lõi: Rule System, Skill System, Gateway, và Workflow Engine. Multi-channel sẽ bắt đầu sau khi các tính năng trên ổn định production.

**Mục tiêu:** Cho phép nhà phát triển giao việc và nhận báo cáo từ AI **mọi lúc mọi nơi** — thông qua Discord, Telegram, Slack, hoặc voice note. Không cần mở dashboard, chỉ cần nhắn tin.

---

## Tính Năng Planned

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
