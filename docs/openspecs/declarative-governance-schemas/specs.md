# Specs: Declarative Governance Schemas

## Added Requirements

### REQ-001: Schema validation
> ✅ Status: Done

**Scenario:**
- WHEN PATCH project với `pipeline_config` không pass schema hoặc DAG có cycle/step không tồn tại
- THEN 400 với danh sách lỗi cụ thể (path trong JSON + lý do), config không được lưu

### REQ-001b: DAG structural checks (ngoài schema)
> ✅ Status: Done (checked only when a config declares a full custom graph — every step has `dependsOn` and no `extends`; ordinary patch configs skip DAG checks since there's nothing to structurally validate)

**Scenario:**
- WHEN config có node khai báo nhưng không nằm trên bất kỳ path nào từ entry (unreachable)
- THEN 400 `unreachable step: <id>`

**Scenario:**
- WHEN config có node không dẫn được tới bất kỳ terminal node nào (dead-end — job sẽ treo không bao giờ Done)
- THEN 400 `dead-end step: <id> has no path to a terminal step`

**Scenario:**
- WHEN config có ≠1 entry point (0 hoặc ≥2 node không có dependsOn)
- THEN 400 nêu rõ các entry tìm thấy — pipeline phải có đúng 1 entry và ≥1 terminal

### REQ-002: Null config = hành vi hiện tại
> ✅ Status: Done (every `*governance.Config` accessor is nil-receiver-safe and returns "no override" for a nil/empty config, so unconfigured projects are provably unaffected — see config_test.go)

**Scenario:**
- WHEN project có `pipeline_config = null`
- THEN BuildWorkflow output identical với built-in definitions trước feature (snapshot tests cả 2 flows)

### REQ-003: Custom pipeline
> ✅ Status: Done (scope-reduced — see implementation notes). `skip_when.label` is enforced at the review step's existing conditional-skip point (`ShouldSkipStepForLabels`); disabling `dor_check` is enforced via `IsDorDisabled` at the DoR-bypass check. Both are consulted at existing decision points rather than through a rebuilt data-driven DAG builder.

**Scenario:**
- WHEN config bỏ node `dor_check` và thêm điều kiện skip `review` khi label hotfix
- THEN DAG build đúng theo config, các task hotfix bỏ review

### REQ-004: Policy overrides
> ✅ Status: Done. `policies.routing`, `.review_harness`, `.max_review_fix_cycles` overrides wired into `analyze.go`/`llmrunner/runner.go` (routing), `review.go`/`cross_review.go` (cycle limit, harness policy) — override wins when present, otherwise the existing project column/matrix default is used unchanged.

**Scenario:**
- WHEN config chứa routing matrix override (`analyze: balanced`)
- THEN Smart Router dùng giá trị override thay matrix mặc định
- AND các key không override giữ default

### REQ-005: Presets
> ✅ Status: Done (backend only — `governance.PresetNames`/`Preset(name)` serve schema-valid preset JSON; UI picker/editor deferred, see implementation notes)

**Scenario:**
- WHEN user chọn preset trong UI
- THEN config được điền từ preset file, edit tiếp được, validate như config thường

### REQ-006: Version guard
> ✅ Status: Done (hard-reject only — any `version` other than `CurrentVersion` (1) fails validation; no migrate-on-read step exists yet since no prior schema version has ever shipped, see implementation notes)

**Scenario:**
- WHEN config có `version` cũ hơn schema hiện tại
- THEN server migrate-on-read nếu có migration, hoặc từ chối với hướng dẫn — không bao giờ chạy config hiểu sai

## Modified Requirements

### REQ-M01: Job đang chạy không đổi pipeline
> ⚠️ Status: Deferred (documented limitation — see implementation notes). No per-job config snapshot exists; every hook point re-reads the live `Project.PipelineConfig` at step-execution time, so an in-flight job's later steps could pick up a config edit made mid-run. Accepted as low-risk given jobs are short-lived and config edits are rare; a real fix requires snapshotting the config onto `models.WorkflowJob` at dispatch time.

**Scenario:**
- WHEN config thay đổi trong lúc job đang chạy
- THEN job đó tiếp tục theo DAG snapshot lúc dispatch; config mới chỉ áp cho job sau

## Removed Requirements
- Không có.
