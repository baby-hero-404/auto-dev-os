# Báo Cáo Phân Tích — Multica

## Tổng Quan (TL;DR)
Multica biến các AI coding agent thành những "đồng nghiệp ảo" thật sự trong nhóm làm việc — mỗi agent có hồ sơ, ảnh đại diện riêng, có thể được gắn thẻ vào công việc, tham gia nhóm nhỏ, và tự báo cáo tiến độ y như một lập trình viên thật, thay vì chỉ là một con bot vô danh chạy ngầm.

## Tổng Quan (Kỹ Thuật)
Nền tảng "Managed agents" mã nguồn mở — biến coding agents thành teammates thực sự. Trong Multica, agent không chỉ là bot ẩn danh mà là những thành viên thực thụ: có profile, avatar, có thể được tag (@mention) vào issue, tham gia squad, tự báo cáo blockers, và cập nhật timeline y như developer người thật.
Stack: TypeScript monorepo (pnpm + Turborepo), Next.js cho web, Electron cho desktop, React Native cho mobile, và backend sử dụng **Convex** để đạt được realtime sync (WebSocket) out-of-the-box mà không cần quản lý kết nối.

## Tính Năng Nổi Bật (Best Features)
1. **Agents-as-Teammates Architecture**
   - *Là gì:* Agent được đối xử như một thành viên nhóm thực sự chứ không phải một tiện ích ẩn — nó có thể nhận việc, báo "đang làm", báo tiến độ, và báo hoàn thành/thất bại, y hệt quy trình một người thật cập nhật công việc của mình.
   - *Cách triển khai:* Một agent có thể claim task, bắt đầu chạy (start), cập nhật tiến trình (progress), và hoàn thành/thất bại (complete/fail). Mọi state transition này được stream real-time lên frontend qua WebSocket, hiển thị trên bảng Kanban và Timeline.
2. **Squads (Phân Quyền Phân Cấp)**
   - *Là gì:* Agent và người thật có thể được gom vào chung một "nhóm nhỏ" (squad), và khi giao việc cho cả nhóm, hệ thống tự biết chuyển việc tới đúng người/agent phù hợp trong nhóm đó — giống như giao việc cho một trưởng nhóm rồi để họ tự phân công.
   - *Cách triển khai:* Khi user giao việc cho `@FrontendTeam`, hệ thống sẽ delegate (ủy quyền) cho leader của team phân phối cho thành viên phù hợp. Đảm bảo routing ổn định khi team mở rộng.
3. **Autopilots**
   - *Là gì:* Một số agent có thể tự động làm việc theo lịch định kỳ mà không cần ai nhắc, ví dụ tự tạo báo cáo đứng hàng ngày hoặc tự rà soát chất lượng code định kỳ.
   - *Cách triển khai:* Các agent tự động làm việc định kỳ (cron triggers) để tạo issue và phân công tự động (ví dụ: daily standup, code audit).
4. **Comment-Triggered Action**
   - *Là gì:* Chỉ cần nhắc tên một agent trong phần bình luận của một công việc, hệ thống sẽ tự động kích hoạt agent đó xử lý — và luôn phản hồi ngay cho người dùng biết yêu cầu đang ở trạng thái nào (đang chờ, bị hoãn, hay bị chặn) thay vì im lặng khó hiểu.
   - *Cách triển khai:* Tương tác qua comment có thể trigger action của agent. Tag `@Agent` vào một issue sẽ đẩy action queue, hệ thống trả về outcome (`queued`, `deferred`, `blocked`) để UX phản hồi ngay lập tức.

## Áp Dụng Cho Auto Code OS (Applied Takeaways — ranked)
1. **Kiến trúc Task Surface phân tách Query Planning**
   - **What**: Đóng gói query logic (workspace, project, assigned, created) thành các "Surface plan" tách biệt khỏi component UI (như `packages/core/issues/surface/query-plan.ts`).
   - **Apply**: Refactor `server/internal/service/` của Auto Code OS để tách filter/query logic. Tạo package `task_surface` giúp quản lý data theo context (my tasks, all tasks, active tasks) một cách an toàn và dễ cache.
   - **Impact**: H · **Effort**: M · **Risk**: L · **Est**: 3 days.
2. **WebSocket Lifecycle Streaming**
   - **What**: Toàn bộ lifecycle events (enqueue → claim → progress) stream real-time qua WebSocket, với stable-sorting tại client để chống out-of-order.
   - **Apply**: Thêm structured WS events cho từng DAG step (`context_loading`, `analyzing`, `coding`) trong orchestrator. Frontend nhận push updates thay vì polling `/api/tasks/{id}` liên tục.
   - **Impact**: H · **Effort**: M · **Risk**: L · **Est**: 2-3 days.
