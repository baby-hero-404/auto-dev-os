# Báo cáo phân tích: API Mode vs CLI Mode trong Execution Engine

**Ngày:** 2026-07-29
**Phạm vi:** `server/internal/orchestrator`, `server/internal/prompts`, `server/pkg/models/cli_profiles.go`
**Mục đích:** So sánh sâu 2 execution path hiện có (API tool-calling vs CLI headless agent), chỉ ra điểm khác biệt cấu trúc, và đề xuất hướng hài hòa (harmonize) mà không phá vỡ bản chất của từng mode.

---

## 1. Bối cảnh: 2 pipeline song song, không phải 1 pipeline có 2 chế độ

Cả 2 mode được chọn **một lần cho toàn bộ workflow** của 1 task, qua `Orchestrator.ResolveExecutionProvider` (`execution_router.go:77`) → trả về `resolved.Type == "api"` hoặc `"cli"`. Từ đó, toàn bộ DAG bước (step graph) rẽ nhánh:

| | API mode | CLI mode |
|---|---|---|
| Step graph | `context_load → analyze → plan → code_backend/code_frontend → review → ...` | `cli_analyze → cli_spec → cli_implement → cross_review → cli_mr` |
| Đơn vị thực thi | LLM message-loop nội bộ (`llmrunner.RunToolLoop`) | Container spawn 1 binary CLI (agy/claude/codex) |
| "Tool" là gì | JSON schema function-call do harness định nghĩa & chạy | Built-in shell/file tool bên trong chính binary CLI |
| Kiểm soát output | Structured JSON, validate schema nghiêm ngặt | File contract (đọc file CLI ghi ra, hoặc `git diff` đo thay đổi) |

Đây là điểm nền tảng: **không phải "CLI = API thiếu tool"**, mà là 2 kiến trúc thực thi khác hẳn nhau — một cái harness-controlled (mình lái từng tool call), một cái agent-autonomous (giao việc rồi để nó tự chạy vòng lặp riêng trong container).

---

## 2. So sánh chi tiết theo từng trục

### 2.1 Cách prompt được build

**API (`analyze.go`)**
- `PromptAssembler.AssembleForAgent` — lắp ráp modular từ nhiều `PromptSection` (global rules, role prompt, project rules strict/advisory, git diff, execution contract, task spec, step prompt, tool list), có **priority-based budget pruning** (cắt bớt section thấp ưu tiên khi vượt token budget).
- `resolveSkills` chọn 3-5 skill liên quan nhất (JIT) dựa trên role match / required_skills_map / keyword match — chỉ đưa **tên + description** vào dưới dạng `llm.ToolDefinition` (model tự quyết định có "gọi" skill đó không qua tool-calling).
- `formatRepoContext` (`analyze.go:829-972`) tổng hợp: git log/branch, test command, CI config, conventions/architecture/CONTRIBUTING (đọc từ file trong repo), **learned_skills** (từ `LearnedSkillRepo` — Postgres), context-cache (repo map, semantic snippet) do bước `context_load` trước đó tính sẵn.
- Kết quả: `[]llm.Message` + `[]llm.ToolDefinition`, model tự chọn gọi tool nào.

**CLI (`cli_analyze.go`, `cli_spec.go`, `cli_implement.go`)**
- Instruction là **string phẳng, ghép trực tiếp**: `base .md step prompt + "## Task" + title + description` (+ vài field phụ như slug, analysis raw markdown, reviewer feedback).
- **Không có**: project rules, git diff, skills (dù là tên hay nội dung), tool registry, context-cache, budget pruning.
- Lý do (đã verify bằng code): binary CLI có sẵn shell access trong container (`git`, `cat`, `grep`, file read/write) nên **tự khám phá được** git diff, file convention, CONTRIBUTING.md, v.v. — không cần harness "mớm" cho nó.

→ Đây chính là premise user đã chỉ ra: nếu bê nguyên `PromptAssembler` pipeline sang CLI thì **trùng lặp thông tin CLI tự lấy được**, tốn token, sai bản chất kiến trúc.

### 2.2 Vòng lặp thực thi & kiểm soát output

**API**: `runAnalyzeToolLoop` — `llmrunner.RunToolLoop`, tối đa 12 iteration, model gọi tool → harness chạy tool → trả kết quả → model tiếp tục — đến khi model trả JSON cuối. `validateAnalyzeSpec` kiểm tra schema rất chặt: `complexity`, `primary_category`, `execution_phases`, `affected_files`, `acceptance_criteria`, `execution_boundaries`, `proposal_md`, `specs_md`, `design_md`, `execution_units`, `execution_irs[]` (nested), `required_skills_map` (role-key validate chặt theo whitelist role).

