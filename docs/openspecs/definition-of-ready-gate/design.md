# Design: Definition-of-Ready Gate

## Readiness criteria (v1, cố ý đơn giản)

```go
type dorResult struct {
    HasAcceptanceCriteria bool // analysis JSON có key acceptance_criteria non-empty, HOẶC description chứa heading "Acceptance"/"AC:"
    HasFileScope          bool // analysis có files/scope list non-empty
    OpenClarifications    int
}
func (r dorResult) Ready() bool { return r.HasAcceptanceCriteria && r.HasFileScope && r.OpenClarifications == 0 }
```

Đánh giá thuần Go trên `task.Analysis` + `task.Clarifications` — LLM chỉ được gọi khi **không** ready, để sinh câu hỏi.

## Question generation

Prompt `dor_check.md`: input = task title/description + analysis + tiêu chí thiếu; output JSON `[{question, why_needed}]` max 5. Append vào `task.Clarifications`:

```json
{"id": "...", "question": "...", "why_needed": "...", "status": "open", "answer": null, "round": 1}
```

## Pause/resume

- Tái dùng cơ chế pause của boundary-resolution/spec-approval nếu có chung khung (khảo sát ở task 1.1 — `BoundaryResolutionControls.tsx` cho thấy đã có pattern chờ user input giữa pipeline). Nếu khung chung chưa có, đây là nơi tạo helper `PauseJob(reason)` mà `cli-spec-first-flow` REQ-004 cũng dùng — 2 spec sets phối hợp, ai làm trước tạo helper.
- Resume trigger: handler answer clarification đếm open==0 → enqueue lại job từ `dor_check`.

## Round limit

`round` trong clarification record; `dor_check` đếm max(round) ≥ 2 và vẫn thiếu → pass với `SpecStatus=ready_with_warnings`, log tiêu chí thiếu. Chống deadlock giữa gate và user không muốn trả lời thêm.

## UI

Panel trong task detail (vị trí cạnh `SupportingAccordion`): list câu hỏi open, textarea trả lời từng câu, badge `awaiting_clarification` trên status. API: `POST /tasks/{id}/clarifications/{cid}/answer`.

## CLI mode: không thêm step riêng

Với engine=cli (`cli_spec_first` flow), DoR **không** là node riêng — tách step nghĩa là tốn thêm 1 lần spawn CLI + context-load chỉ để check readiness. Thay vào đó:
- Readiness evaluation (thuần Go, không LLM) chạy như **precondition của cli_analyze** trong cùng step: fail tiêu chí → sinh câu hỏi (1 LLM call nhỏ qua API-native, không spawn CLI) → pause `awaiting_clarification` y hệt REQ-002/003.
- Sau khi clarifications answered, cli_analyze spawn CLI với answers trong prompt — chỉ 1 lần context-load thật.
- Spec set `cli-spec-first-flow` tham chiếu mục này; implement chung hàm `dorResult` (export từ package steps).

**Dependency trap — LLMClient trong CLI mode**: question-generation là API-native call, nghĩa là `cli_analyze` cần **cả** `ExecutionEngine` (CLI) **lẫn** `LLMClient` (DoR) được inject từ worker. Nhưng user chọn CLI mode có thể chính là để không cần API key — server không có key cấu hình thì call này sẽ fail.

Fallback bắt buộc: `LLMClient` unavailable (không key / lỗi cấu hình) → **bypass question-generation**, log warning `dor: no API-native LLM available, skipping clarification generation`, và pipeline tiếp tục (readiness evaluation thuần Go vẫn chạy và ghi `SpecStatus=ready_with_warnings` nếu thiếu tiêu chí). DoR không bao giờ được phép chặn cứng luồng CLI vì thiếu API key.

## Trade-offs

- Tiêu chí ready v1 là heuristic thô (key tồn tại, non-empty) — đủ chặn case "task 1 dòng mô tả"; scoring tinh hơn để governance schemas (P4.2) cấu hình.
- Gate sau `analyze` chứ không trước: cần analysis output để biết file scope; đổi lại tốn 1 lần analyze cho task chưa ready — chấp nhận, analyze rẻ hơn coding nhiều.
