# Tasks: Roadmap UI Surfacing

**Goal:** Surface 4 backend features đã ship (attestation, governance config, learned skills, smart routing) lên web UI.

**Architecture:** Chỉ 1 endpoint backend mới (`GET /governance/presets`); còn lại là web components consume API có sẵn theo pattern `useAuthedSWR` + `api.*` client + sonner toast. Chi tiết trong `design.md`.

**Tech Stack:** Next.js app router, Tailwind, SWR, chi router (Go) cho endpoint mới.

---

## P0 — Attestation Audit panel (REQ-001)

- [x] 1.1 `web/src/lib/types.ts`: thêm `Attestation`, `AttestationVerifyResult` (shape trong design.md)
- [x] 1.2 `web/src/lib/api/attestations.ts` (new): `listByTask(taskID)`, `getByCommit(commit)` theo pattern client hiện có (`web/src/lib/api/*.ts`); export qua `index.ts`
- [x] 1.3 `web/src/app/projects/[id]/tasks/[taskID]/components/AuditPanel.tsx` (new): list attestations per commit (short hash, coded_by, reviewed_by, key_id, timestamp); empty state khi list rỗng; lazy verify badge per row (gọi `getByCommit` khi panel mở); modal "View envelope" pretty-print JSON, fetch on click
- [x] 1.4 Mount AuditPanel vào `TaskDetailLayout.tsx` (qua `SupportingAccordion.tsx`) cạnh CheckpointsPanel, dùng taskID từ `TaskDetailContext` — follow đúng cách `ReviewVerdictCard` được mount
- [x] 1.5 Verify: task có attestation → panel render + badge đúng theo `verified`; task không có → empty state, không console error

## P0 — Governance presets endpoint (REQ-002)

- [x] 2.1 `server/internal/handler/governance.go` (new): `GovernanceHandler.ListPresets` loop `governance.PresetNames` → `governance.Preset(name)` → trả `[]{name, config}`
- [x] 2.2 Register `r.Get("/governance/presets", ...)` trong route block `/api/v1` đã auth của `router.go`
- [x] 2.3 Test: `server/internal/handler/governance_test.go` — 200 trả đủ 2 preset với config JSON hợp lệ; theo pattern handler test hiện có
- [x] 2.4 `go build ./... && go vet ./... && go test ./...` xanh

## P1 — Governance config editor (REQ-003)

- [x] 3.1 `web/src/lib/api/governance.ts` (new): `listPresets()`; `web/src/lib/types.ts`: `GovernancePreset`
- [x] 3.2 `web/src/components/projects/GovernanceConfigEditor.tsx` (new): monospace textarea load `project.pipeline_config` (pretty-printed, empty state nếu null); preset picker fill editor (chưa save); client-side `JSON.parse` check trước khi PATCH; hiển thị inline errors từ 400 response body dưới editor, giữ nguyên giá trị đang edit; toast khi save thành công
- [x] 3.3 Mount vào project settings (`project-profile.tsx`)
- [x] 3.4 Verify: save config sai schema → thấy lỗi inline; chọn preset → editor fill; save hợp lệ → persist sau reload

## P1 — Smart routing toggle (REQ-005, REQ-006)

- [x] 4.1 `web/src/components/ui/switch.tsx` (new): boolean toggle, `role="switch"` + `aria-checked`, keyboard accessible, disabled state, style theo `components/ui/*` hiện có
- [x] 4.2 `project-profile.tsx`: thêm row "Smart LLM Routing" dùng Switch, PATCH `smart_routing` on change, toast xác nhận; `Project` type thêm `smart_routing: boolean` nếu thiếu
- [x] 4.3 Verify: toggle → persist sau reload; API nhận đúng `smart_routing: false`

## P2 — Learned Skills panel (REQ-004)

- [x] 5.1 `web/src/lib/types.ts`: `LearnedSkill`; `web/src/lib/api/learned-skills.ts` (new): `listByProject`, `update`, `remove`
- [x] 5.2 `web/src/components/projects/LearnedSkillsPanel.tsx` (new): bảng filter theo status; hiển thị title, trigger_keywords, usage_count, success_count, link source task; actions: approve (draft→active), disable, delete (có confirm); SWR mutate sau mỗi action
- [x] 5.3 Mount vào project detail page / settings — project-scoped, KHÔNG đụng trang `/skills` global; label rõ "Learned Skills"
- [x] 5.4 Verify: approve draft → status đổi không reload; delete → biến mất; empty state khi project chưa có skill nào

## Closeout

- [x] 6.1 Update status các REQ trong `specs.md` + deviation notes tại đây
- [x] 6.2 Update `tasks.md` gốc của 4 spec cũ: đổi note "Deferred" → trỏ tới set này
- [x] 6.3 Update `ROADMAP-cli-execution-engine.md` nếu cần

## Docs sync

- [x] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md
