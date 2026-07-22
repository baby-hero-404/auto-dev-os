# Specs: Definition-of-Ready Gate

## Added Requirements

### REQ-001: Gate pass khi task ready
> ✅ Status: Done (đã có sẵn qua `policy.ShouldAutoApproveSpec`)

**Scenario:**
- WHEN task có acceptance criteria, file scope từ analysis, và 0 clarification open
- THEN `dor_check` pass không gọi LLM, thêm <1s vào pipeline
- AND `task.SpecStatus` → `ready`

### REQ-002: Gate chặn khi thiếu thông tin
> ✅ Status: Done (đã có sẵn qua `AnalyzeStep`'s clarification_questions + `TaskSpecStatusClarificationRequired` pause)

**Scenario:**
- WHEN task thiếu acceptance criteria hoặc còn clarification open
- THEN 1 LLM call sinh tối đa 5 câu hỏi cụ thể, append vào `task.Clarifications` với `status=open`
- AND job pause trạng thái `awaiting_clarification`, không step nào sau đó chạy

### REQ-003: Resume sau khi trả lời
> ✅ Status: Done — round limit mới: `policy.MaxClarificationRounds`/`IsDefinitionOfReadyBypassed`

**Scenario:**
- WHEN mọi clarification chuyển `answered`
- THEN job resume, `dor_check` re-validate (câu trả lời được đưa vào context các step sau)
- AND nếu vẫn thiếu → sinh câu hỏi vòng 2, tối đa 2 vòng rồi pass với warning (tránh loop vô hạn)

### REQ-004: Bypass
> ✅ Status: Done — `policy.IsDefinitionOfReadyBypassed` (hotfix label)

**Scenario:**
- WHEN task có label `hotfix` hoặc project autonomy = `autonomous`
- THEN gate log warning liệt kê tiêu chí thiếu nhưng vẫn pass

### REQ-004b: Fallback khi không có API-native LLM (CLI mode)
> ❌ Status: Not Started — skipped, xem tasks.md 1.5b (CLI flow hiện không có cơ chế clarification nào để cần fallback)

**Scenario:**
- WHEN task engine=cli, readiness thiếu tiêu chí, và server KHÔNG có API-native LLM khả dụng (không key)
- THEN question-generation bị bypass với warning log, `SpecStatus=ready_with_warnings`, pipeline tiếp tục — không crash, không chặn
- AND WHEN LLM khả dụng THEN hành vi sinh câu hỏi như REQ-002

### REQ-005: UI clarifications
> ✅ Status: Done (đã có sẵn từ trước — clarifications answer flow); `ready_with_warnings` badge riêng chưa thêm, xem tasks.md 1.8

**Scenario:**
- WHEN task ở `awaiting_clarification`
- THEN task detail hiển thị câu hỏi + form trả lời; submit → answered → job resume tự động

## Modified Requirements

### REQ-M01: DAG shape
> ✅ Status: Done — không cần thay đổi DAG, xem tasks.md 1.6

**Scenario:**
- WHEN BuildWorkflow cho api_native flow
- THEN `dor_check` nằm sau `analyze`, mọi node từng dependsOn `analyze` giờ dependsOn `dor_check`

## Removed Requirements
- Không có.
