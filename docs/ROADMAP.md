# Nền Tảng AI-Native SDLC — Lộ Trình & Tài Liệu Tham Khảo

## 1. Product Vision

Tài liệu này trình bày một lộ trình chi tiết và các dự án mã nguồn mở tham khảo cho việc xây dựng một nền tảng AI-Native SDLC (Software Development Lifecycle). Mục tiêu chính là cung cấp một hướng dẫn toàn diện, từ cấu trúc sản phẩm cốt lõi đến các tính năng cụ thể và các dự án mã nguồn mở có thể được sử dụng làm nền tảng hoặc nguồn cảm hứng. Điều này nhằm hỗ trợ các tổ chức và nhà phát triển xây dựng hệ thống của riêng mình một cách hiệu quả và chiến lược, tận dụng tối đa tiềm năng của trí tuệ nhân tạo trong quy trình phát triển phần mềm.
## 2. Target Architecture

**Mục tiêu chính của nền tảng:** Xây dựng một nền tảng giúp các nhà phát triển tự động hóa quy trình phát triển phần mềm thông qua các AI agent, từ việc tạo tác vụ đến hợp nhất mã nguồn (merge code).

**Quy trình phát triển phần mềm dự kiến với AI:**
1.  **Nhà phát triển tạo tác vụ (task)** kèm mô tả chi tiết (tiêu đề, yêu cầu, ngữ cảnh, file liên quan).
2.  **AI agent tiếp nhận, phân tích và phân loại tác vụ** theo độ phức tạp (Easy / Medium / Hard). Agent tạo bản phân tích task theo chuẩn Spec-driven (JSON/YAML Schema), bao gồm: phạm vi thay đổi, file ảnh hưởng, rủi ro, và kế hoạch thực thi.
    - **Nếu thiếu thông tin:** Agent sẽ hỏi ngược lại nhà phát triển để bổ sung (ví dụ: "Task này ảnh hưởng đến module nào?", "Có yêu cầu backward compatibility không?", "File test nào cần cập nhật?"). Vòng lặp hỏi-đáp tiếp tục cho đến khi agent có đủ ngữ cảnh để phân tích chính xác.
    - Tham khảo: `resources/OpenSpec/src/core/` — Trình xử lý xác thực (Validation) kết quả phân tích.
    - Tham khảo: `resources/OpenSpec/schemas/spec-driven/schema.yaml` — Định nghĩa cấu trúc dữ liệu bắt buộc cho spec.
    - Tham khảo: `resources/OpenSpec/openspec/specs/` — Ví dụ spec thực tế.

**↓ Tại đây, quy trình phân nhánh theo độ phức tạp:**

---

**🟢 Luồng EASY (Task dễ — linting, docs, sửa lỗi nhỏ, cập nhật config):**

> Task dễ bỏ qua bước review của con người, đi thẳng vào thực thi để tiết kiệm thời gian.

3.  *(Bỏ qua)* — Agent tự động xác nhận task đạt chuẩn Definition of Ready. Nếu vẫn thiếu thông tin nhỏ, agent sẽ hỏi nhanh nhà phát triển trước khi bắt đầu code.
4.  AI thực hiện viết mã nguồn.
5.  AI thực hiện đánh giá, sửa lỗi và kiểm thử mã nguồn.
6.  AI tạo Pull Request (PR) hoặc Merge Request (MR).
7.  Nhà phát triển thực hiện đánh giá cuối cùng (lightweight review).
8.  Hợp nhất mã nguồn vào nhánh chính.

---

**🟠🔴 Luồng MEDIUM / HARD (Task phức tạp — feature mới, refactor, kiến trúc, bảo mật):**

> Task phức tạp yêu cầu con người review và chốt spec trước khi AI bắt đầu code, tránh lãng phí token vào task mơ hồ.

3.  **Con người review, cập nhật và chốt final bản phân tích task.** Đảm bảo:
    - Task có đủ thông tin (Definition of Ready): spec đầu vào/đầu ra rõ ràng, tài liệu kiến trúc, file liên quan.
    - **Nếu thiếu thông tin:** Reviewer yêu cầu nhà phát triển bổ sung hoặc tự bổ sung trực tiếp vào spec. Không chuyển sang bước 4 cho đến khi spec đầy đủ.
    - Kế hoạch thực thi (sub-tasks, thứ tự thực hiện) là hợp lý.
    - Các rủi ro đã được xác định và có phương án xử lý.
    - Đọc thêm: `resources/OpenSpec/` — Cách sử dụng Schema (JSON/YAML) chuẩn để giao nhiệm vụ cho AI (Spec-driven Development).
    - Tham khảo: `resources/OpenSpec/openspec-parallel-merge-plan.md` — Thiết kế tính năng chạy song song và merge.
4.  AI thực hiện viết mã nguồn (có thể chia sub-tasks song song cho nhiều agent).
5.  AI thực hiện đánh giá, sửa lỗi và kiểm thử mã nguồn.
6.  AI tạo Pull Request (PR) hoặc Merge Request (MR).
7.  Nhà phát triển thực hiện đánh giá cuối cùng (deep review).
8.  Hợp nhất mã nguồn vào nhánh chính.

---

```
Tóm tắt luồng:

  ┌─────────────────┐
  │ 1. Tạo Task     │
  └────────┬────────┘
           ▼
  ┌─────────────────────────┐
  │ 2. AI phân tích &       │
  │    phân loại (E/M/H)    │
  └────────┬────────────────┘
           │
     ┌─────┴──────┐
     ▼            ▼
  [EASY]     [MEDIUM/HARD]
     │            │
     │    ┌───────▼────────┐
     │    │ 3. Con người   │
     │    │ review & chốt  │
     │    │ final spec     │
     │    └───────┬────────┘
     │            │
     ▼            ▼
  ┌─────────────────────────┐
  │ 4. AI viết mã nguồn     │
  │ 5. AI review & test     │
  │ 6. AI tạo PR            │
  │ 7. Con người review     │
  │ 8. Merge                │
  └─────────────────────────┘
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
| Phase 4 | AI Gateway + Skill System + Evals | In Progress / Planned |
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
      ├── Knowledge Base
      └── Environments
```
## 4. Execution Roadmap

