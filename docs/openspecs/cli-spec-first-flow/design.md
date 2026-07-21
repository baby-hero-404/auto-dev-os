# Design: CLI Spec-First Flow

## 1. Pipeline

```
engine=cli:
  cli_analyze ──▶ cli_spec ──▶ [awaiting_spec_approval]* ──▶ cli_implement ──▶ cli_mr
                                (* chỉ khi autonomy=supervised)

engine=api_native: DAG hiện tại, không đổi.
```

Mỗi `cli_*` step (trừ cli_mr) = một lần `cliEngine.RunCodeStep` (Phase 1 interface) với prompt template riêng trong `server/internal/prompts/steps/`. Cùng worktree xuyên suốt job — CLI giữ được context qua chính filesystem (analysis.md, spec set) thay vì server phải truyền state.

## 2. Step contracts (file-based, không parse CLI output)

Triết lý: mọi hợp đồng giữa server và CLI black-box là **file trong worktree** — nhất quán với "đánh giá bằng git diff" của Phase 1.

| Step | Server ghi trước khi spawn | CLI phải tạo ra | Server validate sau |
|------|---------------------------|-----------------|--------------------|
| cli_analyze | `.autocode/prompt.md` (task desc + yêu cầu output format) | `.autocode/analysis.md` | file tồn tại, non-empty → parse sections vào `task.Analysis` (jsonb `{raw_markdown, tech_stack?, files?, risks?}` — parse best-effort, raw luôn giữ) |
| cli_spec | prompt kèm nội dung analysis.md + OpenSpec convention (4-file template nhúng trong prompt) | `docs/openspecs/<task-slug>/{proposal,specs,design,tasks}.md` | đủ 4 files; `tasks.md` có ≥1 `- [ ]` |
| cli_implement | prompt kèm đường dẫn spec set + yêu cầu tick checkbox | code changes + checkboxes ticked | git diff có file ngoài `docs/openspecs/` (bypass khi docs-only — xem dưới); đếm checkbox cho progress |
| cli_mr | — (không spawn CLI) | — | reuse `steps/pr.go` |

`<task-slug>` dùng cùng slug function với branch naming (đã chuẩn hóa ở commit 65564f0).

**Checkbox parsing** (mọi chỗ đếm tasks.md): helper chung `parseCheckboxes(md) (done, total int)`, 2 bước bắt buộc:
1. **Strip fenced code blocks trước** (mọi nội dung giữa cặp ``` ``` ``` — kể cả ``` ```markdown ``` fence): LLM hay viết ví dụ checkbox trong code block khi giải thích → đếm trúng sẽ đội `total` ảo.
2. Chạy regex khoan dung `(?im)^\s*[-*]\s*\[([ xX])\]` trên phần còn lại — LLM format checkbox không nhất quán (`- [X]`, `* [x]`, thừa/thiếu space); exact-match `- [ ]` sẽ đếm sai.

**Docs-only bypass**: validation "phải có diff ngoài `docs/openspecs/`" bị bypass khi task label `docs-only` hoặc `proposal.md` frontmatter `type: documentation` — khi đó chỉ cần diff non-empty.

Lưu ý phạm vi thật của bypass: task "update README" **tự pass điều kiện gốc** (README.md nằm ngoài `docs/openspecs/`) — không cần bypass. Bypass chỉ cứu đúng loại task mà **toàn bộ sản phẩm nằm trong `docs/openspecs/`** (vd "phân tích và viết OpenSpec cho tính năng X, chưa code").

**Bẫy frontmatter**: agent không tự nghĩ ra `type: documentation`. Prompt template `cli_spec.md` PHẢI chứa chỉ dẫn tường minh: *"If this task requires writing specs/docs ONLY and no application code, you MUST include YAML frontmatter `type: documentation` in proposal.md."* Thiếu câu này, đường bypass qua frontmatter chết — chỉ còn đường label do user gắn.

## 2b. DoR precondition (không phải step riêng)

Khi `definition-of-ready-gate` ship: readiness check (thuần Go) chạy như precondition **bên trong cli_analyze** trước khi spawn CLI — không thêm node vào DAG, không tốn spawn/context-load thừa. Thiếu tiêu chí → sinh câu hỏi qua API-native call nhỏ → pause `awaiting_clarification`; answers được nhúng vào prompt cli_analyze khi resume. Chi tiết: `definition-of-ready-gate/design.md` §CLI mode.

## 3. Approval gate

- Trạng thái mới trên job/step: sau cli_spec success và autonomy=supervised, worker set step `cli_implement` sang trạng thái chờ (`awaiting_approval`) thay vì dispatch — theo đúng cơ chế boundary-resolution/pause đã có nếu tồn tại (kiểm tra `BoundaryResolutionControls.tsx` flow hiện tại trước khi thêm mới; ưu tiên tái dùng).
- API: `POST /tasks/{id}/spec-review` body `{action: "approve" | "request_changes", comment?}`.
  - approve → dispatch cli_implement.
  - request_changes → re-dispatch cli_spec, comment được append vào prompt (`## Reviewer feedback` section); đếm số vòng, max = `project.MaxReviewFixCycles`.

## 4. Đọc spec từ worktree cho UI

Endpoint mới `GET /tasks/{id}/spec` → server đọc `docs/openspecs/<task-slug>/*.md` từ host worktree path (`repoutil.HostWorktreePath`), trả `{proposal, specs, design, tasks, progress: {done, total}}`. Read-only, 404 khi chưa có. Frontend render markdown (component render md đã có trong task detail — tái dùng).

## 5. Prompt templates (3 file mới)

- `cli_analyze.md`: vai trò analyst; cấm sửa code; output đúng path `.autocode/analysis.md` với sections cố định.
- `cli_spec.md`: nhúng 4-file OpenSpec convention (rút từ chính skill openspec-authoring của repo này); cấm implement; chỉ ghi vào `docs/openspecs/<task-slug>/`.
- `cli_implement.md`: bám `specs.md` scenarios làm acceptance; yêu cầu chạy test nếu có; tick checkbox `tasks.md`; cấm sửa ngoài scope spec.

Guard nhẹ: server không thể ép CLI tuân thủ (black-box) — validation sau-step (mục 2) mới là enforcement thật.

## 6. Timeline UI

4 step mới đăng ký tên hiển thị: Analyze / Author Spec / Implement / Merge Request. Panel Spec đặt trong `SupportingAccordion` hoặc tab mới cạnh Checkpoints (`CheckpointsPanel.tsx` pattern). Trạng thái `awaiting_spec_approval` hiển thị controls approve — reuse visual pattern của `BoundaryResolutionControls.tsx`.

## 7. Trade-offs

- **2 workflow definitions thay vì 1 định nghĩa cấu hình được**: đơn giản trước; declarative governance schemas là P4.2 trong roadmap, sẽ tổng quát hóa sau.
- **Spec commit vào repo user**: chủ đích — spec là deliverable, reviewer thấy trong MR. Nếu user không muốn, cấu hình exclude là enhancement sau.
- **Không có review/test step riêng trong CLI flow**: CLI agent tự test trong tool-loop của nó; cross-harness review (API-native review CLI diff) là P3.1, sẽ chèn thêm step khi đến lượt.
