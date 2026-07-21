# Specs: Declarative Governance Schemas

## Added Requirements

### REQ-001: Schema validation
> ❌ Status: Not Started

**Scenario:**
- WHEN PATCH project với `pipeline_config` không pass schema hoặc DAG có cycle/step không tồn tại
- THEN 400 với danh sách lỗi cụ thể (path trong JSON + lý do), config không được lưu

### REQ-001b: DAG structural checks (ngoài schema)
> ❌ Status: Not Started

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
> ❌ Status: Not Started

**Scenario:**
- WHEN project có `pipeline_config = null`
- THEN BuildWorkflow output identical với built-in definitions trước feature (snapshot tests cả 2 flows)

### REQ-003: Custom pipeline
> ❌ Status: Not Started

**Scenario:**
- WHEN config bỏ node `dor_check` và thêm điều kiện skip `review` khi label hotfix
- THEN DAG build đúng theo config, các task hotfix bỏ review

### REQ-004: Policy overrides
> ❌ Status: Not Started

**Scenario:**
- WHEN config chứa routing matrix override (`analyze: balanced`)
- THEN Smart Router dùng giá trị override thay matrix mặc định
- AND các key không override giữ default

### REQ-005: Presets
> ❌ Status: Not Started

**Scenario:**
- WHEN user chọn preset trong UI
- THEN config được điền từ preset file, edit tiếp được, validate như config thường

### REQ-006: Version guard
> ❌ Status: Not Started

**Scenario:**
- WHEN config có `version` cũ hơn schema hiện tại
- THEN server migrate-on-read nếu có migration, hoặc từ chối với hướng dẫn — không bao giờ chạy config hiểu sai

## Modified Requirements

### REQ-M01: Job đang chạy không đổi pipeline
> ❌ Status: Not Started

**Scenario:**
- WHEN config thay đổi trong lúc job đang chạy
- THEN job đó tiếp tục theo DAG snapshot lúc dispatch; config mới chỉ áp cho job sau

## Removed Requirements
- Không có.
