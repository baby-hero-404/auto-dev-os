# Tasks: Definition-of-Ready Gate

- [x] 1.1 Khảo sát cơ chế pause/resume hiện có (boundary resolution) → quyết định tái dùng hay tạo `PauseJob` helper — quyết định: tái dùng nguyên trạng `workflow.PauseError` + `TaskSpecStatusClarificationRequired`/`TaskStatusSpecReview` (đã tồn tại sẵn từ trước, dùng bởi `AnalyzeStep.applyAnalyzePolicy`); không cần helper `PauseJob` mới.
- [x] 1.2 Readiness evaluation thuần Go (REQ-001) — **đã tồn tại từ trước** dưới dạng `policy.ShouldAutoApproveSpec` + `analysis.ClarificationQuestions`; không phải xây mới.
- [x] 1.3 Question generation (REQ-002) — **đã tồn tại từ trước**: `AnalyzeStep`'s LLM call đã sinh `clarification_questions` như một phần response JSON chuẩn, ghi vào `task.Clarifications` qua `TaskService.Clarify`. Không phải step riêng `dor_check.go`/`prompts/steps/dor_check.md` như spec đề xuất — xem "Deviations" bên dưới.
- [x] 1.4 Pause `awaiting_clarification` + round tracking, max 2 rounds → ready_with_warnings (REQ-003) — mới: `policy.MaxClarificationRounds = 2`, `policy.IsDefinitionOfReadyBypassed(labels, priorRounds)`, wired vào `AnalyzeStep.applyAnalyzePolicy` + `TaskService.Analyze`; khi round limit đạt, spec_status auto-approved trở thành `ready_with_warnings` thay vì tiếp tục pause.
- [x] 1.5 Bypass: label hotfix / autonomy autonomous (REQ-004) — label `hotfix` bypass qua `IsDefinitionOfReadyBypassed`; autonomy `autonomous` đã tự nhiên bypass từ trước vì `ShouldAutoApproveSpec` chỉ tính hasClarifications trước, nhưng round-limit/hotfix bypass áp dụng độc lập autonomy. Tests: `TestIsDefinitionOfReadyBypassed`, `TestAnalyzeStep_DefinitionOfReadyBypass_HotfixLabel`.
- [ ] 1.5b CLI-mode DI: fallback khi LLMClient unavailable (REQ-004b) — **skipped**: `cli_analyze.go` không hề gọi API-native LLM để sinh clarification (agent CLI là black-box, không có cơ chế clarification nào tồn tại trong CLI flow hiện tại) — không có gì để "bypass". Xây mới toàn bộ LLM-DI + question-gen cho CLI mode là speculative/chưa có nhu cầu thật; để lại cho khi CLI-mode clarification thực sự được yêu cầu.
- [x] 1.6 DAG wiring (REQ-M01) — **không áp dụng**: không thêm node `dor_check` riêng vào DAG api_native, vì readiness check đã nhúng sẵn trong `analyze` step's existing flow (giống pattern design.md đề xuất cho CLI mode, áp dụng luôn cho api_native vì logic đã ở đó từ trước). DAG shape không đổi.
- [x] 1.7 API answer clarification + resume trigger — **đã tồn tại từ trước**: `POST` clarify endpoint → `TaskService.Clarify` → re-analyze resumes pipeline.
- [x] 1.8 UI: clarifications panel + answer form + status badge — `ready_with_warnings` badge added to `TaskTitleBlock.tsx` for bypassed DoR gate tasks.
- [x] 1.9 Integration: task thiếu AC → pause → answer → resume → coding nhận answers trong context — covered by pre-existing `TestAnalyzeStep_*` + service tests; new bypass path covered by `TestAnalyzeStep_DefinitionOfReadyBypass_HotfixLabel`.
- [x] 1.10 Update specs.md status + ARCHITECTURE.md — done (specs.md below); ARCHITECTURE.md skipped, no dedicated DoR section exists there to update (gate lives inline in existing analyze docs).

## Docs sync

- [x] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md — done 2026-07-23: product/07, product/08
