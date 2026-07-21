# Proposal: Smart LLM Router + Token Usage Tracking (P3.3)

## Why

Mọi step hiện dùng model theo `DefaultModelLevel` của project bất kể độ khó — `context_load`/`analyze` đơn giản vẫn chạy model đắt như `coding`. 9Router (reference) chứng minh routing theo complexity tiết kiệm đáng kể. Prerequisite P0.1 (prompt caching + usage metrics trong log) đã cho số liệu thô; set này thêm persistence + routing policy. Task model đã có `Complexity` field (easy/…) — tín hiệu routing có sẵn.

## What Changes

### Issue 1: token_usage table

- Bảng `token_usage`: `task_id, job_id, step_id, provider, model, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, cost_estimate, created_at`.
- Ghi từ tool-loop sau mỗi LLM call (chỗ đang log usage). Cost estimate từ bảng giá tĩnh per-model trong config.

### Issue 2: Step-complexity routing policy

- Ma trận mặc định (config được, per-project override sau qua governance P4.2):
  - `context_load`, `analyze`, `dor_check` question-gen → model level `fast`
  - `plan`, `review` → `balanced`
  - `code_*`, `fix` → theo `DefaultModelLevel` project (thường `powerful`)
  - task `Complexity=easy` → hạ mỗi step 1 bậc (floor `fast`)
- Map model-level → model cụ thể đã tồn tại (DefaultModelLevel) — router chỉ chọn level per-step trước khi resolve.

### Issue 3: Cost dashboard tối thiểu

- API `GET /projects/{id}/usage?days=30` aggregate theo task/model/step.
- UI: card "Token Usage" trong project page (tổng cost, breakdown model, savings từ cache).

## Capabilities

### New Capabilities
- Persist token usage per LLM call; per-step model routing; usage API + UI card.

### Modified Capabilities
- Model resolution nhận step id + task complexity.

### Removed Capabilities
- Không có.

## Impact

| Area | Files Affected |
|------|----------------|
| Migration + repo | bảng `token_usage` + repository |
| LLM/tool-loop | ghi usage tại call-site trong `llmrunner` |
| Routing | model resolver (nơi DefaultModelLevel đang được map) |
| API/Web | usage endpoint + project page card |
