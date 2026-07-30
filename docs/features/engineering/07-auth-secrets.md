---
sources:
  - "server/internal/service/auth.go"
  - "server/internal/service/secrets.go"
  - "server/cmd/minttoken/"
verified: 2026-07-30
---

# 07. Authentication & Secrets Management

**Status:** 🟢 Implemented  
**Owner docs:** `docs/ARCHITECTURE.md`  
**Code areas:** `server/internal/service/auth.go`, `server/internal/service/secrets.go`, `server/cmd/minttoken/`

**Mục tiêu:** Quản lý vòng đời xác thực của user và agent, đồng thời bảo vệ các thông tin nhạy cảm (API Keys, môi trường biến) trong quá trình lưu trữ và thực thi.

---

## 1. Authentication Mechanisms

- **JWT Authentication:** Hệ thống sử dụng JSON Web Tokens (JWT) để xác thực các request API và kết nối WebSocket (Ticket-Based Auth cho Terminal).
- **CLI Development (`minttoken`):** Công cụ CLI nội bộ (`server/cmd/minttoken/main.go`) hỗ trợ các nhà phát triển sinh JWT tokens hợp lệ cục bộ (local testing) để dễ dàng kiểm thử các endpoint yêu cầu quyền Admin mà không cần qua luồng login đầy đủ.
- **Verified Auth Claims:** Hệ thống trích xuất `org_id`, `user_id` và các claims (phân quyền) từ JWT để đảm bảo Role-Based Access Control (RBAC).

## 2. Secrets Encryption (Bảo Mật API Keys)

- **AES-256-GCM:** Tất cả các secret (API keys của LLM, biến môi trường `.env` truyền cho CLI Agent) đều được mã hóa bằng thuật toán `AES-256-GCM` trước khi lưu xuống database.
- Không bao giờ trả bản rõ (plaintext) của keys về phía frontend.
- Cung cấp cơ chế giải mã an toàn (Just-in-Time Decryption) ngay trước khi tiêm vào LLM Gateway hoặc Sandbox Execution Environment.