Roadmap này mô tả trạng thái sản phẩm hiện tại và các mục tiêu tiếp theo. Mỗi tính năng có status block để phân biệt phần đã triển khai, phần đang làm và phần còn là migration target.

### 4.1 Done

*   Git integration baseline: GitHub account, repository credential resolution, sandbox clone, commit, push, PR generation.
*   Project/task/agent/rule/skill CRUD baseline.
*   Sandbox, orchestrator core, workflow execution UI.
*   Dashboard, analytics, audit/PR baseline.

### 4.2 In Progress

*   Unified AI Gateway backend core: virtual key validation, budget tracking, credential pool rotation, cooldown recovery.
*   REST APIs and frontend rebuild for provider credentials / virtual keys.

### 4.3 Next

*   Role-Based Capability Agents migration.
*   Tool isolation through assigned skills.
*   Gateway-backed `model_route` integration for agents.
*   Rule & Skill System hardening.

### 4.4 Later

*   Remote coding sessions through Discord/Telegram/Slack.
*   Episodic memory and self-improving agent loops.
*   Advanced repository intelligence and semantic search.

## 5. Feature Details

### 5.1. Tích Hợp Git

**Status:** Implemented  
**Owner docs:** `docs/ARCHITECTURE.md`  
**Code areas:** `server/internal/gitops`, `server/internal/workflow`, `server/internal/orchestrator`, `web/`  
**Blocking decisions:** GitLab/Bitbucket/Gitea priority order.  
**Acceptance criteria:** User can attach a Git account to a project, agent can clone, commit, push, create PR, and persist PR URL on the task/workflow.

**Mục tiêu:** Cho phép AI tự động thực thi toàn bộ vòng đời Git — từ clone repo đến tạo Pull Request — bằng cách sử dụng tài khoản Git do người dùng cấu hình, đảm bảo an toàn và tách biệt credential giữa các project.

---

#### Luồng Vận Hành Thực Tế (Đã Triển Khai)

```
Bước 1: Người dùng thêm GitHub Account
        └── GitAccount: token, provider (github/gitlab), base_url (tùy chọn cho GitHub Enterprise)

Bước 2: Người dùng tạo Project
        ├── Nhập Repo URL (ví dụ: https://github.com/org/my-repo.git)
        └── Chọn Git Account → Repository.git_account_id được lưu

Bước 3: AI Agent nhận Task
        └── Orchestrator bắt đầu workflow cho task

Bước 4: Sandbox clone Repo
        ├── Resolve credentials theo thứ tự ưu tiên:
        │   1. Token gắn trực tiếp trên Repository (override thủ công)
        │   2. Git Account được chọn khi tạo Project (repo.git_account_id)
        │   3. Git Account cấp Org khớp với provider (fallback)
        └── Clone vào workspace cô lập theo task ID

Bước 5: Commit
        ├── Tự động cấu hình git identity theo Agent thực hiện:
        │   user.name  = "AutoCodeOS [{agent.role}]"  (vd: "AutoCodeOS [backend-specialist]")
        │   user.email = "{agent.role}@autocodeos.local"
        ├── Stage tất cả thay đổi (git add .)
        └── Bỏ qua nếu không có thay đổi (tránh crash trên working tree sạch)

Bước 6: Push
        └── Push lên branch mới: autocode/task-{task_id}

Bước 7: Tạo Pull Request
        ├── Tiêu đề: "AutoCodeOS: {task.title}"
        ├── Mô tả: task ID + description
        └── PR URL được lưu vào workflow output → Task chuyển sang HUMAN_REVIEW
```

---

**Cơ chế Resolve Credentials (3 lớp ưu tiên):**

| Ưu tiên | Nguồn Credential | Khi Nào Dùng |
| :------ | :--------------- | :----------- |
| 1 | `Repository.token` (override thủ công) | Repo có token riêng, không dùng git account |
| 2 | `GitAccount` được liên kết qua `git_account_id` | Project đã chọn Git Account cụ thể |
| 3 | `GitAccount` cấp Org khớp `provider` | Fallback khi không có link trực tiếp |

**Hỗ trợ GitHub Enterprise / Self-hosted:**
*   `GitAccount.base_url` để tùy chỉnh API endpoint (ví dụ: `https://github.company.com/api/v3`).
*   Mặc định là `https://api.github.com` khi `base_url` để trống.

**Mở rộng trong tương lai:**
*   Hỗ trợ tích hợp với GitLab.
*   Hỗ trợ tích hợp với Bitbucket.
*   Hỗ trợ Gitea (Git self-hosted).

**Dự án tham khảo:**
| Dự án           | Lý do tham khảo                                |
| :-------------- | :--------------------------------------------- |
| GitHub App Docs | Cung cấp các mẫu tích hợp Git và API hiệu quả |
| Gitea           | Kiến trúc và triển khai Git self-hosted       |
| GitLab CE       | Ý tưởng về quy trình CI/CD và Merge Request   |


### 5.2. Unified AI Gateway (Lớp Gateway AI Hợp Nhất)

