# Backlog: §4.3 Hệ Thống Agent — Tính năng Chưa Triển Khai

> **Ngày ghi nhận:** 2026-06-02
> **Nguồn phân tích:** Kiểm tra code trực tiếp tại session 2026-06-02T18:11 (+07:00); cập nhật lại 2026-06-02 sau khi rà soát backend hiện tại
> **Trạng thái:** ❌ Chưa triển khai — cần plan riêng

---

## 1. Skill Generation từ LearningSuggestion

### Mô tả
`DetectPatterns()` trong [learning.go](../../server/internal/orchestrator/learning.go) đã phát hiện pattern lặp lại từ memory và tạo ra một `LearningSuggestion` có `SuggestionType = "pattern"`. `POST /api/v1/suggestions/{suggestionID}/approve` và `LearningService.ApproveSuggestion()` đã tồn tại, nhưng **không có cơ chế nào** để convert suggestion đã được phê duyệt thành một `Skill` entry thực sự trong database.

### Hiện trạng code

```
LearningSuggestion (DB)
    └── status: "pending" | "approved" | "rejected" | "applied"
    └── suggestion_type: "pattern" | "rule" | "prompt_patch" | "skill"

Skill (DB)
    └── name, description, schema
    └── [KHÔNG CÓ liên kết nào từ LearningSuggestion → Skill]

LearningService.ApproveSuggestion()
    └── rule          → auto-create Rule và mark suggestion = "applied"
    └── prompt_patch  → log manual application required
    └── skill         → log manual registration required
    └── pattern       → log stored for reference
```

**File liên quan:**
- `server/internal/orchestrator/learning.go` — `DetectPatterns()`
- `server/internal/service/learning.go` — `ApproveSuggestion()` và `applySuggestion()`
- `server/pkg/models/phase6.go` — model `LearningSuggestion`, constants `SuggestionType*`, `SuggestionStatus*`
- `server/internal/repository/skill.go` — `Create()` skill
- `server/internal/service/skill.go` — `SkillService.Create()`

### Cần làm
- [ ] Inject `SkillRepo` hoặc `SkillService` vào `LearningService` để có thể tạo skill khi approve suggestion.
- [ ] Trong `LearningService.applySuggestion()`: nếu `suggestion_type == "skill"` hoặc `suggestion_type == "pattern"` đủ điều kiện → tự động tạo `Skill` từ nội dung suggestion.
- [ ] Mark suggestion thành `applied` và lưu metadata/feedback chứa `applied_skill_id`.
- [ ] Cân nhắc schema mặc định cho skill tự sinh: dùng `{}` hay yêu cầu suggestion metadata chứa JSON schema?
- [x] Handler/API endpoint `POST /api/v1/suggestions/{suggestionID}/approve` đã tồn tại.
- [x] `LearningService.ApproveSuggestion()` đã tồn tại và đã auto-apply rule suggestions.

---

## 2. Auto-join Agent khi Project mới được tạo

### Mô tả
Logic query trong DB (`ListByProjectID`, `FindAvailableForTask`) đã **hỗ trợ** `assignment_strategy = 'auto_join'` — agent có strategy này sẽ tự động khả dụng trong mọi project cùng org. Nhưng **không có hook/event** nào được gọi khi một Project mới được tạo để link agent vào `project_agents` table.

### Hiện trạng code

```
ProjectService.Create()
    └── repo.Create() → tạo project vào DB
    └── [KHÔNG GỌI] AgentRepo.AssignAutoJoinAgents() ← hàm này chưa tồn tại

AgentRepo.ListByProjectID()
    └── Query đã có: WHERE assignment_strategy = 'auto_join' OR EXISTS (project_agents)
    └── ✅ Auto-join agent vẫn hiện ra trong danh sách qua JOIN
    └── ✅ FindAvailableForTask() cũng đã có logic tương tự

→ Kết luận: Auto-join HOẠT ĐỘNG qua query JOIN nhưng
  không ghi record vào bảng project_agents
  → Có thể gây thiếu nhất quán nếu sau này cần audit trail
```

**File liên quan:**
- `server/internal/service/project.go` — `Create()` ← điểm inject hook
- `server/internal/repository/agent.go` — `AssignToProject()` ← đã có, cần gọi nếu chọn materialize assignment
- `server/internal/repository/agent.go` — `ListByOrgID()` ← list agents để filter `auto_join`

### Cần làm
- [ ] Trong `ProjectService.Create()`: sau khi tạo project thành công, gọi helper để lấy tất cả agent `auto_join` của org và `AssignToProject()` cho từng agent
- [ ] Hoặc: Chấp nhận approach hiện tại (JOIN-only, không ghi `project_agents`) và document rõ đây là thiết kế có chủ đích — không cần sửa

> [!NOTE]
> Approach JOIN-only hiện tại **vẫn hoạt động đúng** về mặt chức năng. Việc ghi thêm vào `project_agents` chỉ cần thiết nếu muốn có audit trail hoặc cho phép admin xem danh sách agent nào đang được gắn vào project cụ thể.

---

## Tóm tắt ưu tiên

| Feature | Mức độ ảnh hưởng | Độ phức tạp | Ưu tiên |
|:---|:---:|:---:|:---:|
| Skill Generation từ Suggestion | Cao — tính năng Self-Improve chưa khép vòng | Medium | 🔴 High |
| Auto-join hook khi tạo Project | Thấp — query hiện tại đã hoạt động đúng | Low | 🟡 Medium |
