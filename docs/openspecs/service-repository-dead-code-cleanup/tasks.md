# Tasks: Service/Repository/Models Dead Code Cleanup

## P1 — xoá an toàn, không cần xác nhận thêm

### Task 1.1: Xoá `FindAvailableByRole`/`FindByRole`/`FindAnyAvailable`
- [x] Re-verify 0 caller.
- [x] Xoá cả 3 method khỏi `agent.go`.
- [x] `go build ./...`, `go test ./internal/repository/...`.
- Satisfies: REQ-R01

### Task 1.2: `ResetAllRunningJobs` — điều tra, kết luận: giữ, không xoá
- [x] Kiểm tra `cmd/api/main.go`: không có cơ chế reset job "running" nào chạy lúc khởi động server (`orch.StartWorker` được gọi thẳng, không có `ResetAllRunningJobs`/tương đương phía trước). `ResetStuckJobs` (đang dùng thật ở `queue.go:73`) chỉ chạy định kỳ theo `ticker`, có ngưỡng stale 10 phút — không phải cơ chế thay thế tương đương cho crash-recovery-lúc-boot.
- [x] Kết luận: đây là **gap thật** (tính năng "reset job còn 'running' khi server restart" có vẻ được thiết kế qua `ResetAllRunningJobs` nhưng chưa bao giờ được wiring vào `main.go`), không phải dead code — **không xoá**. Không tự ý wiring vào startup trong lúc này vì đó là thay đổi hành vi reliability cần review riêng, ngoài phạm vi 1 OpenSpec "dọn dead code". Để lại như 1 follow-up riêng.
- Satisfies: (kết luận: giữ nguyên, không có REQ xoá)

### Task 1.3: Xoá `LearningService.skills`/`SetSkillService`
- [x] Xoá field + setter trong `learning.go`.
- [x] Bỏ dòng `learningSvc.SetSkillService(skillSvc)` trong `cmd/api/main.go`.
- [x] `go build ./...`, `go test ./internal/service/...`.
- Satisfies: REQ-R02

### Task 1.4: Xoá `WorkflowStepSandbox`
- [x] Re-verify 0 reference.
- [x] Xoá khỏi `workflow.go`.
- Satisfies: REQ-R03

## P1 — cần xác nhận trước khi code

### Task 2.1: Knowledge-edge write path
- [x] Hỏi user: giữ hay xoá `CreateEdge`/repo `Create`/`Delete`?
- [x] **Quyết định: giữ nguyên, không đụng vào** — groundwork cho authoring UI tương lai. Không code gì thêm.
- Satisfies: (quyết định — giữ nguyên)

### Task 3.1: Audit `IPAddress` — nâng cấp thành bug thật, đã fix
- [x] Điều tra thêm trước khi hỏi: phát hiện `web/src/app/audit/page.tsx` **đã hiển thị** cột `log.ip_address` trong UI — cột này luôn trống/"local" vì backend chưa bao giờ ghi field này. Đây không còn là "dead code cần hỏi giữ/xoá" mà là **bug UI-visible thật**, xử lý trực tiếp thay vì hỏi.
- [x] Thêm `service.WithIPAddress(r.RemoteAddr)` vào cả 3 chỗ gọi `RecordAction` hiện có (`internal/handler/pr.go`: `Approve`/`Reject`/`StartReview`) — theo đúng convention `r.RemoteAddr` thô đã dùng ở `ratelimit.go` (không thêm X-Forwarded-For parsing, giữ tối giản/nhất quán).
- [x] Test: `TestPRHandler_Reject_TriggersRepair` mở rộng `mockAuditSvc` để áp dụng `AuditOption`, assert `IPAddress` không rỗng.
- Satisfies: (bug fix, không thuộc REQ dead-code — nhưng đã đóng gap UI thật)