**Status:** In Progress  
**Owner docs:** `docs/plans/PLAN-unified-ai-gateway.md`  
**Code areas:** `server/internal/gateway`, `server/pkg/llm`, `server/migration/000011_unified_ai_gateway.*.sql`, `web/src/` provider settings UI  
**Blocking decisions:** Final REST API shape for credentials/virtual keys, frontend provider settings UX, audit-log coverage.  
**Acceptance criteria:** Admin can configure multiple provider credentials, issue virtual keys with budget/rate limits, route requests by `model_route`, and observe usage/cooldown/audit events.

**Mục tiêu:** Cung cấp một lớp gateway tập trung quản lý **Virtual Key, định tuyến model, budget tracking, format translation và fallback tự động** — thay thế cách tiếp cận đơn giản "lưu API key + chọn model" trước đây. Agent và client chỉ gọi vào gateway với một route ổn định, không cần biết provider thật phía sau.

> **Tiến độ triển khai (Đang thực hiện):**
> - ✅ **Hoàn thành Backend Core:** Đã implement Virtual Key validation, Budget tracking (atomic), Multi-key pool rotation (fill-first/round-robin), Cooldown recovery.
> - ✅ **Tích hợp Orchestrator:** Gateway đã được gắn vào pipeline (fallback tự động sang biến môi trường nếu DB trống).
> - ⏳ **Đang phát triển:** REST API endpoints cho credentials/virtual keys.
> - ⏳ **Chưa bắt đầu:** Xây dựng lại giao diện AI Providers trên Frontend, hệ thống Audit Log.

> **Nguyên lý cốt lõi (tham khảo 9router):** Agent không bao giờ gọi thẳng OpenAI/Anthropic/Gemini. Agent gọi vào internal gateway có contract cố định. Gateway chịu trách nhiệm resolve model, dịch format, chọn credential, fallback và tracking.

---

#### A. Virtual Key Architecture (tham khảo LiteLLM + Langfuse)

*   **Khái niệm:** Hệ thống cấp phát "Virtual Key" (ví dụ: `sk-aco-...`) cho mỗi Agent/Project/User. Client sử dụng virtual key này thay vì real API key của provider.
*   **Lưu trữ:** Real provider credentials lưu trong bảng `provider_credentials` (AES-256-GCM), thay thế pattern cũ lưu provider key trong `secrets` hoặc `.env`. Virtual keys lưu trong bảng `virtual_keys` với quan hệ đến organization/project/agent. Legacy `.env`/`secrets` chỉ là fallback hoặc migration context khi DB gateway chưa được cấu hình.
*   **Multi-Key per Provider (tham khảo 9router):** Mỗi provider hỗ trợ **nhiều API key** (nhiều account) cùng lúc. Gateway tự chọn credential phù hợp tại runtime:
    *   **Credential Selection Strategy:** `fill-first` (dùng key đầu tiên cho đến khi hết quota) hoặc `round-robin` (luân phiên giữa các key).
    *   **Auto-exclude:** Key bị rate limit/quota → gateway tự động exclude và chuyển sang key khác cùng provider.
    *   **Cooldown & Recovery:** Key bị lock tạm thời theo cooldown period, tự động unlock khi hết cooldown.
    *   **Lợi ích:** Aggregate rate limit cao hơn (3 key OpenAI = 3x RPM), không bao giờ gián đoạn khi 1 key hết quota.
*   **Lợi ích Virtual Key:**
    *   Decouple authentication khỏi provider — thay đổi provider key không ảnh hưởng agent config.
    *   Mỗi virtual key có **budget cap** (dollar-value) và **rate limit** (RPM/TPM) riêng biệt.
    *   Nếu agent loop vô hạn, chỉ tiêu hết budget của virtual key đó, không ảnh hưởng toàn organization.
*   **Audit:** Ghi log đầy đủ lifecycle: Who (user/agent), What (action), When (timestamp), Where (IP/source) cho mọi event tạo/cập nhật/sử dụng/xóa key (tham khảo Langfuse RBAC audit pattern).

#### B. Model Routing & Combo (tham khảo 9router)

Agent config chỉ cần 2 trường ổn định — không lưu raw vendor model:

```json
{
  "provider": "gateway",
  "model_route": "balanced"
}
```

**Các chế độ routing:**

| Chế độ | Cách hoạt động | Ví dụ |
| :----- | :------------- | :---- |
| **Auto by Complexity** | Gateway tự map theo task complexity: `easy→fast`, `medium→balanced`, `hard→powerful` | `"model_route": "auto"` |
| **Tier** | Chọn tier cố định, gateway resolve sang provider tốt nhất hiện có | `"model_route": "balanced"` |
| **Combo** | Chuỗi fallback nhiều model/provider — nếu model đầu lỗi quota, thử model tiếp theo | `"model_route": "coding-default"` |
| **Specific** | Chọn model cụ thể qua dropdown (advanced mode, có validation) | `"model_route": "anthropic/claude-sonnet-4"` |

**Combo routing** (pattern mạnh nhất):
```
coding-default → 1. Subscription model (đã trả phí)
                 2. Cheap API model
                 3. Free model (emergency fallback)
```
Agent chỉ biết route `coding-default`, gateway chịu trách nhiệm fallback — đảm bảo zero downtime.

#### C. Format Translation & Token Saver

*   **Format Translation:** Gateway tự động phát hiện source format (OpenAI Chat, Claude Messages, Gemini, OpenAI Responses) và dịch sang target format của provider đích. Cho phép dùng bất kỳ model nào cho bất kỳ client nào mà không cần thay đổi code.
*   **Token Saver (tham khảo RTK — 9router):** Trước khi gửi request lên LLM, gateway tự phát hiện `tool_result` (git diff, grep, ls, build logs) và nén nội dung lại — tiết kiệm 20-40% input token mà không mất context. Không nén error traces.

#### D. Cấu hình Model per Provider (Hiện tại)

