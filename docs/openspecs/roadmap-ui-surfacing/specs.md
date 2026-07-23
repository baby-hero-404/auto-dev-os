# Specs: Roadmap UI Surfacing

## Added Requirements

### REQ-001: Audit panel trên task detail
> ✅ Status: Completed

**Scenario:**
- WHEN mở task detail của task đã có PR và attestation records
- THEN section "Audit" hiển thị 1 row per commit: commit hash (short), coded_by (provider/model), reviewed_by (nếu có), key_id, timestamp
- AND mỗi row có verify badge: `Verified ✓` (xanh) khi API trả `verified: true`, `Tampered ✗` (đỏ) khi `false`

**Scenario:**
- WHEN task chưa có attestation nào (API trả list rỗng)
- THEN section Audit ẩn hoặc hiển thị empty state, không lỗi console

**Scenario:**
- WHEN click "View envelope" trên 1 row
- THEN modal hiển thị raw DSSE envelope JSON (pretty-printed), không nhúng sẵn full envelope vào page load ban đầu

### REQ-002: Governance presets endpoint
> ✅ Status: Completed

**Scenario:**
- WHEN GET `/api/v1/governance/presets`
- THEN trả `[{name: "api_native", config: {...}}, {name: "cli_spec_first", config: {...}}]` từ `governance.PresetNames`/`Preset()`
- AND endpoint yêu cầu auth như các route `/api/v1` khác

### REQ-003: Pipeline config editor với preset picker
> ✅ Status: Completed

**Scenario:**
- WHEN mở governance editor trong project settings
- THEN hiển thị JSON editor chứa `pipeline_config` hiện tại của project (hoặc empty state nếu null)
- AND preset picker cho phép chọn `api_native`/`cli_spec_first` → fill editor với preset JSON (chưa save)

**Scenario:**
- WHEN save `pipeline_config` không hợp lệ (sai schema hoặc DAG lỗi)
- THEN project-update trả 400, UI hiển thị inline danh sách validation errors ngay dưới editor, giá trị đang edit không bị mất

**Scenario:**
- WHEN save config hợp lệ
- THEN PATCH project thành công, toast xác nhận, editor phản ánh giá trị đã lưu

### REQ-004: Learned Skills management panel
> ✅ Status: Completed

**Scenario:**
- WHEN mở panel Learned Skills của 1 project
- THEN list các learned skill nhóm/filter theo status (draft/active/disabled), mỗi item hiển thị title, trigger_keywords, usage_count, success_count
- AND item có source_task_id hiển thị link tới task detail nguồn

**Scenario:**
- WHEN approve 1 skill draft
- THEN PATCH `/learned-skills/{id}` với `status: "active"`, list cập nhật không cần reload trang

**Scenario:**
- WHEN delete 1 skill
- THEN confirm trước khi DELETE; sau khi xoá item biến mất khỏi list

### REQ-005: Smart routing toggle
> ✅ Status: Completed

**Scenario:**
- WHEN mở project settings
- THEN thấy toggle "Smart LLM Routing" phản ánh giá trị `smart_routing` hiện tại của project

**Scenario:**
- WHEN bật/tắt toggle
- THEN PATCH project với `smart_routing: true|false`, toast xác nhận, giá trị persist sau reload

### REQ-006: Switch UI primitive
> ✅ Status: Completed

**Scenario:**
- WHEN dùng `<Switch checked onChange>` trong bất kỳ form settings nào
- THEN render boolean toggle theo design system hiện có (Tailwind, matching `components/ui/*` style), keyboard-accessible (space/enter), có disabled state

## Modified Requirements
- None (chỉ thêm UI/endpoint mới, không đổi behavior backend hiện có).

## Removed Requirements
- None.
