# Specs: Review Verdict Split

## Added Requirements

### REQ-001: Structured 2-verdict output
> ❌ Status: Not Started

**Scenario:**
- WHEN review step chạy
- THEN output parse được thành `{spec_compliance, code_quality}` mỗi cái có verdict + danh sách lý do
- AND parse fail → fallback hành vi single-verdict cũ + log warning (không chặn pipeline)

### REQ-002: Spec-fail routing
> ❌ Status: Not Started

**Scenario:**
- WHEN `spec_compliance=fail`
- THEN fix step nhận violations + trích yêu cầu gốc làm instruction ưu tiên, quality issues xếp sau

### REQ-003: Spec-fail escalation
> ❌ Status: Not Started

**Scenario:**
- WHEN cùng một violation (so khớp fuzzy theo nội dung) xuất hiện ở 2 review cycle liên tiếp
- THEN job pause `awaiting_review_escalation` cho user quyết (tiếp tục / hủy / sửa yêu cầu) thay vì chạy tiếp fix cycle

### REQ-004: Quality-only fail
> ❌ Status: Not Started

**Scenario:**
- WHEN spec pass, quality fail
- THEN fix cycle như hiện tại, đếm vào `MaxReviewFixCycles` như cũ

### REQ-005: UI hiển thị 2 verdict
> ❌ Status: Not Started

**Scenario:**
- WHEN task có review kết quả
- THEN task detail hiện badge Spec (pass/fail) + Quality (pass/fail) + expandable list lý do

## Modified Requirements

### REQ-M01: Backward compatibility
> ❌ Status: Not Started

**Scenario:**
- WHEN review của job cũ (trước feature) được render lại
- THEN UI hiển thị dạng single-verdict cũ không lỗi

## Removed Requirements
- Single-verdict duy nhất (thay bằng structured, có fallback).