| Provider | Fast Model | Balanced Model | Powerful Model |
| :------- | :--------- | :------------- | :------------- |
| OpenAI | `gpt-4o-mini` | `gpt-4o` | `gpt-4o` |
| Anthropic | *(không có)* | `claude-sonnet-4-20250514` | `claude-opus-4-20250514` |
| Google Gemini | `gemini-2.5-flash` | `gemini-2.5-pro` | `gemini-2.5-pro` |
| Gateway | `fast` | `balanced` | `powerful` |

#### E. Luồng Vận Hành

```
Bước 1: Admin vào Settings → AI Providers
        ├── Thêm 1 hoặc NHIỀU API Key cho mỗi provider (multi-key)
        │   ├── Mỗi key → AES-256-GCM → lưu DB với label + trạng thái
        │   ├── Chọn Credential Strategy: fill-first | round-robin
        │   └── Hỗ trợ Base URL tuỳ chỉnh per key (self-hosted / proxy)
        └── Hiển thị trạng thái: active ✓ / rate-limited ⚠ / disabled ✗ + 4 ký tự cuối

Bước 2: Cấu hình Model Tier cho từng provider
        ├── Fast Model   → task đơn giản, nhanh
        ├── Balanced     → mặc định cho hầu hết task
        └── Powerful     → task phức tạp, kiến trúc, review

Bước 3: Cấu hình Combo Routes (tuỳ chọn)
        └── coding-default, premium-coding, cheap-fallback...

Bước 4: Hệ thống cấp Virtual Key cho mỗi Agent/Project
        ├── Budget cap (USD) + Rate limit (RPM/TPM)
        └── Audit log tự động

Bước 5: Runtime resolve theo thứ tự ưu tiên
        1. Virtual Key → resolve model_route/tier/combo
        2. Key & model lưu trong DB (qua UI Settings)
        3. Biến môi trường .env (fallback nếu DB chưa cấu hình)

Bước 6: Gateway xử lý request
        ├── Validate virtual key + check budget
        ├── Resolve model route → chọn provider
        ├── Chọn credential (từ pool multi-key của provider)
        │   ├── Lọc key bị exclude (rate-limited / cooldown)
        │   ├── Chọn theo strategy: fill-first hoặc round-robin
        │   └── Nếu hết key cùng provider → fallback sang provider khác
        ├── Format Translation (source → target format)
        ├── Token Saver nén tool_result trước khi dispatch
        ├── Execute provider → nếu lỗi quota/rate-limit:
        │   ├── Lock key hiện tại (cooldown)
        │   └── Thử key khác cùng provider → thử provider khác
        └── Record usage telemetry (provider, model, key_label, tokens, cost, latency)
```

#### F. Bảo mật & Lưu trữ

*   Real API Key mã hoá **AES-256-GCM** — không bao giờ trả plaintext.
*   Virtual Key hashed with salt cho verification, audit log đầy đủ lifecycle.
*   Mỗi Organization có bộ key + model config riêng biệt.
*   Backend validate chặt: reject provider không nằm trong allowlist, model route không hợp lệ.

#### G. Mở rộng trong tương lai

*   Per-project provider override (project dùng key / model riêng khác org).
*   Rotation key tự động khi phát hiện leak/expire.
*   Tích hợp AWS Secrets Manager / HashiCorp Vault.
*   OAuth token refresh tự động cho các subscription provider.

**Dự án tham khảo:**

| Dự án | Lý do tham khảo |
| :---- | :-------------- |
| 9router | Combo routing, format translation, RTK token saver, 3-tier fallback (xem `docs/references/9router-agent-connection-report.md`) |
| LLM Key Manager | Hybrid AI Gateway, Browser-native, Multi-key failover, Effective Score routing (xem `resources/llm-key-manager/README.md`) |
| LiteLLM | Virtual Key architecture, proxy đa provider, budget/quota per key |
| Langfuse | Audit logging, RBAC, cách lưu trữ API key an toàn (hash + AES-256-GCM) |
| Free Claude Code | Drop-in proxy pattern, protocol normalization OpenAI↔Anthropic (xem `docs/references/Learning_Report.md` §9) |
| OpenHands | Quản lý LLM provider key theo từng user/session |

---

### 5.3. Hệ Thống Agent (Role-Based Capability Agents)

**Status:** Planned / Migration Target  
**Owner docs:** `docs/plans/PLAN-role-based-agents.md`  
**Code areas:** `server/pkg/models/agent.go`, `server/internal/repository/agent.go`, `server/internal/service/agent.go`, `server/internal/orchestrator`, `web/src/` agent CRUD UI  
**Blocking decisions:** Whether migration `000012` drops legacy agent rows or backfills them into capability-based records; default role template set.  
**Acceptance criteria:** Agent config uses Role, Goal, Allowed Tools, Context Config, Autonomy Level, and `model_route`; orchestrator only exposes tools assigned through `agent_skills`.

> **Migration note:** Đây là target shape, chưa phải schema hiện tại. Code hiện tại ở `server/pkg/models/agent.go` vẫn dùng `provider`, `model`, và `level`; migration chi tiết nằm trong `docs/plans/PLAN-role-based-agents.md`.

**Mục tiêu:** Chuyển đổi từ mô hình phân loại Agent theo cấp độ khó (Easy/Medium/Hard) sang kiến trúc **Agent dựa trên Vai trò & Năng lực** — mỗi Agent được định nghĩa bởi Role, Goal, Allowed Tools và Autonomy Level. Orchestrator phối hợp các Agent qua các pattern rõ ràng (Hierarchical, Sequential, Fan-out, Handoff).

> **Tham khảo chính:** CrewAI (Role/Goal/Backstory), AutoGen (Actor Model — Handoff, Fan-out, Group Chat), AI-SDLC Framework (Cross-Harness Review, Autonomy Tracker), Multica (Task claiming, daemon-based agent execution).

