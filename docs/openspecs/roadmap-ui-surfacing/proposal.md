# Proposal: Roadmap UI Surfacing

## Why

Wave 1–4 của `ROADMAP-cli-execution-engine.md` đã ship xong toàn bộ backend (trừ `feature-docs-sync`), nhưng 4 feature bị deferred phần UI theo pattern "backend-only, UI để sau" ghi trong từng `tasks.md`:

1. **attestation-audit-trail** REQ-005 — Audit panel + verify badge trên task detail. Backend endpoints (`/attestations/*`) đã đầy đủ, không có UI nào consume.
2. **declarative-governance-schemas** REQ-005 — preset picker + `pipeline_config` JSON editor. Validation chạy server-side khi update project, nhưng user không có cách nào edit/nhìn lỗi từ web.
3. **reusable-skills-system** 5.1/5.2 — trang quản lý Learned Skills (approve draft, activate/deactivate, usage stats). `LearnedSkillHandler` CRUD đầy đủ; trang `/skills` hiện tại là repo-sourced skills — feature khác hoàn toàn.
4. **smart-llm-router** 1.8 — toggle `smart_routing` trong project settings. Field tồn tại (default `true`), chỉ set được qua API trực tiếp vì UI settings không có primitive boolean toggle.

Hệ quả: các feature enterprise-facing (attestation, governance) hoàn toàn vô hình với người dùng web — giá trị đã build ra không được deliver.

## What Changes

### Issue 1: Attestation Audit panel (task detail)
- Thêm section "Audit" vào task detail hiển thị chain coded_by → reviewed_by → attested cho từng commit, kèm verify badge (`Verified ✓` / `Tampered ✗`) từ `verified` flag của API.
- Link "View envelope" mở raw DSSE envelope (JSON viewer / modal), không nhúng sẵn vào page.

### Issue 2: Governance pipeline_config editor (project settings)
- **Backend (nhỏ)**: `GET /api/v1/governance/presets` trả `{name, config}` cho 2 preset built-in — hiện presets chỉ tồn tại trong Go, UI không lấy được.
- Preset picker (`api_native` / `cli_spec_first`) + JSON editor cho `pipeline_config`, hiển thị inline validation errors trả về từ project-update 400 response.

### Issue 3: Learned Skills management (project scope)
- Tab/section "Learned Skills" trong project detail: list theo status (draft/active/disabled), approve draft → active, disable, delete, hiển thị `usage_count`/`success_count` + link `source_task_id`.

### Issue 4: Smart routing toggle (project settings)
- Thêm boolean toggle primitive (Switch) vào UI kit — codebase hiện chỉ có `<Select>` cho settings.
- Wire toggle `smart_routing` vào `project-profile.tsx`.

## Capabilities

### New Capabilities
- Audit panel với per-commit verify badge trên task detail.
- Governance config editor với preset picker + inline errors.
- Learned Skills lifecycle management UI.
- `GET /api/v1/governance/presets` endpoint.
- Reusable `Switch` UI primitive.

### Modified Capabilities
- Project settings (`project-profile.tsx`) — thêm smart_routing toggle + governance editor entry.
- Task detail layout — thêm Audit section.

### Removed Capabilities
- None.

## Impact

| Area | Files Affected |
|------|----------------|
| Backend handler | `server/internal/handler/governance.go` (new), `server/internal/handler/router.go` |
| Web API client | `web/src/lib/api/attestations.ts` (new), `web/src/lib/api/governance.ts` (new), `web/src/lib/api/learned-skills.ts` (new), `web/src/lib/api/projects.ts` |
| Task detail | `web/src/app/projects/[id]/tasks/[taskID]/components/AuditPanel.tsx` (new), `TaskDetailLayout.tsx` |
| Project settings | `web/src/components/projects/project-profile.tsx`, `web/src/components/projects/GovernanceConfigEditor.tsx` (new), `web/src/components/projects/LearnedSkillsPanel.tsx` (new) |
| UI kit | `web/src/components/ui/switch.tsx` (new) |
| Types | `web/src/lib/types.ts` |
