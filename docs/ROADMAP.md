# Nền Tảng AI-Native SDLC — Lộ Trình & Tài Liệu Tham Khảo

## 1. Product Vision

Tài liệu này trình bày một lộ trình chi tiết và các dự án mã nguồn mở tham khảo cho việc xây dựng một nền tảng AI-Native SDLC (Software Development Lifecycle). Mục tiêu chính là cung cấp một hướng dẫn toàn diện, từ cấu trúc sản phẩm cốt lõi đến các tính năng cụ thể và các dự án mã nguồn mở có thể được sử dụng làm nền tảng hoặc nguồn cảm hứng. Điều này nhằm hỗ trợ các tổ chức và nhà phát triển xây dựng hệ thống của riêng mình một cách hiệu quả và chiến lược, tận dụng tối đa tiềm năng của trí tuệ nhân tạo trong quy trình phát triển phần mềm.
## 2. Target Architecture

**Mục tiêu chính của nền tảng:** Xây dựng một nền tảng giúp các nhà phát triển tự động hóa quy trình phát triển phần mềm thông qua các AI agent, từ việc tạo tác vụ đến hợp nhất mã nguồn (merge code).

**Quy trình phát triển phần mềm dự kiến với AI:**
1. **Con người tạo tác vụ (task)** kèm mô tả chi tiết. → DB status: `todo`
2. **Agent Planner phân tích và chia nhỏ tác vụ thành các task con** sử dụng định dạng OpenSpec. Bao gồm việc xác định Workflow cho tác vụ (ví dụ: task không có UI sẽ không cần FE join vào flow). Agent Planner quyết định luôn việc sử dụng Agent ở mức độ nào (Fast/Balanced/Powerful) và những nhóm kỹ năng (skills) nào là cần thiết để hoàn thành task. → DB status: `analyzing`
    - **Nếu thiếu thông tin:** Agent sẽ hỏi ngược lại nhà phát triển để bổ sung (ví dụ: "Task này ảnh hưởng đến module nào?", "Có yêu cầu backward compatibility không?", "File test nào cần cập nhật?"). Vòng lặp hỏi-đáp tiếp tục cho đến khi agent có đủ ngữ cảnh để phân tích chính xác.
    - Tham khảo: `resources/OpenSpec/src/core/` — Trình xử lý xác thực (Validation) kết quả phân tích.
    - Tham khảo: `resources/OpenSpec/schemas/spec-driven/schema.yaml` — Định nghĩa cấu trúc dữ liệu bắt buộc cho spec.

**↓ Tại đây, quy trình phân nhánh theo độ phức tạp:**

---

**🟢 Luồng EASY (Task đơn giản — linting, docs, sửa lỗi nhỏ):**

> Task dễ bỏ qua bước review của con người, đi thẳng vào thực thi.

3. *(Bỏ qua)* — Agent tự động xác nhận task đạt chuẩn Definition of Ready. Nếu vẫn thiếu thông tin nhỏ, agent sẽ hỏi nhanh nhà phát triển trước khi bắt đầu code. → DB status: auto-skip `spec_review`, chuyển thẳng sang `coding`
4. AI thực hiện viết mã nguồn. → DB status: `coding`
5. AI thực hiện đánh giá, sửa lỗi và kiểm thử mã nguồn. → DB status: `reviewing` ⟷ `fixing` → `testing`
6. AI tạo Pull Request (PR) hoặc Merge Request (MR). → DB status: `testing` (PR created)
7. Nhà phát triển thực hiện đánh giá cuối cùng (lightweight review). → DB status: `human_review`
8. Hợp nhất mã nguồn vào nhánh chính. → DB status: `merged`

---

**🟠🔴 Luồng MEDIUM / HARD (Task phức tạp — feature mới, refactor, kiến trúc):**

> Task phức tạp yêu cầu con người review và chốt spec trước khi AI bắt đầu code, tránh lãng phí token vào task mơ hồ.

