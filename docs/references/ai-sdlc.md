# Phân Tích Kiến Trúc Hệ Thống AI-SDLC

## 1. Source Snapshot
- **Source Repository:** `resources/ai-sdlc`
- **Reviewed Paths:** `orchestrator/src/`, `mcp-advisor/`, `README.md`
- **Snapshot Date:** 2026-07-06
- **Scope:** Phân tích mã nguồn lõi của Orchestrator, cơ chế quản lý trạng thái, và các pipeline điều phối AI.
- **Evidence Confidence:** High (Phân tích trực tiếp từ mã nguồn thực tế).
- **Adoption Recommendation Confidence:** Medium (Đang trong giai đoạn đánh giá cho Auto Code OS).
- **License:** Apache 2.0
- **Status:** Observed (Phân tích từ mã nguồn gốc) / Proposed (Một số cơ chế đề xuất áp dụng cho Auto Code OS)

## 2. Mô Hình Lý Thuyết vs Pipeline Thực Thi

### Conceptual Lifecycle (Vòng Đời 7 Bước)
Vòng đời ý niệm của AI-SDLC xoay quanh quy trình phát triển phần mềm cơ bản:
1. **WATCH:** Giám sát tín hiệu.
2. **TRIAGE:** Phân loại yêu cầu.
3. **PLAN:** Lập kế hoạch.
4. **BUILD:** Thực thi (Code).
5. **VERIFY:** Kiểm thử & Đánh giá.
6. **DEPLOY:** Triển khai.
7. **LEARN:** Rút kinh nghiệm.

### Actual Execution Pipeline (Step 0-13)
Dựa trên kiến trúc mã nguồn (`pipeline-cli/docs/steps.md`), pipeline thực tế của hệ thống là một chuỗi 14 bước khắt khe. Dưới đây là diễn giải mức cao (high-level summary) của luồng thực thi:
- **Khởi tạo & Chuẩn bị (Step 0-4):** Quét dọn các merged worktrees (Step 0), Validate task (Step 1), Compute branch (Step 2), Setup worktree (Step 3), và Bắt đầu task (Step 4).
- **Phân rã & Thực thi (Step 5-7):** Phân rã Task (Task Decomposer), nạp JIT Credentials và chạy Dev Agent để thực thi mã nguồn (bao gồm cả Triage rủi ro).
- **Đánh giá (Step 8-10):** Chạy song song 3 Agent đánh giá (Cross-Harness Reviewers: Code, Test, Security).
- **Hoàn thiện (Step 11-13):** Ký số chứng thực (Attestation Sign), mở Pull Request tự động, dọn dẹp Worktree và giải phóng tài nguyên.

## 3. Phân Tích Kỹ Thuật (14 Hệ Thống Phụ)

Dưới đây là 14 hệ thống phụ (sub-systems) cốt lõi thể hiện rõ kiến trúc điều phối của AI-SDLC:

1. **Worktree Isolation (`WorktreePoolManager`):** Quản lý các luồng công việc song song trên Git Worktree, ngăn chặn xung đột mã nguồn khi nhiều Agent hoạt động cùng lúc.
2. **Attestation Verifier (`attestations.ts`):** Cơ chế mã hóa DSSE envelope để lưu trữ chứng nhận review, giúp CI bỏ qua các bước kiểm thử trùng lặp một cách an toàn.
3. **Claude Code Runner:** Chụp snapshot môi trường trước khi khởi chạy Agent, giúp phân tách chính xác các tệp do AI tạo ra so với các tệp tạm thời hiện có.
4. **Harness Independence:** Cơ chế cô lập mô hình kiểm thử, đảm bảo mô hình thực hiện đánh giá (Reviewer) bắt buộc phải khác loại với mô hình đã sinh ra mã nguồn.
5. **Task Decomposer (`task-decomposer.ts`):** Phân rã task bằng đồ thị có hướng (DAG) theo ranh giới module hoặc mối quan tâm (Concern) bằng hệ thống luật đơn định, không phụ thuộc vào LLM.
6. **Design System Correction Loop (`design-system-correction-loop.ts`):** Vòng lặp phản hồi hình ảnh (Visual Regression), kiểm soát ranh giới sửa đổi bằng các biến `maxRetries` và `costSoftLimitUsd`.
7. **Journey Router (`journey-sa2-router.ts`):** Định tuyến động dựa trên ngữ cảnh, điều chỉnh chuẩn Accessibility (WCAG) và tính đồng nhất UI tùy thuộc vào phân luồng người dùng (User Journey).
8. **Tessellation Drift (`tessellation-drift.ts`):** Quét cây AST ở các thư viện lõi (Substrate) nhằm phát hiện sớm các logic nghiệp vụ bị hard-code sai quy tắc kiến trúc.
9. **Process Escalation (`process-escalation.ts`):** Tự động điều chỉnh mức độ nghiêm ngặt của CI/CD (nới lỏng hoặc chèn thêm Security Scan) dựa trên chỉ số phức tạp của công việc.
10. **Cost Governance (`cost-governance.ts`):** Hệ thống kiểm soát chi phí API qua các ngưỡng Soft Limit (cảnh báo) và Hard Limit (ngắt quy trình hoặc yêu cầu phê duyệt).
11. **Design Authority (`design-authority.ts`):** Phân quyền nghiêm ngặt, chỉ cho phép một nhóm người dùng (Design Authority Principals) được quyền gắn nhãn ưu tiên về thiết kế.
12. **Audit SQLite Sink (`audit-sqlite-sink.ts`):** Lưu vết toàn bộ thay đổi thông qua cơ chế băm dữ liệu nối tiếp (hash chain) để đảm bảo tính toàn vẹn của Audit Trail.
13. **Asymmetric Triage (`triage.ts`):** Cổng kiểm duyệt bảo mật tự động chặn các yêu cầu rủi ro cao (Prompt Injection) nhưng bắt buộc yêu cầu phê duyệt thủ công (Human-in-the-loop) cho các yêu cầu được đánh giá là an toàn.
14. **Prompt Engineering Architecture (`appendSystemPrompt`):** Thiết kế tách bạch phần lõi của Prompt (Lưu bằng tệp `.md` để dễ đọc, quản lý phiên bản) với phần cấu hình động (Tiêm siêu dữ liệu từ `.json`/`.yml` vào cuối thông qua hàm `appendSystemPrompt` - *đã xác minh qua `claude-code-sdk.ts`*). Giúp dễ dàng bảo trì kịch bản của AI mà không cần compile lại source code.

*(Lưu ý: Các module như `priority.ts`, `handoff-executor.ts`, `pipeline-cycle-detector.ts`, và `rollout-controller.ts` cũng đóng vai trò vệ tinh quan trọng bổ trợ cho 14 hệ thống trên).*

## 4. Bảng Đánh Giá Áp Dụng Cho Auto Code OS (Adoption Matrix)

| Cấu trúc / Tính năng | Giá trị (Value) | Nỗ lực (Effort) | Rủi ro (Risk) | Phụ thuộc (Dependencies) | Trạng thái (Adopt) |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **Task Decomposer (DAG)** | Rất cao | Trung bình | Thấp | YAML/Markdown Parser | **Adopt Now** (MVP only) |
| **Worktree Isolation** | Cao | Trung bình | Trung bình | Git CLI | **Adopt Now** |
| **Handoff Executor (Schema)** | Cao | Thấp | Thấp | JSON Schema Validator | **Adopt Now** |
| **Pipeline Cycle Detector** | Cao | Thấp | Thấp | File State/Local DB | **Adopt Now** |
| **Cost Governance** | Cao | Thấp | Thấp | Token Tracker module | **Adopt Now** |
| **Asymmetric Triage** | Trung bình | Trung bình | Cao | LLM phân tích rủi ro | **Adopt Later** |
| **Process Escalation** | Trung bình | Cao | Trung bình | CI/CD linh hoạt | **Adopt Later** |
| **Tessellation Drift** | Thấp | Cao | Thấp | AST Parser | **Skip** |
| **Progressive Rollout** | Thấp | Rất Cao | Rất Cao | DevOps / Cloud Infra | **Skip** |
| **Attestation Verifier** | Thấp | Cao | Thấp | Cryptographic Signatures | **Skip** |

## 5. Rào Cản An Toàn (Guardrails) & Các Hạn Chế Khi Triển Khai

Để đảm bảo tính an toàn cho Auto Code OS, các cơ chế đề xuất từ AI-SDLC phải đi kèm các rào cản (guardrails) nghiêm ngặt sau:

1. **Cấp & Thu Hồi Quyền Tức Thời (JIT Agent Credentials):**
   *   **Token Scope:** Token chỉ được cấp với quyền hạn tối thiểu (chỉ đọc/ghi đúng repo hiện tại, không có quyền xóa repo).
   *   **TTL (Time-To-Live):** Giới hạn thời gian sống nghiêm ngặt (ví dụ: 15 phút).
   *   **Revoke Failure Behavior:** Nếu quá trình `revoke` thất bại, hệ thống cần tự động khóa các process đang chạy của Agent và cảnh báo người dùng khẩn cấp.