---

#### A. Agent Definition (Capability-Based)

Mỗi Agent được định nghĩa bởi 5 tham số cốt lõi thay vì chỉ `role + tier`:

| Tham số | Mô tả | Ví dụ |
| :------ | :----- | :---- |
| **Role** | Vai trò chuyên biệt | `backend-specialist`, `security-auditor`, `qa-engineer` |
| **Goal** | Mục tiêu cụ thể của Agent | "Viết code backend Go tuân thủ clean architecture" |
| **Allowed Tools** | Danh sách tool/skill Agent được phép sử dụng | `["run_tests", "git_commit", "read_file", "write_file"]` |
| **Context Config** | Cấu hình context window: max tokens, RAG sources | `{"max_input_tokens": 100000, "rag_sources": ["codebase", "docs"]}` |
| **Autonomy Level** | Mức độ tự chủ khi thực thi | `autonomous` / `supervised` / `approval_required` |

**Các vai trò Agent mở rộng:**

| Vai trò | Chức năng | Allowed Tools (mặc định) |
| :------ | :-------- | :----------------------- |
| **Planner** | Phân tích task, tạo spec, chia sub-tasks | `analyze_codebase`, `create_spec`, `decompose_task` |
| **Backend** | Phát triển mã nguồn backend | `read_file`, `write_file`, `run_tests`, `git_commit` |
| **Frontend** | Phát triển giao diện người dùng | `read_file`, `write_file`, `run_tests`, `git_commit` |
| **Reviewer** | Review chéo code (Cross-Harness) | `read_file`, `analyze_diff`, `add_review_comment` |
| **QA** | Kiểm thử và đảm bảo chất lượng | `run_tests`, `analyze_logs`, `read_file` |
| **Security Auditor** | Quét lỗ hổng, kiểm tra secret leak | `scan_vulnerabilities`, `read_file`, `analyze_logs` |
| **DB Architect** | Thiết kế schema, migration, query optimization | `read_file`, `write_file`, `create_migration`, `run_tests` |

> **Nguyên tắc Tool Isolation (tham khảo AutoGen):** Mỗi Agent chỉ thấy các tool trong `allowed_tools` — tránh "Tool Overload" (agent bị nhầm lẫn khi có 15+ tool). Orchestrator gán tool phù hợp dựa trên role.

#### B. Orchestration Patterns (Multi-Agent)

Thay vì chỉ dispatch 1 agent/task, Orchestrator hỗ trợ các pattern phối hợp đa agent:

| Pattern | Mô tả | Khi nào dùng |
| :------ | :----- | :----------- |
| **Sequential** | Output agent A → Input agent B | Planner → Backend → Reviewer → QA |
| **Hierarchical** | Manager agent delegate xuống worker agents | Task phức tạp cần phân chia sub-tasks |
| **Fan-out (Concurrent)** | Orchestrator giao sub-tasks song song cho nhiều agent | Nhiều module độc lập cần code cùng lúc |
| **Handoff** | Agent chuyển control cho agent chuyên môn khác | Backend gặp vấn đề DB → handoff cho DB Architect |
| **Group Chat** | Nhiều agent tranh luận để đưa ra giải pháp tối ưu | Quyết định kiến trúc, code review phức tạp |
| **Cross-Harness Review** | 2+ agent kiểm tra chéo code của nhau | Đảm bảo chất lượng, ghi attestation (tham khảo AI-SDLC) |

**Luồng điều phối chính:**
```
Task nhận vào
  → Planner Agent phân tích & chia sub-tasks
  → Orchestrator chọn pattern phù hợp:
      ├── Easy task    → Sequential (1 agent, auto-approve)
      ├── Medium task  → Sequential + Cross-Harness Review
      └── Hard task    → Fan-out (nhiều agent song song) + Group Chat review
  → Mỗi agent chạy trong Sandbox cách ly (Docker)
  → Kết quả merge (tham khảo OpenSpec parallel merge strategy)
  → Cross-Harness Review giữa 2+ agent
  → PR tạo tự động
```

#### C. Agent Assignment & Gateway Integration

*   **Provider mặc định:** Agent trỏ `gateway` + `model_route` thay vì vendor model trực tiếp (xem §5.2.B).
*   **Assignment Strategy:**
    *   **Auto-Join:** Agent tự động tham gia tất cả project trong Organization.
    *   **Manual Add:** Agent chỉ tham gia project khi được chỉ định.
*   **Virtual Key per Agent:** Mỗi agent được cấp virtual key riêng với budget cap — kiểm soát chi phí ở mức agent.

#### D. Self-Improving Learning Loop (tham khảo Hermes Agent)

Agent có khả năng tự cải thiện qua vòng lặp học tập tích hợp:

1.  **Task hoàn thành thành công** → hệ thống verify success criteria (test pass, review approved).
2.  **Skill Extraction:** Trích xuất procedural steps thành "Skill" mới (Markdown) ở trạng thái `DRAFT`.
3.  **Human/Reviewer Gate:** Con người hoặc reviewer agent promote skill sang `ACTIVE`.
4.  **Future Retrieval:** Ở các task sau, agent dùng Vector Search (pgvector) hoặc FTS5 để pull relevant skills vào context trước khi thực thi — giảm token, latency và error rate.
5.  **Autonomy Tracking:** Ghi nhận kết quả vào `AutonomyTracker` (tham khảo AI-SDLC) để đánh giá độ tin cậy của agent theo thời gian.

#### E. Quy tắc bắt buộc