3.  **Con người review, cập nhật và chốt final bản phân tích task.** Đảm bảo: → DB status: `spec_review`
    - Task có đủ thông tin (Definition of Ready): spec đầu vào/đầu ra rõ ràng, tài liệu kiến trúc, file liên quan.
    - **Nếu thiếu thông tin:** Reviewer yêu cầu nhà phát triển bổ sung hoặc tự bổ sung trực tiếp vào spec. Không chuyển sang bước 4 cho đến khi spec đầy đủ.
    - Kế hoạch thực thi (sub-tasks, thứ tự thực hiện) là hợp lý.
    - Các rủi ro đã được xác định và có phương án xử lý.
    - Đọc thêm: `resources/OpenSpec/` — Cách sử dụng Schema (JSON/YAML) chuẩn để giao nhiệm vụ cho AI (Spec-driven Development).
    - Tham khảo: `resources/OpenSpec/openspec-parallel-merge-plan.md` — Thiết kế tính năng chạy song song và merge.
4.  AI thực hiện viết mã nguồn (có thể chia sub-tasks song song cho nhiều agent). → DB status: `coding`
5.  AI thực hiện đánh giá, sửa lỗi và kiểm thử mã nguồn. → DB status: `reviewing` ⟷ `fixing` → `testing`
6.  AI tạo Pull Request (PR) hoặc Merge Request (MR). → DB status: `testing` (PR created)
7.  Nhà phát triển thực hiện đánh giá cuối cùng (deep review). → DB status: `human_review`
8.  Hợp nhất mã nguồn vào nhánh chính. → DB status: `merged`

---

```
Tóm tắt luồng:

  ┌──────────────────────────────────┐
  │ 0. Context Load                  │
  │ checkout repo, đọc conventions,  │
  │ CI config, ARCHITECTURE.md       │
  └────────┬─────────────────────────┘
           ▼
  ┌──────────────────────────────────┐
  │ 1. Tạo Task                      │
  │ → status: todo                   │
  └────────┬─────────────────────────┘
           ▼
  ┌────────────────────────────────────┐
  │ 2. Agent Planner                   │
  │ phân tích, tạo spec + complexity   │
  │ đánh giá risk domains              │
  │ chọn model level & skills (JIT)    │
  │ → status: analyzing               │
  └────────┬───────────────────────────┘
           │
     ┌─────┴──────────────┐
     ▼                    ▼
  [EASY + low-risk]  [EASY + high-risk /
     │                MEDIUM / HARD]
     │                    │
     │           ┌────────────────────────┐
     │           │ 3. Human review & chốt │
     │           │ spec, plan, risks      │
     │           │ → status: spec_review  │
     │           └───────┬────────────────┘
     │                   │
     ▼                   ▼
  ┌───────────────────────────────────────────┐
  │ 4. Plan execution → Code (song song nếu  │
  │    cần, branch riêng theo ownership)      │
  │ 5. Merge integration branch               │
  │ 6. Agent review chéo                      │
  │ 7. Fix (bounded: max N vòng)              │
  │ 8. Full test + lint + build               │
  │ 9. Tạo PR            → pr_ready           │
  │10. Human final review → human_review      │
  │11. Merge              → merged            │
  └───────────────────────────────────────────┘
```

**Hệ thống hướng tới các đặc điểm cốt lõi sau:**
*   **Self-host:** Cung cấp khả năng triển khai và vận hành trên hạ tầng riêng của doanh nghiệp, đảm bảo quyền kiểm soát và bảo mật dữ liệu.
*   **Enterprise-friendly:** Thiết kế để phù hợp với môi trường doanh nghiệp, hỗ trợ khả năng mở rộng, tích hợp và tuân thủ các tiêu chuẩn bảo mật cao.
*   **Multi-agent workflow:** Hỗ trợ các luồng làm việc phức tạp, cho phép nhiều AI agent phối hợp thực hiện các tác vụ phát triển phần mềm khác nhau.
*   **Autonomous SDLC:** Tự động hóa mức độ cao các giai đoạn trong vòng đời phát triển phần mềm, giảm thiểu sự can thiệp thủ công.
*   **Dễ dùng cho đội ngũ phát triển:** Cung cấp giao diện người dùng trực quan và trải nghiệm thân thiện, dễ dàng tích hợp vào quy trình làm việc hiện có của đội ngũ phát triển.
## 3. Current Implemented Baseline

Repo hiện tại không còn ở trạng thái "Phase 1 MVP". Theo `docs/ARCHITECTURE.md`, các phần Phase 1, Phase 2, Phase 3 và Phase 5 đã có baseline triển khai; Phase 4 và Phase 6 vẫn là kế hoạch/mục tiêu mở rộng.

