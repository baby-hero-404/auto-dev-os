# Proposal: Cross-Harness Review (P3.1)

## Why

Model tự review code chính nó viết có blind spots hệ thống (cùng prior, cùng lỗi suy luận). ai-sdlc (reference) enforce "khác harness mới được review" bằng DSSE — Auto Code OS có thể áp mềm hơn: **provider/model review khác với provider/model đã code**. `pkg/llm` đã multi-provider (verified) nên phần lớn là wiring + policy. Với CLI mode (Wave 1), cross-review còn quan trọng hơn: CLI là black-box, review bằng API-native model là lớp kiểm soát duy nhất server có.

Prerequisite: Wave 1 xong (để có engine metadata trên task) và `review-verdict-split` xong (review output đã structured).

## What Changes

### Issue 1: Review harness selection

- Config per-project: `review_harness_policy`: `same` (như cũ, default ban đầu) | `different_model` | `different_provider`.
- Khi review step chạy: resolver chọn model/provider cho review dựa trên policy + metadata "ai đã code" (model level của code step; với CLI mode: luôn coi là "khác" vì code bởi CLI).
- Fallback: nếu không có provider thứ 2 được cấu hình → log warning, dùng model khác cùng provider; nếu cũng không được → chạy same + warning (không chặn pipeline).

### Issue 2: Harness metadata trên task

- Ghi vào step state + task record: `coded_by` (engine + provider + model), `reviewed_by`. Hiển thị ở task detail + đưa vào PR description (nền cho attestation P4.3).

### Issue 3: CLI-mode review step

- Thêm optional step `cross_review` vào `cli_spec_first` flow (giữa cli_implement và cli_mr, bật qua policy): API-native model review git diff của CLI + spec set, dùng 2-verdict schema của `review-verdict-split`; fail → re-dispatch cli_implement với violations (max cycles theo project).

## Capabilities

### New Capabilities
- Policy chọn review harness khác code harness; review step cho CLI flow.
- `coded_by`/`reviewed_by` metadata.

### Modified Capabilities
- Review step nhận model override; `cli_spec_first` definition thêm node optional.

### Removed Capabilities
- Không có.

## Impact

| Area | Files Affected |
|------|----------------|
| Models | `project.go` (`review_harness_policy`), task/step state metadata |
| Steps | `steps/review.go` (model override), `cli_spec_first` definition + `cross_review` step |
| LLM | resolver chọn provider/model trong `pkg/llm` hoặc caller |
| Web | project setting + task detail metadata display |