**CLI**: `cliStepRunner.RunCLIStep` → `engine.NewCLIEngine` spawn container, chạy `Command + Args` (đã templating `{prompt_file}`/`{workdir}`), chờ exit. Không có vòng lặp nào do harness lái — toàn bộ "reasoning loop" nằm trong chính binary CLI (agy/claude/codex tự explore, tự sửa file, tự dừng).
- Kiểm soát output theo **file contract**, mỗi step tự định nghĩa hợp đồng riêng, lỏng hơn nhiều so với JSON schema của API:
  - `cli_analyze`: đọc `.autocode/analysis.md`, chỉ cần non-empty (`cli_analyze.go`).
  - `cli_spec`: check tồn tại đủ 4 file `proposal.md/specs.md/design.md/tasks.md` + `tasks.md` có ít nhất 1 checkbox (`validateSpecFiles`).
  - `cli_implement`: check `len(out.ChangedFiles) > 0` + có thay đổi ngoài `docs/openspecs/` (trừ khi task gắn nhãn `docs-only` hoặc proposal khai `type: documentation`) — "evaluate by diff" thay vì evaluate by schema.

→ API tin vào **structured contract do model tự trả**; CLI tin vào **side-effect quan sát được** (file tồn tại, git diff không rỗng). Đây là khác biệt triết lý cốt lõi, không chỉ là thiếu tool.

### 2.3 Invocation mechanism (chỉ CLI mode có)

- `cli_profiles.go`: registry tĩnh `Command/Args/AuthCheckCommand/TimeoutMinutes/CredentialProvider`, dùng placeholder `{prompt_file}`/`{workdir}`.
- `cli.go`/`RunCodeStep`: ghi instruction ra file host-side `.autocode/prompt.md` trước khi spawn container (tránh giới hạn `MAX_ARG_STRLEN`/128KB của Linux argv) → `{prompt_file}` được thay bằng path phía container.
- Đã xác nhận qua thực nghiệm (đợt fix trước): mỗi CLI thực (`claude`, `codex`, `agy`) có cách nhận prompt & flag khác nhau hoàn toàn (xem `docs/guides/*-cli-headless.md`) — đây là điểm CLI mode "trả giá" cho việc dùng agent có sẵn: phải tự tay chuẩn hoá per-tool quirks (thứ tự flag, tên flag auto-approve, cách CLI đọc file vs literal text).
- API mode không có lớp này — chỉ gọi thẳng LLM API, không cần quan tâm quirk của từng binary.

### 2.4 Nguồn skill / "kiến thức nền" — điểm mấu chốt của phân tích

Đã trace code (`assembler.go`, `builder.go`):

| Nguồn | Vị trí thực | CLI tự thấy được? |
|---|---|---|
| Git diff, branch, log, CI config | Chạy lệnh trong repo (container) | ✅ Có |
| Conventions/architecture/CONTRIBUTING | File committed trong repo | ✅ Có |
| Project-disk skills | `filepath.Join(a.dataRoot, schemaPath)` — **`dataRoot` là `.data/` của server**, không nằm trong repo | ❌ Không |
| Database skills | `a.skills.List(ctx)` — bảng Postgres | ❌ Không |
| `learned_skills` | `LearnedSkillRepo` (Postgres), tổng hợp từ task đã merge trước | ❌ Không |
| `task.Analysis.TaskRules` / `RequiredSkillsMap` | Cột `analysis` (JSON) trong DB task record | ❌ Không |

→ Nhóm "❌ Không" là **candidate hợp lệ duy nhất** để inject vào CLI instruction — vì đây là thông tin sống trong platform của mình, container CLI dù có full shell cũng không với tới được (không có network tới DB của app, không mount `.data/`).

---

## 3. Rủi ro/vấn đề hiện tại của CLI mode (quan sát được qua debug thực tế)

1. **Không có validate schema chặt** → lỗi phát hiện muộn hơn (vd. `.autocode/analysis.md` từng bị agy ghi sai nội dung do bug arg-order, chỉ phát hiện khi file rỗng/sai định dạng, không có schema báo lỗi ngay).
2. **Per-binary quirks dễ vỡ khi CLI update version** (đã xảy ra: `agy --headless` không tồn tại, `-p` là value-flag nhạy thứ tự) — API mode miễn nhiễm với việc này vì không phụ thuộc CLI binary nào.
3. **Zero context về learned_skills/project rules** — CLI hiện tại "mù" hoàn toàn với tri thức tích luỹ của platform (best practice riêng của project, bài học từ task trước) — điều mà API mode có.
4. **Silent duplication risk nếu làm sai hướng**: nếu sau này ai đó "port" nguyên formatRepoContext sang CLI (như 3 option brainstorm ban đầu tôi từng đề xuất — đã bị bạn bác đúng), sẽ gây lãng phí token vì CLI tự lấy lại được các phần đó bằng tool riêng, có khi còn conflict (thông tin platform đưa vs. thông tin CLI tự đọc bị lệch pha do timing).

