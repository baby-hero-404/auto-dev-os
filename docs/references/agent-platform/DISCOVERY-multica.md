# Báo Cáo Phân Tích — Multica

## Tổng Quan
Nền tảng "Managed agents" mã nguồn mở — biến coding agents thành teammates thực sự. Trong Multica, agent không chỉ là bot ẩn danh mà là những thành viên thực thụ: có profile, avatar, có thể được tag (@mention) vào issue, tham gia squad, tự báo cáo blockers, và cập nhật timeline y như developer người thật.
Stack: TypeScript monorepo (pnpm + Turborepo), Next.js cho web, Electron cho desktop, React Native cho mobile, và backend sử dụng **Convex** để đạt được realtime sync (WebSocket) out-of-the-box mà không cần quản lý kết nối.

## Tính Năng Nổi Bật (Best Features)
1. **Agents-as-Teammates Architecture**: Agent tồn tại ở tầm vóc "thành viên nhóm". Một agent có thể claim task, bắt đầu chạy (start), cập nhật tiến trình (progress), và hoàn thành/thất bại (complete/fail). Mọi state transition này được stream real-time lên frontend qua WebSocket, hiển thị trên bảng Kanban và Timeline.
2. **Squads (Phân Quyền Phân Cấp)**: Agent và Human có thể gom chung vào một "Squad" (nhóm). Khi user giao việc cho `@FrontendTeam`, hệ thống sẽ delegate (ủy quyền) cho leader của team phân phối cho thành viên phù hợp. Đảm bảo routing ổn định khi team mở rộng.
3. **Autopilots**: Các agent tự động làm việc định kỳ (cron triggers) để tạo issue và phân công tự động (ví dụ: daily standup, code audit).
4. **Comment-Triggered Action**: Tương tác qua comment có thể trigger action của agent. Tag `@Agent` vào một issue sẽ đẩy action queue, hệ thống trả về outcome (`queued`, `deferred`, `blocked`) để UX phản hồi ngay lập tức.

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
