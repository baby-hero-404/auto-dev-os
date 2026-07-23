---
sources:
  - "server/**"
  - "web/src/components/projects/LearnedSkillsPanel.tsx"
verified: 2026-07-23
---

# 03. Skill System (Git-Sync Architecture)

**Status:** 🟢 Implemented  
**Owner docs:** `docs/features/product/03-skill-system.md` (this file)  
**Code areas:** `server/pkg/models`, `server/internal/service/skill.go`, `server/internal/prompts`, `web/src/app/skills`

## 1. Tổng Quan & Tiêu Chí Nghiệm Thu (Acceptance Criteria)
Thay vì hỗ trợ các thao tác CRUD tùy biến trực tiếp trên UI hoặc cơ sở dữ liệu (điều này gây phức tạp và dễ xung đột phiên bản), hệ thống Skill chuyển sang sử dụng mô hình **Git-as-Source-of-Truth**. Toàn bộ tri thức (skills) của AI được quản lý bên ngoài qua Git repository và đồng bộ (sync) vào Auto Code OS.

**Tiêu chí nghiệm thu:**
- Giao diện (UI) tuân thủ nguyên tắc **Read-Only** đối với nội dung Skill. Không có các chức năng Tạo/Sửa/Xóa skill đơn lẻ.
- Quản lý tập trung qua việc cấu hình URL của Git Repository và thực hiện đồng bộ (Clone/Pull).
- Các Repository bắt buộc phải có file cấu hình `registry.json` hoặc `registry.min.json` tại thư mục gốc.
- Hỗ trợ giao diện xem cấu trúc thư mục (Folder Tree) và đọc trực tiếp nội dung các file bên trong Skill repository.
- Hệ thống hỗ trợ tích hợp và hợp nhất (merge) liền mạch giữa **Global Skills** và **Local Skills**.

---

## 2. Kiến Trúc & Luồng Hoạt Động (Architecture & Workflow)

### 2.1. Phân Loại Skills (Global vs Project Local)
Hệ thống cung cấp cơ chế phân tầng kỹ năng để linh hoạt cho các ngữ cảnh sử dụng:

1. **Global Skills (Git-Synced):** 
   - Quản lý tập trung tại trang Admin.
   - Được đồng bộ trực tiếp từ Git Repositories và lưu trữ tại `{SkillsRoot}/git/<repo_name>/`.
   - Khả dụng cho mọi Agent trên toàn hệ thống.
2. **Local Skills (Project-Specific):** 
   - Được định nghĩa riêng rẽ cho từng dự án cụ thể (ví dụ lưu tại thư mục `[ProjectRoot]/skills/`).
   - Bị ẩn khỏi giao diện quản lý Global chung.
   - Orchestrator sẽ tự động quét và hợp nhất (merge) các Local Skills này vào danh sách công cụ của AI chỉ khi thực thi các tác vụ thuộc dự án đó (JIT Knowledge cục bộ).

### 2.2. Luồng Đồng Bộ (Git Sync Workflow)
1. **Đăng ký Nguồn (Source Registration):** Admin khai báo URL của Git repository (ví dụ: `https://github.com/org/prompt_base.git`).
2. **Đồng bộ hóa (Synchronization):**
   - *Lần đầu:* Backend chạy lệnh `git clone` để tải dữ liệu về `{SkillsRoot}/git/{repo_name}`.
   - *Các lần tiếp theo:* Khi user ấn "Sync", Backend chạy lệnh `git pull` để lấy dữ liệu mới.
3. **Phân Tích Cấu Trúc (Manifest Parsing):** Backend tự động đọc file `registry.json` để lập chỉ mục (index) các skills hợp lệ.
4. **Khám Phá (Read-Only Exploration):** Người dùng có thể duyệt và đọc nội dung các file `SKILL.md` hoặc bash scripts của hệ thống thông qua giao diện File Viewer.

---

## 3. Quy Chuẩn Kỹ Thuật (Data Contracts & Standards)

### 3.1. Cấu Trúc Thư Mục Repository
Để hệ thống nhận diện đúng, cấu trúc file trong Git Repository cần tuân theo quy chuẩn sau:

```text
[git-repo-root]/    <-- Sẽ được clone vào: {SkillsRoot}/git/<repo_name>/
├── registry.json   <-- (hoặc registry.min.json) File cấu hình bắt buộc
└── skills/
    ├── core/
    │   └── architecture/
    │       ├── SKILL.md
    │       └── details.txt
    └── tech/
        └── database-design/
            └── SKILL.md
```

### 3.2. Định Dạng `registry.json`
```json
{
  "skills": {
    "core": [
      {
        "id": "architecture",
        "name": "Architecture Planner",
        "description": "Analyze system design, evaluate trade-offs, and design architectures.",
        "path": "skills/core/architecture/SKILL.md"
      }
    ]
  }
}
```

---

## 4. Yêu Cầu Triển Khai (Engineering Requirements)

### 4.1. Backend Service (`server/internal/service/skill.go`)
- **Loại bỏ:** Các endpoint thực hiện `Create`, `Update`, `Delete` đối với từng skill đơn lẻ.
- **Giữ lại & Bổ sung:** 
  - API quản lý nguồn Git: `ListSources`, `AddSource`, `DeleteSource`, `SyncSource` (thực thi clone/pull).
  - API duyệt thư mục (Tree Explorer): `GET /api/skills/sources/{sourceID}/files?path=...` (trả về danh sách thư mục con và files).
  - API xem nội dung (Content Viewer): `GET /api/skills/sources/{sourceID}/file-content?path=...` (trả về raw text của file).

### 4.2. Frontend Web UI (`web/src/app/skills/page.tsx`)
- **Quản Lý Nguồn (Source Management):** Hiển thị Form nhập Git URL, danh sách các nguồn đang có, và nút **Sync / Refresh**.
- **Skill Explorer:**
  - Hiển thị danh sách tổng hợp tất cả các skills từ `registry.json`.
  - **Giao diện Split View khi chọn 1 Skill:** 
    - *Left Panel:* Hiển thị cấu trúc cây (Folder Tree) của Skill đó.
    - *Right Panel:* Trình xem code/markdown có hỗ trợ highlight syntax cho file đang chọn.
- **Ràng Buộc Chặt Chẽ:** Đảm bảo không tồn tại bất kỳ Component nào cho phép sửa đổi dữ liệu (Read-only UI).

### 4.3. Project Local Skills UI (`web/src/components/projects/LearnedSkillsPanel.tsx`)
- Bảng quản lý Project-Scoped Learned Skills trong phần Project Settings (ẩn khỏi Global `/skills`).
- Hỗ trợ filter theo status, xem thông số sử dụng (`usage_count`, `success_count`) và thực hiện các thao tác: approve, disable, delete (có dialog confirm).