---

## 4. Hướng đi hài hoà đề xuất (best practice)

**Nguyên tắc chủ đạo**: Không đồng bộ hoá 2 pipeline thành 1 cấu trúc chung. Thay vào đó, định nghĩa một **"platform-only knowledge" layer nhỏ, dùng chung cho cả 2 mode**, nhưng **render khác nhau** theo đúng bản chất từng mode:

- API mode: layer này tiếp tục là 1 phần của `PromptAssembler` (đã có sẵn, không đổi).
- CLI mode: layer này được trích xuất thành 1 **hàm mới, độc lập**, kiểu `BuildCLIPlatformContext(ctx, task) (string, error)` sống trong `internal/prompts` — chỉ gồm 3 phần đã xác nhận CLI-blind:
  1. Nội dung đầy đủ (không phải chỉ tên) của skill liên quan — tái dùng `resolveSkills`/`loadAllSkills` đã có, lấy `ParsedSkill.Content`, giới hạn 3-5 skill như API đang làm (tránh phình prompt).
  2. `learned_skills` text (đã có sẵn logic build trong `formatRepoContext`, chỉ cần tách hàm ra dùng chung).
  3. `task.Analysis.TaskRules` (nếu có) — rule cụ thể do bước `analyze` API trước đó sinh ra (áp dụng cho case dự án dùng CLI cho `cli_implement` sau khi đã `analyze` bằng API, nếu workflow cho phép mix — hiện tại 2 pipeline tách biệt hoàn toàn nên phần này áp dụng ít, nhưng nên chừa chỗ).
- **Không đưa vào CLI**: git diff, conventions/architecture files, tool list, execution contract JSON schema — để CLI tự khám phá bằng chính công cụ của nó, giữ đúng triết lý "agent tự trị".

### Việc cụ thể nên làm (sơ bộ, chưa implement):

1. Tách phần build `learned_skills` string ra khỏi `formatRepoContext` thành hàm riêng dùng chung (`BuildLearnedSkillsSection` hay tương tự) — tránh duplicate logic.
2. Thêm hàm `PromptAssembler.SkillsContentForCLI(ctx, task) ([]ParsedSkill, error)` — reuse `resolveSkills` (đã có scoring logic), trả full `Content` thay vì chỉ `ToolDefinition`.
3. Sửa `cli_analyze.go`/`cli_spec.go`/`cli_implement.go`: sau khi build `instruction` gốc, append 1 section `## Platform Knowledge` chứa (2) và (1) nếu có, giữ format Markdown đơn giản (CLI đọc Markdown tốt, không cần JSON).
4. **Không** động vào cách CLI tự đọc git diff/conventions — giữ nguyên hiện trạng.
5. Cân nhắc thêm 1 dòng validate nhẹ ở `cli_analyze`/`cli_spec`: nếu muốn siết chặt hơn `.autocode/analysis.md`, có thể thêm check tối thiểu (có heading `## Tech Stack` chẳng hạn) mà không cần full JSON schema như API — giữ đúng "file contract, không phải structured contract" cho CLI.
6. Ghi lại quyết định này vào `docs/ai/` (ADR) theo quy ước CLAUDE.md, để lần sau không ai đề xuất "port toàn bộ PromptAssembler sang CLI" nữa.

### Không nên làm

- Không tạo 1 `AssembleForCLI` dùng chung 100% pipeline với API (đã bị bác — đúng).
- Không cố ép CLI trả JSON schema chặt như API — làm vậy triệt tiêu lợi thế "agent tự trị, tự quyết cách làm" của CLI, và các CLI thực tế (agy/claude/codex) không được thiết kế để tuân thủ JSON schema nghiêm ngặt qua text instruction một cách đáng tin cậy.
- Không chuẩn hoá per-binary flags thành 1 abstraction chung phức tạp — giữ `cli_profiles.go` đơn giản như hiện tại (đã đủ tốt sau các fix gần đây), chỉ cần review lại mỗi khi CLI version đổi (đã ghi chú trong docs).

---

## 5. Tóm tắt 1 dòng

API mode = harness lái tool loop + structured JSON contract; CLI mode = giao việc cho agent tự trị + file/diff contract. Điểm cần hài hoà duy nhất là **platform-only knowledge** (skills content, learned_skills, task rules) — vì đây là thứ CHỈ mình platform biết mà CLI dù có full shell cũng không thấy được; mọi thứ khác (git diff, conventions, tool list) nên để nguyên, không port, vì CLI tự làm được và port thêm chỉ gây lãng phí/trùng lặp.
