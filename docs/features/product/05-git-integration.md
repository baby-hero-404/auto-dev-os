---
sources:
  - "server/**"
  - "server/pkg/attest/**"
verified: 2026-07-23
---

# 05. Git Integration

**Status:** 🟢 Implemented  
**Owner docs:** `docs/ARCHITECTURE.md`  
**Code areas:** `server/internal/gitops`, `server/internal/workflow`, `server/internal/orchestrator`, `web/`  
**Blocking decisions:** GitLab/Bitbucket/Gitea priority order.  
**Acceptance criteria:** User can attach a Git account to a project, agent can clone, commit, push, create PR, and persist PR URL on the task.

**Mục tiêu:** AI tự động thực thi toàn bộ vòng đời Git — từ clone repo đến tạo Pull Request — sử dụng tài khoản Git do người dùng cấu hình. Credential được quản lý an toàn và tách biệt giữa các project.

---

## Luồng Vận Hành

```
1. Người dùng thêm GitHub Account
   └── Nhập token, chọn provider (GitHub hiện tại; GitLab planned), tùy chọn base_url (cho Enterprise)

2. Người dùng tạo Project
   ├── Nhập Repo URL (ví dụ: https://github.com/org/my-repo.git)
   └── Chọn Git Account → liên kết với Repository

3. AI Agent nhận Task → Orchestrator bắt đầu workflow

4. Clone Repo vào workspace cách ly
   └── Resolve credential tự động (xem bảng bên dưới)

5. Agent viết code → Commit
   ├── Git identity tự động: "AutoCodeOS [backend-specialist]"
   ├── Stage tất cả thay đổi
   └── Bỏ qua nếu không có gì thay đổi

6. Push lên branch mới
   • Single agent:  feature/{task_id}
   • Parallel agents (§08 ownership):
     - Backend: feature/{task_id}-be
     - Frontend: feature/{task_id}-fe
     - Integration: feature/{task_id} (merge BE + FE vào đây trước PR)

7. Tạo Pull Request
   ├── Tiêu đề: "AutoCodeOS: {task.title}"
   ├── Mỗi commit trong PR được ký DSSE (Ed25519) và lưu thành attestation record
   │   (coded_by/reviewed_by, prompt_hash, policy_snapshot) — xem §09 "Attestation Audit Panel"
   ├── PR URL được lưu → Task chuyển sang pr_ready (chưa hoàn thành)
   └── Task chỉ hoàn thành sau khi human approve + merge (§08)
```

## Credential Resolution (3 lớp ưu tiên)

Hệ thống tự động tìm credential theo thứ tự, dùng nguồn đầu tiên có sẵn:

| Ưu tiên | Nguồn | Khi nào dùng |
|:--------|:------|:-------------|
| 1 | Token gắn trực tiếp trên Repository | Repo có token riêng (override thủ công) |
| 2 | Git Account được liên kết qua Repository | Repository đã chọn Git Account cụ thể |
| 3 | Git Account cấp Organization khớp provider | Fallback khi không có link trực tiếp |

## GitHub Enterprise / Self-hosted

*   `GitAccount.base_url` để tùy chỉnh API endpoint (vd: `https://github.company.com/api/v3`).
*   Mặc định là `https://api.github.com` khi để trống.

## Mở Rộng Planned

| Provider | Status |
|:---------|:-------|
| GitHub | ✅ Implemented |
| GitLab | Planned |
| Bitbucket | Planned |
| Gitea (self-hosted) | Planned |

---

**Dự án tham khảo:**

| Dự án | Lý do tham khảo |
|:------|:----------------|
| GitHub App Docs | Mẫu tích hợp Git API |
| Gitea | Kiến trúc Git self-hosted |
| GitLab CE | Quy trình CI/CD và Merge Request |
