---
sources:
  - "server/internal/governance/"
  - "server/internal/policy/"
  - "server/internal/service/audit.go"
verified: 2026-07-30
---

# 06. Governance, Policy & Audit

**Status:** 🟢 Implemented  
**Owner docs:** `docs/ARCHITECTURE.md`  
**Code areas:** `server/internal/governance/`, `server/internal/policy/`, `server/internal/service/audit.go`

**Mục tiêu:** Cung cấp hạ tầng kiểm soát rủi ro, thực thi các quy tắc (policy) tĩnh/động cho quy trình CI/CD của AI, và lưu trữ nhật ký kiểm toán (audit log) cho toàn bộ thao tác hệ thống.

---

## 1. Declarative Pipeline Governance (Data-Driven DAG)

Module `server/internal/governance/` chịu trách nhiệm validate và quản lý cấu hình pipeline.
- **Structural Validation (`dag.go`, `validate.go`):** Hệ thống thực thi kiểm tra tính toàn vẹn của đồ thị luồng công việc (acyclic, deps resolve, single entry, no dead ends) trước khi lưu.
- **Config Presets (`presets.go`):** Cung cấp các pipeline template tiêu chuẩn có thể nạp động.

## 2. Policy Enforcement

Module `server/internal/policy/` định nghĩa các quy tắc kiểm soát hệ thống:
- **Review Policy (`review_policy.go`):** Quyết định chiến lược review chéo (ví dụ: Agent Reviewer phải dùng Model khác với Agent Code).
- **Scheduler Policy (`scheduler_policy.go`):** Quy định giới hạn thực thi (timeouts, retries, concurrency bounds).

## 3. System Audit Logging

Module `audit.go` cung cấp cơ chế ghi log bất biến:
- Ghi nhận mọi quyết định của AI, sự can thiệp của con người (Approval/Rejection), và các thay đổi cấu hình dự án.
- Audit logs không thể bị thay đổi sau khi ghi, phục vụ cho quá trình truy xuất trách nhiệm (compliance).
