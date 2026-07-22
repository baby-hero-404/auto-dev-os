# Specs: CLI Spec-First Flow

## Added Requirements

### REQ-001: Workflow selection theo engine
> ✅ Status: Done

**Scenario:**
- WHEN task được dispatch với engine resolve = `cli`
- THEN workflow definition `cli_spec_first` (cli_analyze → cli_spec → cli_implement → cli_mr) được build
- AND WHEN engine = `api_native` THEN DAG hiện tại được build, không khác biệt so với trước feature này

### REQ-002: cli_analyze
> ✅ Status: Done

**Scenario:**
- WHEN step cli_analyze chạy
- THEN CLI được spawn với prompt analyze (repo + task description) yêu cầu ghi `.autocode/analysis.md`
- AND server đọc file đó, lưu nội dung parse được vào `task.Analysis`, fail step nếu file không tồn tại sau khi CLI exit 0

### REQ-003: cli_spec — OpenSpec authoring
> ✅ Status: Done

**Scenario:**
- WHEN step cli_spec chạy
- THEN CLI authoring đủ 4 files `proposal.md`, `specs.md`, `design.md`, `tasks.md` vào `docs/openspecs/<task-slug>/` trong worktree
- AND step fail với message liệt kê file thiếu nếu không đủ 4 files
- AND `tasks.md` phải chứa ít nhất 1 checkbox, parse bằng regex khoan dung `(?im)^\s*[-*]\s*\[([ xX])\]` (LLM hay viết `- [X]`, `* [x]`, thừa space — exact match `- [ ]` sẽ đếm sai)

### REQ-004: Spec approval gate
> ✅ Status: Done

**Scenario:**
- WHEN project autonomy = `supervised` và cli_spec hoàn thành
- THEN pipeline pause ở trạng thái `awaiting_spec_approval`, UI hiển thị spec + nút Approve / Request changes
- AND WHEN user Approve THEN cli_implement được dispatch
- AND WHEN user Request changes (kèm comment) THEN cli_spec chạy lại với comment trong prompt

**Scenario:**
- WHEN autonomy = `autonomous`
- THEN gate được bỏ qua, cli_implement chạy ngay

### REQ-005: cli_implement
> ✅ Status: Done

**Scenario:**
- WHEN cli_implement chạy
- THEN CLI được prompt implement theo spec set (đường dẫn spec truyền trong prompt), yêu cầu tick checkboxes trong `tasks.md`
- AND kết quả đánh giá bằng git diff: có thay đổi ngoài `docs/openspecs/` → pass sang checkpoint; chỉ có spec không có code → fail "implement produced no code changes"
- AND WHEN task có label `docs-only` HOẶC proposal.md của spec set khai `type: documentation` → rule trên được bypass: chỉ cần diff non-empty (kể cả chỉ .md) là pass — task "update README"/"draft design doc" không bị fail oan
- AND checkpoint được tạo qua `CreateGitCheckpoint` như code steps hiện tại

### REQ-006: cli_mr
> ✅ Status: Done

**Scenario:**
- WHEN cli_implement thành công
- THEN PR step hiện có push branch + tạo merge request chứa cả spec set lẫn code changes
- AND PR description dẫn lại nội dung `proposal.md` (Why/What Changes)

### REQ-007: Spec panel trong task detail
> ✅ Status: Done

**Scenario:**
- WHEN task chạy CLI flow đã qua cli_spec
- THEN task detail hiển thị panel Spec render proposal + tasks checkboxes (đọc từ worktree)
- AND tiến độ checkbox cập nhật sau cli_implement

## Modified Requirements

### REQ-M01: BuildWorkflow signature
> ✅ Status: Done

**Scenario:**
- WHEN `BuildWorkflow` được gọi
- THEN nhận thêm engine (hoặc task+project) để chọn definition; call-sites hiện có compile và giữ hành vi cũ khi engine=api_native

## Removed Requirements
- Không có.