*   Các Agent phải tuân thủ rule contract mô tả trong §5.6 cho đến khi có tài liệu rule manual riêng.
*   Backend validate chặt khi tạo agent: reject provider không hợp lệ, role không nằm trong allowlist, model route không khớp provider.
*   Agent config tách biệt `model_route` (input config) và `resolved_model` (chỉ dùng cho telemetry sau khi request chạy xong).

**Dự án tham khảo:**

| Dự án | Lý do tham khảo |
| :---- | :-------------- |
| CrewAI | Kiến trúc Agent dựa trên Role/Goal/Backstory, flow Hierarchical & Sequential |
| AutoGen | Actor Model — Handoff, Fan-out, Group Chat; giải quyết Context Window Saturation & Tool Overload |
| Multica | Task claiming (ClaimTask), daemon-based agent execution, analytics tracking (xem `docs/references/Learning_Report.md` §1) |
| AI-SDLC | Cross-Harness Review, Autonomy Tracker, Definition of Ready gate (xem `docs/references/Learning_Report.md` §3) |
| Hermes Agent | Closed Learning Loop — tự tạo skill, tự cải tiến, subagent parallelization (xem `docs/references/Learning_Report.md` §8) |
| OpenHands | Runtime sandbox cách ly, smart masking ngăn secret leak |
| OpenClaw | Multi-channel gateway, sandboxing với phân quyền nghiêm ngặt (xem `docs/references/Learning_Report.md` §2) |

### 5.4. Hệ Thống Dự Án (Project System)

**Status:** Implemented baseline / Planned enhancements  
**Owner docs:** `docs/ARCHITECTURE.md`  
**Code areas:** `server/internal/service/project`, `server/internal/repository`, `server/pkg/models`, `web/src/` project screens  
**Blocking decisions:** Scope of shared knowledge base and project-level workflow defaults.  
**Acceptance criteria:** Project can contain repositories, shared configuration, rules/skills references, and AI workflow defaults.

**Khái niệm:** Một Project là một đơn vị tổ chức cấp cao, bao gồm nhiều repository, cấu hình workflow AI chung, và các quy tắc/kiến thức chia sẻ, tạo nên một môi trường phát triển thống nhất.

**Tính năng chính:**
*   **Tạo Project:** Cho phép định nghĩa tên dự án, mô tả, cấu hình môi trường, quy tắc chung và workflow AI mặc định.
*   **Thêm Repositories:** Kết nối và quản lý các repository liên quan, gắn thẻ (tag) và gán ngôn ngữ/loại để phân loại.
*   **Kiến thức chia sẻ:** Cung cấp một kho lưu trữ tập trung cho tài liệu, quy ước mã hóa, kiến trúc hệ thống và các RFCs (Request for Comments) chung của dự án.

**Dự án tham khảo:**
| Dự án       | Lý do tham khảo                                |
| :---------- | :--------------------------------------------- |
| Backstage   | Khái niệm cổng developer (developer portal) toàn diện |
| Plane       | Trải nghiệm người dùng (UX) quản lý dự án/tác vụ hiện đại |
| OpenProject | Các tính năng quản lý dự án cấp doanh nghiệp   |


### 5.5. Hệ Thống Task

**Status:** Implemented baseline / Planned integrations  
**Owner docs:** `docs/ARCHITECTURE.md`  
**Code areas:** `server/internal/service/task`, `server/internal/repository`, `server/pkg/models`, `web/src/` task screens  
**Blocking decisions:** External issue tracker priority: Jira, Linear, GitHub Issues, Notion.  
**Acceptance criteria:** Developer can create tasks with repository context, task lifecycle states are persisted, and orchestrator can advance task status through planning/coding/review/testing/human review.

**Mục tiêu:** Cung cấp cơ chế để nhà phát triển tạo tác vụ và giao cho AI thực thi một cách tự động.

**Tính năng chính:**
*   **Tạo Task:** Cho phép định nghĩa tiêu đề, mô tả chi tiết, độ khó, độ ưu tiên, các repository liên quan và nhãn (labels) cho mỗi tác vụ.
*   **Vòng đời Task:** Quản lý trạng thái của tác vụ qua các giai đoạn:
    *   TODO → ASSIGNED → PLANNING → CODING → REVIEWING → FIXING → TESTING → HUMAN_REVIEW → MERGED

**Tích hợp trong tương lai:**
*   Tích hợp với các hệ thống quản lý tác vụ phổ biến như Jira.
*   Tích hợp với Linear để quản lý tác vụ hiệu quả.
*   Đồng bộ hóa với GitHub Issues.
*   Tích hợp với Notion để quản lý tài liệu và tác vụ.

**Dự án tham khảo:**
| Dự án       | Lý do tham khảo                                |
| :---------- | :--------------------------------------------- |
| Plane       | Trình quản lý issue hiện đại và thân thiện người dùng |
| OpenProject | Cung cấp workflow quản lý dự án cấp doanh nghiệp |
| Linear      | Nguồn cảm hứng về trải nghiệm người dùng (UX) trong quản lý tác vụ |

### 5.6. Hệ Thống Quy Tắc & Kỹ Năng (Rule & Skill System)

**Status:** Implemented baseline / Next hardening  
**Owner docs:** This section; add a dedicated rule manual only after the rule contract stabilizes.  
**Code areas:** `server/pkg/models`, `server/internal/service`, `server/internal/orchestrator`, `web/src/` rules/skills UI  
**Blocking decisions:** Final precedence model for global vs project rules and how conflicts are reported to users.  
**Acceptance criteria:** Global rules are immutable in agent system context, project rules are injected by task/project context, conflicting local rules are rejected, and executable skills run only in isolated environments.

**Mục tiêu:** Kiểm soát hành vi của AI thông qua một kiến trúc ngữ cảnh phân lớp nghiêm ngặt (Strict Layered Context), đảm bảo tính nhất quán và an toàn.