2. **Cơ Chế Override (Human-in-the-Loop):**
   *   **Role Constraint:** Chỉ định rõ role hoặc user ID nào được phép kích hoạt nút Override. 
   *   **Hard Gates:** Các cổng kiểm tra bảo mật (Security Scan) hoặc lỗi biên dịch (Syntax Error) không cho phép override dưới bất kỳ hình thức nào.
   *   **Audit Schema:** Mọi thao tác override phải kèm theo lý do bắt buộc và được ghi vào cơ sở dữ liệu lưu vết (Audit Trail).

3. **Điều Khiển Triển Khai (Deploy Controller):**
   *   **Approval Boundary:** Hệ thống AI **không được phép trực tiếp deploy lên Production**. Mọi luồng Rollout Controller chỉ được phép đẩy code lên môi trường cục bộ (Local) hoặc Staging. Việc đưa lên môi trường thực tế phải thông qua phê duyệt thủ công của kỹ sư (Approval Boundary).

## 6. Những Cơ Chế Không Phù Hợp (Not Adopted / Future Enterprise)

AI-SDLC chứa một số cơ chế mang tính "Enterprise-heavy" (phục vụ doanh nghiệp quy mô lớn), không phù hợp hoặc quá phức tạp đối với mục tiêu cốt lõi của Auto Code OS hiện tại:

- **Attestation & Cryptographic Signatures (DSSE):** Cấu trúc mã hóa và ký số chéo giữa các AI rất tốn kém tài nguyên và chỉ phù hợp với các tổ chức yêu cầu Compliance cao. Việc này là "Over-engineering" đối với một công cụ hỗ trợ code cục bộ.
- **Rollout Controller:** Tính năng Canary Release tự động đòi hỏi tích hợp chặt chẽ với hạ tầng Cloud (Vercel, K8s). Điều này đi lệch khỏi phạm vi của Auto Code OS vốn chỉ tập trung vào việc tạo mã nguồn và tự động hóa IDE.
- **PPA Priority Formula:** Thuật toán tính toán ưu tiên phức tạp chỉ phù hợp cho Product Manager quản lý Backlog lớn, không mang lại nhiều giá trị khi User chỉ giao 1-2 task tại một thời điểm.
- **Tessellation Drift (AST Scanning lõi):** Việc phân tích AST liên tục để chống rò rỉ nghiệp vụ (coupling) rất tốn kém CPU, làm chậm IDE, không phù hợp để chạy thời gian thực trên máy tính cá nhân của lập trình viên.

## 7. Chiến Lược Quản Lý Ngữ Cảnh (Context Management) Cho Auto Code OS

Để giải quyết triệt để bài toán tràn RAM và tăng vọt chi phí Token khi AI phải đọc toàn bộ dự án, Auto Code OS sẽ áp dụng chiến lược **Hybrid** kết hợp giữa **Option C (Agent tự dò đường)** và một phần của **Option A (Phân tích đồ thị code tĩnh - Đề xuất thêm)**:

*   **Về kiến trúc thiết kế:** Một OS (Hệ điều hành) nên đóng vai trò cung cấp System Tools (như Grep, Find, File Viewer) thay vì cố gắng đoán trước AI cần gì.
*   **Chiến lược triển khai:**
    *   **Giai đoạn 1 (Áp dụng ngay - Option C):** Cấp cho Agent bộ Tool để tự tìm code. Bắt đầu với 0 ngữ cảnh, AI tự suy luận và tự tối ưu token bằng cách chỉ gọi tool để đọc chính xác những file nó thấy cần thiết.
    *   **Giai đoạn 2 (Tương lai - Option A):** Khi Auto Code OS lớn mạnh, xây dựng tính năng chụp Đồ thị phụ thuộc code (Code Dependency Graph). Khi Agent gõ lệnh xin sửa một file lõi, OS sẽ tự động đính kèm thêm các file Interface/Type liên quan.

*Lưu ý: Sự thật là dự án AI-SDLC gốc hoàn toàn không sử dụng Code Parser để nạp ngữ cảnh như lầm tưởng ban đầu. Họ giải quyết bài toán chống tràn RAM bằng cách sử dụng **Task Dependency Graph** (Đồ thị phụ thuộc đầu việc) quản lý qua file YAML - ép con người chia nhỏ task, AI chỉ biết 1 task duy nhất tại một thời điểm. Tuy nhiên, việc Auto Code OS tự cấp Tool cho Agent (Option C) lại đi sát với bản chất của một Hệ Điều Hành tự chủ nhất.*