**Baseline đã triển khai:**

| Phase | Scope | Status |
| :---- | :---- | :----- |
| Phase 0 | PoC CLI: Task → LLM → Code output | Implemented |
| Phase 1 | API Server + DB + CRUD cho Org/Project/Task/Agent/Rule/Skill | Implemented |
| Phase 2 | Auth + Git Integration + Web UI + Project System | Implemented |
| Phase 3a | Sandbox + Agent Manager + Orchestrator Core | Implemented |
| Phase 3b | Workflow Engine + Prompt Assembly + Execution UI | Implemented |
| Phase 4 | AI Gateway + Skill System + Evals | Partial Baseline / Planned Hardening |
| Phase 5 | Dashboard + Analytics + PR & Human Review | Implemented |
| Phase 6 | Remote Chatbots + Episodic Memory + Self-improving Agents | Planned |

**Cấu trúc sản phẩm cốt lõi:**

Cấu trúc sản phẩm được tổ chức theo phân cấp rõ ràng, đảm bảo khả năng quản lý, mở rộng và tích hợp hiệu quả các thành phần:

```
Organization
 └── Projects
      ├── Repositories
      ├── Tasks
      ├── Agents
      ├── Rules
      ├── Skills
      ├── Knowledge Base (Planned)
      └── Environments (Planned)
```
## 4. Execution Roadmap

Roadmap này mô tả trạng thái sản phẩm hiện tại và các mục tiêu tiếp theo. Mỗi tính năng có status block để phân biệt phần đã triển khai, phần đang làm và phần còn là migration target.

### 4.1 Done

*   Git integration baseline: GitHub account, repository credential resolution, sandbox clone, commit, push, PR generation.
*   Project/task/agent/rule/skill CRUD baseline.
*   Sandbox, orchestrator core, workflow execution UI.
*   Dashboard, analytics, audit/PR baseline.
*   Role-Based Capability Agents migration (migration 000012).
*   UI Onboarding: Setup checklist, AI Providers page, Git Accounts page, Members panel, Hire Agent Wizard.

### 4.2 In Progress

*   Unified AI Gateway backend core: budget tracking, credential pool rotation, cooldown recovery.
*   Baseline REST API & UI implemented. Remaining work: model-level config UX and telemetry/audit coverage.
*   UI polish: lint cleanup, setup checklist step alignment, project onboarding flow.

### 4.3 Next (Core Features — Priority)

*   **Global Rules System:** Org-level immutable rules injected into system prompt, rule conflict detection.
*   **Tool Isolation:** Orchestrator filters tools theo nhóm skill được Agent Planner nạp động qua JIT Loading; unauthorized tool execution rejected.
*   **Gateway-backed Model Level Group:** Agents sử dụng gateway provider với Model Level Group (Fast/Balanced/Powerful). Agent Planner quyết định level phù hợp cho từng task, gateway resolve model cụ thể.
*   **Multi-Agent Orchestration:** Sequential + Fan-out + Cross-Harness Review patterns.

### 4.4 Later

*   Episodic memory and self-improving agent loops.
*   Advanced repository intelligence and semantic search.
*   AI PR Assistant (explain changes on demand).
*   Langfuse/Helicone observability integration.
*   GitLab/Bitbucket/Gitea provider support.
*   Jira/Linear/GitHub Issues external tracker sync.

### 4.5 Coming Soon (Deferred)

*   Remote coding sessions through Discord/Telegram/Slack (§5.10) — deferred until core features stabilize.
*   Durable workflow engine migration (Temporal/LangGraph).

## 5. Feature Details

Chi tiết các tính năng cốt lõi của hệ thống đã được tách ra các file riêng biệt để dễ theo dõi:

