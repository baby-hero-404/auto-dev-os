# Specs: Service/Repository/Models Dead Code Cleanup

## Removed Requirements

### REQ-R01: Dead agent-claim methods bị xoá
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN `go build ./...` sau khi xoá `FindAvailableByRole`/`FindByRole`/`FindAnyAvailable`
- THEN build thành công; `ClaimAvailableByRole`/`ClaimAnyAvailable` (đang dùng thật ở `agent_manager.go`) không đổi hành vi

### REQ-R02: `LearningService.skills` plumbing bị xoá
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN `applySkillSuggestion` được gọi (bất kỳ input nào)
- THEN vẫn trả đúng lỗi cố định "skill creation is no longer supported on the UI; please commit the skill to your Git repository registry instead" — hành vi observable không đổi, chỉ xoá field/wiring không còn được đọc

### REQ-R03: `WorkflowStepSandbox` bị xoá
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN `go build ./...` sau khi xoá constant
- THEN build thành công, không step registry nào tham chiếu literal `"sandbox"` bị ảnh hưởng

## Modified Requirements

### REQ-M01: Error-wrapping thống nhất giữa các repository
> ✅ Status: Fully Implemented

**Scenario:**
- WHEN 1 repo method trả "not found" (bất kể trước đây dùng `mapError` hay raw gorm error)
- THEN mọi service/handler kiểm tra not-found bằng đúng 1 cách nhất quán (`errors.Is(err, repository.ErrNotFound)` sau chuẩn hoá, thay vì phải check cả `gorm.ErrRecordNotFound` lẫn `repository.ErrNotFound` tuỳ file)

## Resolved (đã quyết định trong lúc implement)
- **Knowledge-edge write path**: user quyết định **giữ nguyên**, không xoá — groundwork cho authoring UI tương lai.
- **Audit `IPAddress` field**: hoá ra là bug UI-visible thật (`web/src/app/audit/page.tsx` đã hiển thị cột này) — đã **fix** (wiring `WithIPAddress(r.RemoteAddr)` vào 3 call site `RecordAction` trong `pr.go`), không phải dead code cần hỏi giữ/xoá.
- **`AgentSkill` struct**: xác nhận mồ côi hoàn toàn (0 reference Go/FE, không AutoMigrate) — đã **xoá struct**, bảng DB giữ nguyên.
- **`PRStatusApproved`/`Rejected`/`Merged`**: xác nhận 0 reference — đã **xoá**, giữ `PRStatusOpen`.
- **`ResetAllRunningJobs`**: xác nhận không có cơ chế thay thế ở startup — đây là **gap thật, không phải dead code** — giữ nguyên, không xoá, không tự ý wiring (ngoài phạm vi OpenSpec dọn dead code, cần review riêng cho thay đổi reliability behavior).
