---
sources:
  - "server/**"
  - "server/pkg/models/learned_skill.go"
verified: 2026-07-23
---

# 04. Agent System (Role-Based Capability Agents)

**Status:** 🟡 In Progress (baseline implemented; hardening planned)  
**Owner docs:** `docs/features/product/04-agent-system.md` (this file); `docs/ARCHITECTURE.md`  
**Code areas:** `server/pkg/models/agent.go`, `server/internal/repository/agent.go`, `server/internal/service/agent.go`, `server/internal/orchestrator`, `web/src/` agent CRUD UI  
**Blocking decisions:** Whether migration drops legacy agent rows or backfills; default role template set.  
**Acceptance criteria:** Agent config uses Role, Goal, Context Config, Autonomy Level, and Model Level Group. (Baseline) Orchestrator filters tools based on agent roles. (Target) Orchestrator only exposes tools from skills dynamically loaded by Agent Planner via JIT Loading.

**Mục tiêu:** Mỗi AI Agent là một chuyên gia với vai trò rõ ràng (backend, frontend, reviewer, QA...) — giống như một đội ngũ dev thực sự. Orchestrator phối hợp nhiều Agent cùng làm việc trên một task phức tạp.

> **Tham khảo chính:** CrewAI (Role/Goal/Backstory), AutoGen (Actor Model — Handoff, Fan-out, Group Chat), AI-SDLC Framework (Cross-Harness Review, Autonomy Tracker).

---

## Tại Sao Cần Agent System?

Một LLM duy nhất không thể giỏi mọi thứ. Bằng cách chia thành nhiều Agent chuyên biệt, hệ thống đạt được:

- **Chuyên môn hóa:** Backend Agent tập trung vào server logic, Frontend Agent chuyên UI, Reviewer Agent kiểm tra chất lượng.
- **Kiểm soát chi phí:** Task đơn giản dùng model Fast (rẻ), task phức tạp mới dùng model Powerful (đắt).
- **Chất lượng cao hơn:** Nhiều Agent review chéo code lẫn nhau, giống peer review trong đội dev thực.

---

## A. Agent Definition

Mỗi Agent được định nghĩa bởi 5 tham số cốt lõi:

| Tham số | Mục đích | Ví dụ |
|:--------|:---------|:------|
| **Role** | Vai trò chuyên biệt | `backend-specialist`, `security-auditor`, `qa-engineer` |
| **Model Level Group** | Mức độ model phù hợp | `Fast` / `Balanced` / `Powerful` (hoặc `auto`) |
| **Goal** | Mục tiêu cụ thể | "Viết code backend Go tuân thủ clean architecture" |
| **Context Config** | Giới hạn context | `{"max_input_tokens": 100000, "rag_sources": ["codebase", "docs"]}` |
| **Autonomy Level** | Mức tự chủ | `autonomous` / `supervised` / `approval_required` |

> **Nguyên tắc quan trọng (Planned Target):** Agent **không** được gắn chặt với danh sách skill cụ thể. Thay vào đó, **Agent Planner** phân tích task → chọn model level → nạp skill cần thiết qua JIT Loading (§03) cho mỗi sub-task. Hiện tại (Baseline), hệ thống lọc công cụ dựa trên static role-based tool templates của Agent.

## B. Orchestration Patterns (Multi-Agent)

Orchestrator hỗ trợ các mẫu phối hợp đa Agent sau:

| Pattern | Cách hoạt động | Trạng thái |
|:--------|:--------------|:-----------|
| **Sequential** | Điều phối tuần tự qua các bước workflow (Planner -> Backend/Frontend -> Reviewer) | **Implemented** |
| **Fan-out** | Phân nhánh công việc song song/tuần tự cho các bước Backend và Frontend | **Implemented** |
| **Hierarchical** | Manager delegate xuống Worker Agents | *Planned Target* |
| **Handoff** | Agent chuyển control cho Agent chuyên môn khác | *Planned Target* |
| **Group Chat** | Nhiều Agent tranh luận tìm giải pháp | *Planned Target* |
| **Cross-Harness Review** | 2+ Agent kiểm tra chéo code lẫn nhau | *Planned Target* |

Việc pattern nào (Sequential/Fan-out) được chọn cho từng task phụ thuộc vào complexity (Easy/Medium/Hard) và risk assessment — xem bảng đầy đủ tại §08 Workflow Engine — "Workflow Thay Đổi Theo Độ Phức Tạp" và sơ đồ "Parallel Coding & Ownership" (nguồn canonical cho luồng điều phối theo step).

## C. Agent Assignment & Gateway Integration

*   **Model routing:** Agent chỉ cần chọn Level (Fast/Balanced/Powerful). Gateway resolve model cụ thể (§01).
*   **Assignment Strategy:**
    *   **Auto-Join:** Agent tự động tham gia tất cả project trong Organization.
    *   **Manual Add:** Agent chỉ tham gia project khi được chỉ định.

## D. Self-Improving Learning Loop (Reusable Skills System)

> **Audit note (2026-07-23):** Cập nhật so với ghi chú 2026-07-12 — cơ chế mô tả dưới đây **đã được triển khai** dưới dạng bảng `learned_skills` (`server/pkg/models/learned_skill.go`: `trigger_keywords[]`, `usage_count`, `success_rate`, `source_task_id`, `status`), không còn là Planned Target.

Agent tự cải thiện qua vòng lặp học tập (Implemented):

1.  **Task đạt `merged`** → job history (steps, fixes, review feedback) được đưa qua 1 LLM call trích xuất (mở rộng từ `learning.DetectPatterns`).
2.  **Skill Extraction** → Đề xuất 0-2 `learned_skill` record ("cách chạy test ở repo X", "pattern sửa lỗi Y"). Autonomy `supervised` → skill ở trạng thái `draft` chờ approve trong `LearnedSkillsPanel` (§03); `autonomous` → active ngay.
3.  **Context Loading** → `context_load` tìm skill theo `trigger_keywords`/title match với task description (BM25, tái dùng memory search infra), nạp top-3 vào context với budget riêng (~2k tokens).
4.  **Usage Tracking** → Task merged/failed cập nhật `usage_count`/`success_rate` của skill đã được nạp.
5.  **Mid-Task Anti-Loop Nudge:** Trong tool-loop, mỗi 15 iterations, hệ thống chèn 1 system nudge (thuần Go, không LLM) tổng kết "những gì đã thử & thất bại" dựa trên tool-call history — cùng cùng tool + cùng args fail ≥3 lần → nudge cảnh báo cụ thể, chống lặp vòng vô ích.

## E. Quy Tắc Bắt Buộc

*   Agent phải tuân thủ Rule System (§02).
*   Backend validate chặt khi tạo Agent: reject provider không hợp lệ, role không nằm trong allowlist.
*   Tách biệt `model_route` (input config) và `resolved_model` (chỉ dùng cho telemetry).

---

**Dự án tham khảo:**

| Dự án | Lý do tham khảo |
|:------|:----------------|
| CrewAI | Role/Goal/Backstory, flow Hierarchical & Sequential |
| AutoGen | Actor Model — Handoff, Fan-out, Group Chat |
| Multica | Task claiming, daemon-based agent execution |
| AI-SDLC | Cross-Harness Review, Autonomy Tracker |
| Hermes Agent | Closed Learning Loop — tự tạo skill, tự cải tiến |
| OpenHands | Runtime sandbox cách ly, secret masking |
