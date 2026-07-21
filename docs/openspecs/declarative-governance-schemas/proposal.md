# Proposal: Declarative Governance Schemas (P4.2)

## Why

Đến Wave 4, project sẽ tích lũy nhiều lựa chọn hard-code: 2 workflow definitions (api_native, cli_spec_first), DoR criteria, routing matrix, review policy, quality gates. ai-sdlc (reference) chứng minh mô hình "governance là configuration": JSON Schema định nghĩa pipeline/gate/policy, user chỉnh per-project không cần code. Set này tổng quát hóa những gì Wave 1-3 đã xây — vì vậy phải làm **sau cùng**, khi các thực thể cần cấu hình đã tồn tại và ổn định.

## What Changes

### Issue 1: Schema definitions

- `docs/schemas/pipeline.schema.json`: steps, dependsOn, điều kiện bật/tắt node (vd skip dor_check cho hotfix), engine bindings.
- `docs/schemas/policies.schema.json`: DoR criteria, review harness policy, routing matrix override, retry/cycle limits.
- Schemas versioned (`$id` + `version`), validate bằng thư viện JSON Schema Go.

### Issue 2: Per-project pipeline config

- Cột `projects.pipeline_config` (jsonb, nullable — null = built-in defaults, chính là hành vi hard-code hiện tại).
- `BuildWorkflow` đọc config: xây DAG từ config thay vì switch cứng; built-in definitions trở thành 2 config mẫu ship sẵn (`presets/api_native.json`, `presets/cli_spec_first.json`).
- Validation khi save: schema pass + DAG hợp lệ (acyclic, steps tồn tại trong registry, dependsOn resolve).

### Issue 3: UI

- Project settings: chọn preset hoặc edit JSON (editor với schema validation lỗi inline). Không visual DAG editor trong phase này.

## Capabilities

### New Capabilities
- Pipeline/policy per-project bằng config đã validate; presets ship sẵn.

### Modified Capabilities
- `BuildWorkflow` data-driven; DoR/routing/review đọc override từ config.

### Removed Capabilities
- Hard-coded làm nguồn duy nhất (vẫn giữ làm default khi config null).

## Impact

| Area | Files Affected |
|------|----------------|
| Schemas | `docs/schemas/*.json`, `presets/*.json` |
| Workflow | `server/internal/workflow/step.go` (data-driven builder) |
| Models | `project.go` (`pipeline_config`) + validation |
| Consumers | dor_check, router, review policy đọc override |
| Web | settings editor + preset picker |