### Task 4.1: `AgentSkill` — xoá struct Go, giữ nguyên bảng
- [x] Verify: 0 reference Go (không repo, không service, không FE) — kể cả trong `AutoMigrate` (project dùng SQL migration thủ công, không auto-migrate theo struct, nên xoá struct không ảnh hưởng bảng `agent_skills` đã tồn tại).
- [x] Xoá struct `AgentSkill` khỏi `pkg/models/agent.go`. Bảng DB giữ nguyên — drop bảng (nếu cần) là quyết định migration riêng, ngoài phạm vi OpenSpec này.
- Satisfies: (dọn dẹp — struct mồ côi, không có REQ riêng)

### Task 5.1: `PRStatus{Approved,Rejected,Merged}` — đã xoá
- [x] Verify 0 reference Go/FE.
- [x] Xoá 3 constant, giữ `PRStatusOpen` (đang dùng thật ở `pr_generator.go:244`). Comment lại giải thích PR lifecycle thật nằm ở `Task.Status`.
- Satisfies: (dọn dẹp — constant chết, không có REQ riêng)

## P2 — chuẩn hoá error-wrapping (làm cuối, phạm vi thu hẹp sau điều tra)

### Task 6.1: Áp `mapError` cho các repo có bug thật + 1 chỗ consistency
- [x] Điều tra trước khi làm rộng: chỉ `attestation.go` (`GetByCommitHash`, `GetActive`, `GetByKeyID`) có bug thật — trả raw gorm error khiến `internal/service/attestation.go:38` và `internal/handler/attestation.go:29` phải tự check `gorm.ErrRecordNotFound` trực tiếp, vòng qua `statusForError`'s generic handling. `learned_skill.go`'s `GetByID` cũng raw nhưng caller (`handler/learned_skill.go`) đã đi qua `writeServiceError`/`statusForError` (đã tự check cả 3 kiểu lỗi) nên an toàn để wrap cho nhất quán, không cần sửa consumer.
- [x] `audit.go`/`knowledge_edge.go` không có single-record `GetByID`/`First` lookup nào cần not-found semantics (chỉ Create/List/Delete) — không cần đụng.
- [x] `secrets.go` bị bỏ qua: sau Task 1.3-tương-đương ở `handler-api-cleanup` OpenSpec, `SecretRepo`/`SecretService` không còn được construct ở đâu cả (`cmd/api/main.go` đã bỏ dòng khởi tạo) — dead code, không đáng chuẩn hoá lỗi cho code không chạy.
- [x] `analytics*.go` — kiểm tra không có `First`/`GetByID` single-record nào (chỉ aggregate queries), không cần đụng.
- [x] Áp `mapError` cho `attestation.go` (3 hàm) + `learned_skill.go` (`GetByID`).
- [x] Cập nhật `internal/service/attestation.go` (`EnsureActiveKey`) và `internal/handler/attestation.go` (`GetByCommit`) dùng `repository.ErrNotFound` thay vì `gorm.ErrRecordNotFound`, xoá `gorm` import không còn cần ở 2 file này.
- [x] Test mới: `TestAttestationService_EnsureActiveKey_GeneratesWhenNoneExist`, `TestAttestationService_VerifyByCommitHash_NotFound` — khoá lại đúng hành vi vừa sửa (trước đây "tình cờ đúng" vì check raw gorm error khớp; giờ đúng vì check tường minh `repository.ErrNotFound`).
- [x] `go test ./...` toàn bộ pass.
- Satisfies: REQ-M01 (phạm vi thu hẹp còn đúng phần có bug thật + 1 chỗ nhất quán, không rebọc toàn bộ 10 file như dự kiến ban đầu — phần còn lại hoặc không cần, hoặc dead code không đáng sửa)

## Self-review checklist

| REQ | Task |
|---|---|
| REQ-R01 | 1.1 |
| REQ-R02 | 1.3 |
| REQ-R03 | 1.4 |
| REQ-M01 | 6.1 |