**Hệ thống Quy tắc (Rules):**
*   **Quy tắc toàn cầu (Global Rules):** Các quy tắc cốt lõi về bảo mật và quản trị (ví dụ: không tiết lộ API key, luôn viết unit test). Các quy tắc này là bất biến, được tiêm trực tiếp vào **System Prompt** của Agent và không thể bị ghi đè.
*   **Quy tắc dự án (Local/Project Rules):** Các quy ước mã hóa và kiến trúc cụ thể của dự án (ví dụ: sử dụng Next.js, kiến trúc Hexagonal). Các quy tắc này được tiêm động vào **Task Context** tùy theo dự án và sẽ bị AI từ chối nếu xung đột với Global Rules.
*   **Cách ly thực thi (Sandboxing):** Đảm bảo mọi kỹ năng thực thi mã (code execution) phải chạy trong môi trường cô lập (Docker/SSH) để tránh rủi ro bảo mật (tham khảo từ OpenClaw).

**Kỹ năng:** Các hành động có thể tái sử dụng, giúp Agent thực hiện các tác vụ chuyên biệt.

**Ví dụ Kỹ năng:**
| Kỹ năng           | Mục đích                                       |
| :---------------- | :--------------------------------------------- |
| `run_tests`       | Thực thi các bài kiểm thử tự động              |
| `analyze_logs`    | Phân tích log CI/CD để phát hiện vấn đề        |
| `generate_docs`   | Tự động tạo tài liệu từ mã nguồn              |
| `create_migration`| Tạo migration cơ sở dữ liệu                   |

**Dự án tham khảo:**
| Dự án     | Lý do tham khảo                                |
| :-------- | :--------------------------------------------- |
| LangChain | Khung trừu tượng hóa công cụ/kỹ năng cho LLM   |
| OpenWebUI | Giao diện cấu hình mô hình/công cụ trực quan  |
| Flowise   | Nguồn cảm hứng về thiết kế workflow/kỹ năng kéo và thả |

### 5.7. Engine Workflow

**Status:** Implemented baseline / Planned multi-agent expansion  
**Owner docs:** `docs/ARCHITECTURE.md`; `docs/plans/PLAN-role-based-agents.md` for future orchestration patterns  
**Code areas:** `server/internal/workflow`, `server/internal/orchestrator`, `server/internal/sandbox`, `server/internal/gitops`  
**Blocking decisions:** Durable workflow engine choice for future scale: keep current engine, Temporal, or LangGraph-style graph orchestration.  
**Acceptance criteria:** Task can progress through analysis, planning, coding, review, fix, test, PR creation, and human review with persisted workflow state.

**Mục tiêu:** Tự động hóa các workflow kỹ thuật phức tạp, từ phân tích tác vụ đến tạo Pull Request.

**Luồng chính của Workflow:**
1.  Nhà phát triển tạo tác vụ mới kèm mô tả chi tiết.
2.  **AI agent phân tích và phân loại tác vụ** theo độ phức tạp (Easy / Medium / Hard). Agent tạo bản phân tích chuẩn Spec-driven (JSON/YAML Schema). Nếu thiếu thông tin, agent hỏi ngược lại nhà phát triển.
3.  **Phân nhánh theo độ phức tạp:**
    - 🟢 **Easy:** Agent tự động xác nhận DoR → đi thẳng bước 5.
    - 🟠🔴 **Medium/Hard:** Con người review, cập nhật và chốt final bản phân tích task (Definition of Ready gate). Không chuyển sang bước tiếp cho đến khi spec được phê duyệt.
4.  Planner agent chia nhỏ tác vụ thành các sub-task (Subagent-Driven Development). Mỗi sub-task có spec riêng (Spec-driven Development — tham khảo từ OpenSpec: `resources/OpenSpec/src/core/`).
5.  Hệ thống gán các sub-agent phù hợp và cho phép thực thi song song (Parallel Execution & Merging — tham khảo từ OpenSpec: `resources/OpenSpec/openspec-parallel-merge-plan.md` & Hermes Agent).
6.  Coding agent thực hiện viết mã nguồn (áp dụng TDD - Test-Driven Development, tham khảo từ Superpowers).
7.  Reviewer agent thực hiện review chéo (Cross-Harness Review) để đảm bảo chất lượng mã.
8.  Fix agent thử lại và sửa lỗi nếu cần.
9.  Test agent xác thực tính đúng đắn của mã nguồn.
10. Pull Request (PR) được tạo tự động.
11. Con người phê duyệt PR cuối cùng (lightweight review cho Easy, deep review cho Medium/Hard).

**Vòng lặp tự động sửa lỗi:**
*   Khi CI (Continuous Integration) thất bại, hệ thống tự động tạo tác vụ sửa lỗi, gán cho bug-fix agent và chạy lại test cho đến khi thành công.

**Dự án tham khảo:**
| Dự án     | Lý do tham khảo                                |
| :-------- | :--------------------------------------------- |
| Temporal  | Nền tảng cho workflow bền vững (durable workflows) |
| LangGraph | Khung điều phối agent dạng đồ thị linh hoạt    |
| n8n       | Công cụ tự động hóa workflow mạnh mẽ           |

### 5.8. PR & Human Review

**Status:** Implemented baseline / Planned assistant enhancements  
**Owner docs:** `docs/ARCHITECTURE.md`  
**Code areas:** `server/internal/orchestrator/pr_generator`, `server/internal/handler/pr`, `server/internal/service/task`, `web/src/` PR/review UI  
**Blocking decisions:** How much AI explanation should be generated automatically vs on reviewer request.  
**Acceptance criteria:** System creates PRs, persists PR metadata, routes tasks to human review, and supports reviewer approval before merge.

