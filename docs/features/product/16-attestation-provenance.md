---
sources:
  - "server/internal/service/attestation.go"
  - "server/internal/repository/attestation.go"
  - "server/pkg/attest/dsse.go"
  - "server/pkg/attest/statement.go"
verified: 2026-07-30
---

# 16. Attestation & Code Provenance

**Status:** 🟢 Implemented  
**Owner docs:** `docs/ARCHITECTURE.md`  
**Code areas:** `server/internal/service/attestation.go`, `server/pkg/attest/`  
**Acceptance criteria:** Mọi dòng code do AI tạo ra đều có xuất xứ (provenance) rõ ràng, được ký cryptographic (DSSE) và có thể kiểm chứng được nguồn gốc.

**Mục tiêu:** Cung cấp cơ chế xác thực SLSA (Supply Chain Levels for Software Artifacts) cho code do AI sinh ra, đảm bảo tính minh bạch và bảo mật.

---

## A. Code Provenance (Nguồn Gốc Code)

Mỗi khi hệ thống tạo Pull Request, một bản ghi nguồn gốc (provenance) sẽ được tạo ra:
- **Coded By:** Xác định AI Agent (bao gồm Role, Model, Provider) nào đã viết code.
- **Reviewed By:** Xác định AI Agent nào đã thực hiện quá trình Cross-Harness Review.
- **Context Used:** Lưu vết các files ngữ cảnh và tool đã được cấp cho AI trong quá trình thực thi.

## B. Cryptographic Attestation (DSSE)

*   **DSSE Envelope:** Các tuyên bố về nguồn gốc code (`server/pkg/attest/statement.go`) được đóng gói theo chuẩn DSSE (Dead Simple Signing Envelope).
*   **Chữ Ký Số:** Hệ thống ký số các artifact/commit do AI tạo ra để chống giả mạo. Các khóa (keys) được quản lý bảo mật qua `keys.go`.
*   **Xác Minh (Verification):** CI/CD pipeline có thể sử dụng public key để xác minh rằng code thực sự do Auto Code OS tạo ra và không bị thay đổi trái phép.