3. **Comment-Triggered Workflows**
   - **What**: Tương tác qua comment có thể trigger / pause / resume action của agent (`comment-trigger-outcomes.ts`).
   - **Apply**: Bổ sung comment system cho tasks trong Auto Code OS. Agent báo blocker bằng comment làm pause DAG. User comment approve hoặc trả lời blocker sẽ tự động resume DAG.
   - **Impact**: M · **Effort**: M · **Risk**: L · **Est**: 3-4 days.
4. **CLI-Agent-as-Backend thay vì gọi LLM API trực tiếp trong process** *(nghiên cứu bổ sung 2026-07-20, theo yêu cầu)*
   - **What**: Multica **không tự triển khai tool-loop LLM API** ở backend. Thay vào đó, mỗi user/máy chạy 1 **daemon** cục bộ (`server/internal/daemon/`) probe PATH để tìm CLI coding agent đã cài sẵn — danh sách cố định `defaultAgentCommandNames` (`config.go:850-853`): `claude, codex, opencode, deveco, openclaw, hermes, pi, cursor-agent, copilot, kimi, kiro-cli, codebuddy, agy, traecli, grok`. Với mỗi tool, `probe("MULTICA_<X>_PATH", "<cmd>", "MULTICA_<X>_MODEL")` (`config.go:278-311`) thử `exec.LookPath`, cho phép override qua env var, và có xử lý đặc thù (vd Codex Desktop bundle path trên macOS, `codexDesktopAppBundlePaths()`). Daemon giữ kết nối WebSocket RPC bền (`wsrpc.go`) tới server Convex để nhận task dispatch, claim task, rồi **spawn CLI agent đã cài như 1 subprocess** (`exec.CommandContext(ctx, bin, args...)`, pattern xác nhận tại `execenv/openclaw_config.go:762-777` cho `openclaw` CLI — cùng cơ chế áp dụng cho các agent khác) trong 1 git worktree cô lập (`execenv/git.go`: `worktree add -b <branch>`), stream log/kết quả về qua WS RPC. Nói cách khác: **server không hề gọi Anthropic/OpenAI API trực tiếp** — nó chỉ điều phối, còn "trí tuệ" nằm hoàn toàn ở CLI tool người dùng tự cài (Claude Code, Codex CLI, Cursor Agent...), daemon chỉ là orchestration layer + process supervisor + git worktree manager.
   - **So sánh với Auto Code OS hiện tại**: **Đã verify — kiến trúc ngược hoàn toàn**. `grep -rn "exec.Command" server/internal/orchestrator/llmrunner server/internal/sandbox` chỉ có 1 kết quả không liên quan (`docker.go:113`, chạy `go env GOPATH`); `grep -rn "\"claude\"\|\"codex\"\|cursor-agent\|LookPath" server/internal` **rỗng**. Auto Code OS hiện tự triển khai toàn bộ tool-loop trong Go (`server/internal/orchestrator/llmrunner/toolloop.go`) gọi thẳng LLM API qua `server/pkg/llm/` (Anthropic/OpenAI/Gemini provider tự viết), không hề có khái niệm "probe CLI agent đã cài" hay "spawn subprocess CLI coding tool". Sandbox (`server/internal/sandbox/docker.go`) chỉ dùng để chạy build/test/lint trong container, không chạy agent CLI.
   - **Đánh đổi nếu chuyển sang mô hình CLI-agent-as-backend**:
     - *Được*: (1) không cần tự maintain tool-loop/prompt engineering/patch-apply logic — thừa hưởng chất lượng & update liên tục của Claude Code/Codex CLI chính chủ; (2) dễ multi-model/multi-vendor (user tự chọn agent nào cài trên máy); (3) chi phí API do user tự trả qua subscription CLI (Claude Pro/Codex plan) thay vì Auto Code OS phải quản lý API key + billing tập trung.
     - *Mất*: (1) mất toàn quyền kiểm soát prompt/tool-definition/review-gate hiện có (`server/internal/prompts/`, `server/internal/orchestrator/patch/`, learning engine, memory injection) — các CLI agent đóng hộp không cho customize sâu quy trình DAG (DoR gate, cross-harness review, learning loop) như Auto Code OS đang xây; (2) yêu cầu user phải cài đặt + xác thực CLI tool cục bộ, tăng ma sát onboarding so với "chỉ cần API key" hiện tại; (3) khó chạy hoàn toàn server-side/headless cho SaaS multi-tenant nếu không có máy/daemon riêng cho từng user; (4) quan sát/observability (chi phí, token, log chi tiết) phụ thuộc vào CLI tool có expose đủ hay không, thay vì tự đo được như hiện tại qua `server/pkg/llm/pricing.go`.
   - **Khuyến nghị áp dụng có chọn lọc** (không thay thế hoàn toàn, vì đánh đổi #1 và #3 quá lớn so với roadmap học/review/memory hiện có của Auto Code OS): thêm 1 **provider type mới** trong `server/pkg/llm/` — `CLIAgentProvider` implement cùng interface `Provider` hiện có (xem `anthropic.go`), nhưng thay vì gọi HTTP API thì `exec.CommandContext` ra CLI agent cài sẵn trong sandbox container (`server/internal/sandbox/docker.go`) với flag headless/non-interactive (`claude -p "<prompt>" --output-format json` hoặc tương đương của từng tool), parse output JSON trả về `*Response` chuẩn. Cho phép chọn theo task (`task.Provider = "cli:claude"` vs `"api:anthropic"`) — hữu ích cho case muốn tận dụng subscription phẳng (Claude Pro/Max) thay vì pay-per-token API cho các step ít nhạy cảm (vd `analyzing`, không phải `coding`/`review` nơi cần kiểm soát chặt). Cần đưa binary CLI vào Docker image sandbox trước, và audit license/ToS của từng CLI tool cho phép chạy tự động không tương tác.
   - **Impact**: M (mở rộng lựa chọn, không phải bắt buộc) · **Effort**: H (cần build provider mới + đóng gói CLI vào sandbox image + xử lý auth persist) · **Risk**: M (phụ thuộc hành vi/ToS của CLI bên thứ 3, khó test hermetic) · **Est**: 1-2 tuần cho PoC 1 CLI agent (khuyến nghị bắt đầu với `claude -p`).

## Kiến Trúc (Architecture)
- **Monorepo (Turborepo + pnpm)**: Tối ưu build cache và chạy song song. Phân chia rạch ròi `apps/` và `packages/`.
- **Shared Core Logic**: Business logic nằm ở `packages/core`, UI components dùng chung ở `packages/ui`. Mọi nền tảng (Web/Desktop/Mobile) chỉ là "lớp vỏ" render dữ liệu từ core.
- **Backend (Convex)**: Thay thế cả Database và API layer. Convex cung cấp query/mutation chạy trên server, tự động reactive và push state xuống client qua WebSocket.
- **Timeline Engine**: Xử lý event sourcing cho vòng đời issue. Các kiện (như assigned, unassigned, comment) đều đẩy vào một timeline cache chung.

### ADR Suy Luận (Inferred ADRs)
| Quyết Định | Bằng Chứng | Lợi Ích | Đánh Đổi | Confidence |
|---|---|---|---|---|
| Dùng Convex | Convex SDK imports ở `core` | Real-time sync cực nhanh, không cần setup WS server tự chế | Bị lock-in vào hệ sinh thái Convex (Platform risk) | High |
| Turborepo | `turbo.json` | Caching tốt, local DX nhanh khi build đa nền tảng | Hơi cồng kềnh với developer chưa quen monorepo | High |
| Feature-based Folders | `packages/core/issues/` | Domain logic gom cụm gọn gàng (queries/mutations/types ở chung) | Cần structure rõ ràng tránh circular deps | High |

## Luồng Chính (Main Flow)
```mermaid
flowchart TD
    A[Task Created] --> B[Assignee Picker (Agent/Human)]
    B --> C{Là Agent hay Squad?}
    C -- Squad --> D[Leader Agent nhận task]
    D --> E[Leader Delegate cho Member (Agent/Human)]
    C -- Agent --> F[Agent Claim Task]
    E --> F
    F --> G[Start Execution]
    G --> H[Stream Progress qua WS (Convex)]
    H --> I{Thành công?}
    I -- Yes --> J[Complete Status]
    I -- No --> K[Báo Blocker qua Comment / Fallback]
```

### 🔬 Deep Dive: Cơ Chế Spawn CLI Đa Tài Khoản (Multi-Account) — Xác Nhận Bằng Code

**Có hỗ trợ multi-account, ở cấp "1 task = 1 credential set" chứ không phải "1 daemon = 1 account".** Bằng chứng:

1. **`AgentData.CustomEnv`** (`server/internal/daemon/types.go:174`): mỗi "Agent" record (config đại diện 1 tài khoản/persona, tạo qua UI) có riêng field `custom_env map[string]string` — chứa các biến như `ANTHROPIC_API_KEY`, `ANTHROPIC_BASE_URL`, `CLAUDE_CODE_USE_BEDROCK`. Đây là dữ liệu lưu ở tầng server/Convex, gắn với 1 Agent, không phải env của daemon process.

2. **Per-task merge, không phải per-daemon**: `daemon.go:4258-4262` — mỗi lần xử lý 1 task, code đọc `task.Agent.CustomEnv` (credential của agent được gán cho task đó) rồi gọi `layerCustomEnvAndHermesHome(agentEnv, agentCustomEnv, ...)` (`daemon.go:5375`) để merge đè lên `agentEnv` (base env đã build từ `os.Environ()` + các biến hệ thống của daemon) — **trước khi** `agent.New(provider, agent.Config{Env: agentEnv, TaskID: task.ID, ...})` (`daemon.go:4263`) tạo `Backend` mới. Mỗi `Backend`/subprocess được tạo **mới hoàn toàn cho từng task** (`TaskID` là field bắt buộc trong `Config`), nên 2 task chạy đồng thời trên cùng 1 daemon với 2 Agent khác nhau sẽ có 2 subprocess với `cmd.Env` khác nhau — 1 daemon Go process **có thể** phục vụ nhiều tài khoản song song, miễn là task tới được gán đúng `Agent.CustomEnv`.

3. **Blocklist bảo vệ biến nội bộ** (`isBlockedEnvKey`, `daemon.go:5321-5331`): các key `HOME`, `PATH`, `CODEX_HOME`, `CURSOR_DATA_DIR`, `OPENCLAW_CONFIG_PATH`, `OPENCLAW_INCLUDE_ROOTS`, và mọi biến prefix `MULTICA_` bị chặn — user không thể qua `CustomEnv` ghi đè các biến daemon tự set (vd đường dẫn state per-task). Điều này ngăn 1 Agent config độc hại/lỗi làm hỏng cô lập worktree của task khác.

4. **Cô lập state/session per-task, không chỉ credential**: `CODEX_HOME`, `CURSOR_DATA_DIR`, `OPENCLAW_CONFIG_PATH` được daemon tự set **per-task** (không lấy từ `CustomEnv` mà build động dựa trên `env.WorkDir`/`env.HermesHome`, dòng 4230-4251) — nghĩa là ngoài credential khác nhau, mỗi task còn có thư mục state/config CLI riêng, tránh 2 task cùng account nhưng khác task ghi đè session của nhau.

**Kết luận cho Auto Code OS**: nếu sau này muốn thêm lựa chọn "outsource" tool-loop cho CLI agent người dùng cài (theo mô hình subprocess-CLI, xem bảng so sánh trong `README.md`), nên copy đúng pattern 3 lớp này — (a) credential ở tầng data model gắn với 1 "Agent"/"Account" record, (b) merge env **tại thời điểm spawn subprocess cho từng task** chứ không set 1 lần ở daemon startup, (c) blocklist rõ ràng các biến nội bộ để user-provided env không đụng vào cơ chế cô lập của orchestrator.

## Design Patterns & Chất Lượng Code
- **Query/Mutation Pattern**: Phân tách hoàn toàn thao tác đọc (`queries.ts`) và ghi (`mutations.ts`) theo đúng chuẩn CQRS nhưng ở scale frontend/backend boundary.
- **Surface Pattern**: Khái niệm "Surface" như một lớp composable filter quyết định dữ liệu nào hiển thị cho user nào (`surface/query-plan.ts`).
- **Cache Invalidation Explicitness**: Có helper riêng như `cache-helpers.ts` để dọn dẹp bộ nhớ đệm thủ công khi cần, bù trừ cho reactive system đôi khi lưu state cũ.

## Kỹ Thuật Thú Vị & Thực Hành Kỹ Thuật
- **Phân loại Issue bằng Status & Priority Configs**: Config cứng cho các trạng thái chuẩn, dễ dàng đồng bộ UI across apps (`packages/core/issues/config/status.ts`).
- **Defensive Parsing cho Outcomes**: Khi tag 1 Agent vào comment, parse output từ server cực kỳ phòng thủ (`comment-trigger-outcomes.test.ts`), loại bỏ malformed data và có fallback text recover từ markdown mention.

## Engineering Gems
1. `timeline-sort.ts` (`sortTimelineEntriesAsc`): Vấn đề kinh điển của WebSocket là event bay về client out-of-order do network race conditions. Ở đây, họ viết một stable-sort thuần túy dựa trên `created_at` với `id` fallback, ép mọi writer chạy qua hàm này trước khi update UI, đảm bảo Timeline trên màn hình luôn tuyến tính không bị giật lùi.
2. `comment-trigger-outcomes.ts`: Thay vì "gửi tin nhắn rồi cầu nguyện agent đọc", hệ thống có outcome tracking (`queued`, `deferred`, `blocked`, `coalesced`). Nếu agent bị block không thể reply (ví dụ policy cấm), UI sẽ hiện toast báo cho user "Agent X is currently blocked". Điều này giải quyết "lỗ đen UI" khi tương tác với AI.
3. `surface/query-plan.ts`: Biến mọi filter (my tasks, all team, assignee types) thành một QueryPlan object có cấu trúc, sau đó compiler/resolver ở server mới đọc Plan này để build query SQL/Convex. Rất clean, testable không cần DB.

## Câu Hỏi Đáng Suy Ngẫm
- **Bẫy "Platform Lock-in"**: Việc dùng Convex giúp Multica đi siêu nhanh, nhưng nếu Auto Code OS muốn on-premise hoàn toàn (self-hosted enterprise), liệu ta có thể xây dựng một WS/SSE layer trên Postgres đủ mượt mà không?
- **Squad Delegation**: Giao task cho một group agents (Squad) liệu có dẫn đến "diffusion of responsibility" (đẩy qua đẩy lại), cần một orchestrator đủ cứng ở giữa để assign cứng?

## Top 10 Điều Đáng Học
| # | Khái Niệm | File | Vì Sao Hữu Ích | Độ Khó | Thứ Tự |
|---|---|---|---|---|---|
| 1 | Lifecycle WebSocket Streaming | `packages/core/issues/timeline-sort.ts` | Push updates mượt mà, UX xịn xò | ⭐⭐⭐ | 1 |
| 2 | Surface Data Filtering | `packages/core/issues/surface/query-plan.ts` | Cô lập logic phân quyền & scope query | ⭐⭐⭐⭐ | 2 |
| 3 | Comment Trigger Outcomes | `packages/core/issues/comment-trigger-outcomes.ts` | Tránh lỗ đen giao tiếp với AI | ⭐⭐⭐ | 3 |
| 4 | Explicit CQRS Pattern | `packages/core/issues/queries.ts` & `mutations.ts` | Maintainability cao ở scale lớn | ⭐⭐ | 4 |

## Hướng Dẫn Đọc (Reading Guide)
**L0 Khởi động:** `README.md` để hiểu scope.
**L1 Core Concepts:** Vào `packages/core/issues/` đọc `queries.ts` và `mutations.ts`.
**L2 UI Data Layer:** Khám phá `packages/core/issues/surface/query-plan.ts` để xem cách tổ chức state filter cho frontend.
**L3 Realtime Sync:** Đọc `timeline-sort.ts` để hiểu triết lý giải quyết WebSocket race conditions.
**L4 Edge Cases:** Xem `comment-trigger-outcomes.test.ts` để thấy độ kĩ tính khi handle agent feedback.

## Anti-Patterns & Không Nên Copy
1. **Coupling chặt vào Backend-as-a-Service (Convex)**: Phụ thuộc toàn bộ data model vào SDK của Convex. Trong kiến trúc Go của Auto Code OS, ta cần giữ Postgres làm source of truth và viết layer SSE/WS riêng.
2. **Thiếu Error Taxonomy Rõ Ràng**: Các package ít định nghĩa custom Errors, dẫn đến đôi khi quăng Error chung chung, gây khó khăn cho observability ở production.

## Đánh Giá Tổng Thể
| Architecture | Maintainability | Scalability | Clean Code | Learning Value |
|---|---|---|---|---|
| 8/10 | 8/10 | 7/10 | 8/10 | 9/10 |

## Lộ Trình Học Tập
- **Tuần 1**: Đọc `packages/core/issues/queries.ts` và `mutations.ts` để nắm CQRS pattern; chạy thử Convex dev server local để quan sát reactive query thực tế thay đổi khi mutation chạy.
- **Tuần 2**: Đọc `packages/core/issues/surface/query-plan.ts` để hiểu cách cô lập logic phân quyền & scope query khỏi UI layer; đối chiếu với cách Auto Code OS hiện lọc dữ liệu ở `server/internal/handler/`.
- **Tuần 3**: Đọc `timeline-sort.ts` và `comment-trigger-outcomes.ts` (+ file test `.test.ts` đi kèm) để hiểu triết lý giải quyết WebSocket race condition và cách tránh "lỗ đen" giao tiếp với AI agent.
- **Tuần 4**: Prototype 1 event nhỏ theo mô hình lifecycle streaming (enqueue → claim → start → progress → complete/fail) trên kênh WebSocket hiện có của Auto Code OS, đo độ trễ UI so với polling baseline.