**Tính năng:**
*   **Auto PR:** Tự động tạo Pull Request với tiêu đề, tóm tắt, danh sách các file thay đổi và đánh giá mức độ rủi ro.
*   **AI PR Assistant:** Hỗ trợ reviewer bằng cách cung cấp ngữ cảnh và giải thích chi tiết khi được hỏi về các thay đổi trong PR (ví dụ: 
Reviewer có thể hỏi "Tại sao lại thay đổi logic này?" → AI giải thích ngữ cảnh PR).
*   **Chính sách Merge:** Đảm bảo mã nguồn chỉ được hợp nhất khi đã vượt qua tất cả các bài kiểm thử, được review kỹ lưỡng và có sự chấp thuận cuối cùng từ con người.

**Dự án tham khảo:**
| Dự án     | Lý do tham khảo                                |
| :-------- | :--------------------------------------------- |
| Graphite  | Nguồn cảm hứng về workflow PR hiệu quả        |
| Reviewpad | Ý tưởng về review tự động và thông minh        |
| Danger JS | Tự động hóa quy trình review trong CI          |

### 5.9. Dashboard & Analytics

**Status:** Implemented baseline / Planned observability expansion  
**Owner docs:** `docs/ARCHITECTURE.md`; `docs/plans/PLAN-unified-ai-gateway.md` for gateway telemetry  
**Code areas:** `server/internal/handler/analytics_dashboard`, `server/internal/service/analytics_dashboard`, `server/internal/handler/audit`, `web/src/` dashboard screens  
**Blocking decisions:** Langfuse/Helicone/OpenObserve integration depth vs custom telemetry only.  
**Acceptance criteria:** Dashboard shows project/task/agent status and key metrics such as success rate, retries, token usage, cost, latency, and failure reasons.

**Tính năng:**
*   **Project Dashboard:** Cung cấp cái nhìn tổng quan về các tác vụ đang hoạt động, Pull Request đang mở, các lần chạy thất bại và trạng thái của các agent.
*   **Agent Metrics:** Theo dõi các chỉ số hiệu suất của agent như tỷ lệ thành công, số lần thử lại, mức sử dụng token và thời gian hoàn thành tác vụ.

**Dự án tham khảo:**
| Dự án         | Lý do tham khảo                                |
| :------------ | :--------------------------------------------- |
| Langfuse      | Khả năng quan sát AI (AI observability) toàn diện |
| Helicone      | Nền tảng phân tích và tối ưu hóa LLM          |
| OpenObserve   | Ý tưởng về hệ thống logging và dashboard linh hoạt |

### 5.10. Lớp Gateway AI (Cross-reference)

Nội dung chi tiết của AI Gateway đã được hợp nhất vào §5.2 để tránh hai mô tả khác nhau cho cùng một capability. Khi cập nhật gateway, chỉnh §5.2 và `docs/plans/PLAN-unified-ai-gateway.md`; giữ mục này chỉ như cross-reference.

### 5.11. Tương Tác Đa Kênh (Remote Coding Sessions)

**Status:** Planned  
**Owner docs:** Create a dedicated plan before implementation.  
**Code areas:** `server/internal/handler`, `server/internal/service`, future integrations for Discord/Telegram/Slack  
**Blocking decisions:** First channel to support, auth model for chat commands, approval semantics for remote actions.  
**Acceptance criteria:** Developer can create tasks, receive progress, approve/reject actions, and inspect PR status from an authenticated remote chat session.

**Mục tiêu:** Cho phép nhà phát triển giao việc và nhận báo cáo từ AI mọi lúc mọi nơi thông qua các nền tảng nhắn tin, tăng cường khả năng cộng tác và linh hoạt.

**Tính năng:**
*   **Tích hợp Chatbot:** Tích hợp với các nền tảng nhắn tin phổ biến như Discord, Telegram, Slack để tạo thành một Multi-channel Inbox (tham khảo từ OpenClaw & Free Claude Code).
*   **Streaming tiến độ công việc:** Cập nhật tiến độ công việc trực tiếp vào kênh chat, giúp nhà phát triển nắm bắt thông tin kịp thời.
*   **Khả năng can thiệp và phê duyệt:** Cho phép nhà phát triển can thiệp vào quy trình hoặc phê duyệt PR thông qua các lệnh chat đơn giản.
*   **Hỗ trợ ra lệnh bằng giọng nói:** Chuyển đổi ghi chú giọng nói thành văn bản để AI có thể xử lý (Voice notes transcription).

## 6. Open Risks / Decisions

| Area | Decision / Risk | Current Direction |
| :--- | :-------------- | :---------------- |
| Unified AI Gateway | Frontend/API contract for provider credentials and virtual keys must stabilize before UI rebuild. | Follow `docs/plans/PLAN-unified-ai-gateway.md`; use `provider_credentials` as source of truth and `.env` only as fallback. |
| Role-Based Agents | Migration from legacy `provider`/`model`/`level` schema may require dropping or backfilling existing rows. | Treat §5.3 as planned migration target; follow `docs/plans/PLAN-role-based-agents.md`. |
| Rule contract | No dedicated rule manual exists yet, so hard links to one are invalid. | Use §5.6 as canonical rule contract until a dedicated manual is created. |
| Multi-agent orchestration | Fan-out, cross-harness review, handoff and group chat can increase merge conflicts and review complexity. | Start with sequential + fan-out + cross-harness only; keep handoff/group chat later. |
| Repository intelligence | Semantic search, dependency graph and historical failure memory need storage/indexing decisions. | Keep as later roadmap after gateway and agent migration. |
| Security & Governance | RBAC, audit logs, policy engine and sandbox isolation must be complete before enterprise deployment. | Expand audit coverage through gateway/credential lifecycle first. |

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
