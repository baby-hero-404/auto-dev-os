# Tasks: Declarative Governance Schemas

> Prerequisite: Wave 1-3 xong (các thực thể cần cấu hình đã tồn tại). Làm cuối Wave 4.

- [ ] 1.1 Viết `docs/schemas/pipeline.schema.json` + `policies.schema.json` (version 1)
- [ ] 1.2 Presets: `api_native.json`, `cli_spec_first.json` sinh từ built-in definitions (kiểm bằng snapshot REQ-002)
- [ ] 1.3 Migration `projects.pipeline_config` + model + validation endpoint (schema + DAG checks) (REQ-001)
- [ ] 1.3b DAG structural checks: single-entry, BFS reachability, reverse-BFS dead-end + tests từng loại lỗi (REQ-001b)
- [ ] 1.4 Builder data-driven: extends + overrides + skip_when + snapshot vào job (REQ-002/003, REQ-M01)
- [ ] 1.5 Policies struct + refactor consumers (dor, router, review) với legacy-column compat (REQ-004)
- [ ] 1.6 Version guard + migrate-on-read khung (REQ-006)
- [ ] 1.7 UI: preset picker + JSON editor với inline errors (REQ-005)
- [ ] 1.8 Snapshot/property tests: null config identical; random valid config → acyclic
- [ ] 1.9 Update specs.md status + ARCHITECTURE.md
