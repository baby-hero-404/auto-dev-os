# Design: Declarative Governance Schemas

## Config shape (rút gọn)

```jsonc
{
  "version": 1,
  "pipeline": {
    "extends": "api_native",            // preset base, hoặc null = tự định nghĩa full
    "steps": [
      {"id": "dor_check", "enabled": false},
      {"id": "review", "skip_when": {"label": "hotfix"}}
    ]
  },
  "policies": {
    "routing": {"analyze": "balanced"},
    "review_harness": "different_provider",
    "max_review_fix_cycles": 5,
    "dor": {"require_acceptance_criteria": true}
  }
}
```

`extends` + patch-style overrides thay vì bắt define full DAG — case phổ biến là "default trừ 1-2 chỉnh", full-custom vẫn cho phép.

## Builder

`BuildWorkflow(engine, project)`:
1. Resolve base definition (preset theo engine hoặc `extends`).
2. Áp step overrides (enabled/skip_when/thêm node từ registry).
3. Validate DAG structural (JSON Schema chỉ check format — các check này bắt buộc code riêng, sai là worker treo job vĩnh viễn):
   - **Acyclic**: topo sort.
   - **Deps resolve**: mọi dependsOn trỏ tới step tồn tại trong registry.
   - **Exactly one entry**: đúng 1 node không có dependsOn.
   - **Reachability**: BFS từ entry — mọi node khai báo phải reachable (không node "mồ côi").
   - **No dead-ends**: reverse-BFS từ các terminal nodes — mọi node phải có path tới ≥1 terminal (node kẹt giữa chừng = job không bao giờ Done).
4. Snapshot definition JSON vào job record lúc dispatch (REQ-M01) — worker chạy theo snapshot, không re-read.

Step registry: map id → step implementation đã có (`orchestrator/steps/`); config chỉ compose, không định nghĩa step mới.

## Policy consumers

Mỗi consumer (dor_check, router, review resolver) nhận `policies` struct đã parse thay vì đọc field project riêng lẻ — refactor các cột `review_harness_policy`, `smart_routing`… thành computed view từ config + legacy columns (giữ backward-compat: cột cũ = override khi config null).

## Validation stack

`santhosh-tekuri/jsonschema` (pure Go, actively maintained) cho schema; DAG checks tự viết. Lỗi trả dạng `[{path, message}]` cho UI inline.

## Trade-offs

- Patch-style extends phức tạp hơn full-define nhưng khớp hành vi user thật; full-define vẫn hỗ trợ.
- JSON editor thay vì visual DAG builder: đúng effort/value cho v1; visual builder là sản phẩm riêng.
- Migrate-on-read cho version cũ chỉ khi có migration viết sẵn — không đoán.