* [5.1. Unified AI Gateway (Lớp Gateway AI Hợp Nhất)](features/5.1-unified-ai-gateway.md)
* [5.2a. Hệ Thống Quy Tắc (Rule System)](features/5.2a-rule-system.md)
* [5.2b. Hệ Thống Kỹ Năng (Skill System)](features/5.2b-skill-system.md)
* [5.3. Hệ Thống Agent (Role-Based Capability Agents)](features/5.3-agent-system.md)
* [5.4. Tích Hợp Git](features/5.4-git-integration.md)
* [5.5. Hệ Thống Dự Án (Project System)](features/5.5-project-system.md)
* [5.6. Hệ Thống Task](features/5.6-task-system.md)
* [5.7. Engine Workflow](features/5.7-workflow-engine.md)
* [5.8. PR & Human Review](features/5.8-pr-human-review.md)
* [5.9. Dashboard & Analytics](features/5.9-dashboard-analytics.md)
* [5.10. Tương Tác Đa Kênh (Remote Coding Sessions)](features/5.10-multi-channel-interaction.md)

## 6. Open Risks / Decisions

| Area | Decision / Risk | Current Direction |
| :--- | :-------------- | :---------------- |
| Unified AI Gateway | Frontend/API contract for provider credentials must stabilize before UI rebuild. | Follow §5.1 for gateway architecture; use `provider_credentials` as source of truth and `.env` only as fallback. |
| Role-Based Agents | Migration to Model Level Group (Fast/Balanced/Powerful) + deprecated task statuses backfill (`assigned`, `planning`, `in_progress`, `completed`). | Follow §5.3 for agent schema; §5.6 for task status migration. |
| Rule contract | No dedicated rule manual exists yet, so hard links to one are invalid. | Use §5.2a as canonical rule contract until a dedicated manual is created. |
| Multi-agent orchestration | Fan-out, cross-harness review, handoff and group chat can increase merge conflicts and review complexity. | Start with sequential + fan-out + cross-harness only; keep handoff/group chat later. |
| Repository intelligence | Semantic search, dependency graph and historical failure memory need storage/indexing decisions. | Keep as later roadmap after gateway and agent migration. |
| Security & Governance | RBAC, audit logs, policy engine and sandbox isolation must be complete before enterprise deployment. | Expand audit coverage through gateway/credential lifecycle first. |
| Skill CMS Dashboard | Local Dashboard phải sync file-system changes ngược lại Git, cần đảm bảo `registry.json` consistency khi concurrent edits. | Current: DB-backed CRUD; next: single-user Git-backed file sync with locking later. |

## 7. Reference Projects

**KHÔNG** nên xây dựng mọi thứ từ đầu. Cách tiếp cận được khuyến nghị là tái sử dụng các dự án mã nguồn mở làm tham chiếu và khối xây dựng, tập trung vào việc tạo ra giá trị độc đáo.

**Lớp & Nền tảng đề xuất:**
| Lớp                  | Nền tảng đề xuất             |
| :------------------- | :--------------------------- |
| Agent runtime        | OpenHands / OpenClaw         |
| Điều phối (Orchestration) | Multica                      |
| Workflow             | Temporal/LangGraph           |
| Gateway AI           | LiteLLM/9Router/Free Claude Code/LLM Key Manager |
| Task UX              | Plane                        |
| Khả năng quan sát AI | Langfuse                     |

**Tập trung phát triển tùy chỉnh vào:**
1.  **Workflow UX:** Phát triển trải nghiệm người dùng độc đáo và tối ưu cho nhà phát triển.
2.  **Phối hợp Agent:** Xây dựng và tinh chỉnh vòng lặp Task → review → fix → test để đạt hiệu quả cao nhất.
3.  **Hệ thống quy tắc/kỹ năng:** Định nghĩa kiến thức tổ chức và hành vi AI một cách chính xác.
4.  **Thông minh Repository:** Phát triển bộ nhớ mã hóa nhận biết ngữ cảnh để hỗ trợ AI tốt hơn.

## 8. Kết Luận

Việc xây dựng một nền tảng AI-Native SDLC mạnh mẽ đòi hỏi sự kết hợp giữa tầm nhìn chiến lược và khả năng tận dụng các công nghệ hiện có. Bằng cách tham khảo và tích hợp các dự án mã nguồn mở hàng đầu, bạn có thể đẩy nhanh quá trình phát triển, tập trung vào việc tạo ra giá trị độc đáo cho đội ngũ của mình. Lộ trình và các tài liệu tham khảo trong báo cáo này sẽ là kim chỉ nam vững chắc cho hành trình xây dựng nền tảng AI-Native SDLC của bạn.