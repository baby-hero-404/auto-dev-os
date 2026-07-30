# Design: Service/Repository/Models Dead Code Cleanup

## Approach

Giống [`orchestrator-dead-code-cleanup/`](../orchestrator-dead-code-cleanup/): re-verify bằng grep ngay trước khi xoá, xoá, `go build`+`go vet`+`go test ./...` toàn bộ sau mỗi item. Khác biệt: nhiều item ở đây có phần "cần xác nhận" trước khi xoá — không tự quyết định, liệt kê rõ câu hỏi trong tasks.md.

## Issue 1: `agent.go`/`workflow.go` dead methods

```bash
grep -rn "\.FindAvailableByRole(\|\.FindByRole(\|\.FindAnyAvailable(" server/
grep -rn "\.ResetAllRunningJobs(" server/
```
`FindAvailableByRole`/`FindByRole`/`FindAnyAvailable`: xoá thẳng, không cần xác nhận — đã có bản atomic thay thế rõ ràng, không phải "tính năng bị rút" mà là "đã nâng cấp cách implement".

`ResetAllRunningJobs`: **cần xác nhận trước** — khác với 3 method trên, đây có thể là 1 bug (thiếu 1 lời gọi startup-reset đáng lẽ phải có, không phải code thật sự thừa). Kiểm tra: `cmd/api/main.go` có đoạn nào "reset tất cả job đang chạy khi server khởi động" (để tránh job kẹt "running" mãi mãi sau crash) không — nếu không có bất kỳ cơ chế nào làm việc này, đây là **gap cần điền**, không phải dead code cần xoá. Nếu có (qua `ResetStuckJobs` hoặc cơ chế khác đã cover), xoá an toàn.

## Issue 2-3: Knowledge-edge, Audit IP — cần xác nhận trước khi code

Không thiết kế cụ thể cho tới khi có câu trả lời (giữ hay xoá) — xem tasks.md Task 2.1/3.1 là quyết định gate, không phải task code.

## Issue 4: `LearningService.applySkillSuggestion`

```go
// internal/service/learning.go — trước
type LearningService struct {
	...
	skills *SkillService
}
func (s *LearningService) SetSkillService(sk *SkillService) { s.skills = sk }
```
Xoá field + setter. `cmd/api/main.go:193` bỏ dòng `learningSvc.SetSkillService(skillSvc)`. `applySkillSuggestion`'s body (trả lỗi cố định) không đổi — chỉ xoá phần state không còn đọc.

## Issue 5: Dead constants/structs

`WorkflowStepSandbox`: xoá thẳng, thấp rủi ro nhất trong cả OpenSpec.
`AgentSkill`, `PRStatus{Approved,Rejected,Merged}`: **không code gì** cho tới khi có xác nhận — đây là các mục "hỏi trước, không tự quyết" (tasks.md).

## Issue 6: Chuẩn hoá error-wrapping

Hướng đề xuất: **dùng `mapError` cho toàn bộ** (thay vì xoá `mapError` khỏi 12 file đã dùng) — vì `mapError` đã là convention đa số, và các handler (`response.go`) vốn đã kỳ vọng `repository.ErrNotFound`/`repository.ErrConflict` là dạng lỗi chuẩn từ tầng repo. Áp dụng cho `analytics*.go`, `attestation.go`, `audit.go`, `knowledge_edge.go`, `learned_skill.go`, `secrets.go` — bọc các lời gọi gorm bằng `mapError` giống 12 file kia, không đổi logic query.

Sau khi chuẩn hoá: rà lại `internal/service/attestation.go:38`, `internal/handler/attestation.go:29` (2 chỗ đang check `gorm.ErrRecordNotFound` trực tiếp để "né" inconsistency) — đổi sang check `repository.ErrNotFound` cho nhất quán, và cân nhắc `handler/response.go` có cần giữ check `gorm.ErrRecordNotFound` nữa không (giữ lại 1 bản an toàn/defensive cũng được, không bắt buộc xoá).

## Trade-offs

- **Không xoá bảng `agent_skills` trong OpenSpec này** dù `AgentSkill` struct chết — drop bảng là thay đổi schema cần migration + xác nhận riêng, tách khỏi cleanup code thuần.
- **Chuẩn hoá error-wrapping (Issue 6) là refactor rộng nhất trong OpenSpec** (chạm ~6 file repo) — tách thành task riêng cuối cùng để review dễ tách khỏi phần xoá code đơn giản hơn.
