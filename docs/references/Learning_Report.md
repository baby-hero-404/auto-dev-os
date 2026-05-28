# Tài Liệu Phân Tích & Hướng Dẫn Học Hỏi Từ Các Dự Án Mã Nguồn Mở

Tài liệu này tổng hợp phân tích các dự án tiêu biểu trong thư mục `resources/` thuộc lĩnh vực điều phối AI Agent và tự động hóa SDLC. Tài liệu cung cấp cái nhìn sâu sắc về cách các dự án được thiết kế, các vấn đề cốt lõi cần giải quyết, bài học kinh nghiệm, và đường dẫn (paths) cụ thể tới các thành phần mã nguồn để bạn dễ dàng nghiên cứu.

## 1. Multica (Nền tảng quản lý Multi-Agent)

**Mô tả:** Multica là một nền tảng mã nguồn mở chuyên quản lý và điều phối các AI agent trong môi trường lập trình. Nó hoạt động như một "người quản lý dự án" cho các agent (Claude Code, Codex, OpenClaw, Hermes...), giúp giao nhiệm vụ, theo dõi tiến độ và tái sử dụng kỹ năng.

**Tính năng nổi bật & Kiến trúc:**
*   **Kiến trúc Self-hosted Multi-Agent:** Backend Go, Frontend Next.js 16, hỗ trợ triển khai qua Docker Compose.
*   **Vector Database & Memory:** Sử dụng PostgreSQL kết hợp `pgvector` để lưu trữ memory và context của agent.
*   **Daemon & CLI:** Thiết kế CLI và background daemon để chạy các agent script cục bộ.
*   **Tích hợp Git Native:** AI tự động phân tích repo, tạo nhánh, và sinh Pull Request/Merge Request trực tiếp trên GitHub/GitLab.

**Đường dẫn (Paths) để nghiên cứu:**
*   `resources/multica/server/`: Backend (Go), API, routing, task assignment.
*   `resources/multica/packages/` & `resources/multica/apps/`: Frontend (Next.js) và chia sẻ UI components.
*   `resources/multica/docker/` & `docker-compose.selfhost.yml`: Đóng gói môi trường tự deploy.
*   `resources/multica/CLI_AND_DAEMON.md`: Phân tích thiết kế CLI.

**Deep Dive Kỹ thuật (Technical Insights):**
*   **Main Flow (Luồng thực thi chính):** 
    1. Người dùng tạo Issue (qua Web, CLI, hoặc Quick Create).
    2. Backend gọi `EnqueueTaskForIssue` đưa task vào hàng đợi.
    3. Multica Daemon chạy ngầm trên máy developer nhận tín hiệu (qua WebSockets), gọi `ClaimTask` để nhận việc.
    4. Daemon khởi chạy AI Agent cục bộ để đọc codebase và giải quyết Issue.
    5. Agent tự động commit code, đẩy nhánh lên Git provider và tạo Pull Request.
    6. Webhook (từ GitHub/GitLab) báo về Backend, đánh dấu task `Completed` và cập nhật Analytics.
*   **Task Service (`server/internal/service/task.go`):** Quản lý trạng thái task cực kỳ chặt chẽ. Cơ chế `ClaimTask` đảm bảo cấp phát task an toàn, ngăn chặn race condition. Cập nhật Analytics liên tục với `WorkspaceID`, `IssueID`, `ChatSessionID` để tracking chi phí/ngữ cảnh.
*   **Quick Create Context:** Cho phép người dùng nhập mô tả tự nhiên, agent tự động chuyển thành context JSON (`QuickCreateContext`) và mapping vào đúng Project/Squad mà không cần qua form phức tạp.

## 2. OpenClaw (Local-first AI Assistant)

**Mô tả:** OpenClaw là một multi-channel AI gateway thiết kế local-first. Cung cấp một control plane duy nhất cho các phiên làm việc, kênh giao tiếp, công cụ và sự kiện.

**Tính năng nổi bật & Kiến trúc:**
*   **Main Flow (Luồng thực thi chính):**
    1. Tin nhắn/Sự kiện đến từ các kênh giao tiếp (Slack, Discord, Terminal).
    2. Gateway (Control Plane) chuẩn hóa tin nhắn và định tuyến (routing) đến đúng Agent Session.
    3. Agent xử lý ngữ cảnh, đưa ra quyết định gọi Tool/Skill.
    4. Gateway khởi tạo môi trường cách ly (Sandbox) cấu hình sẵn quyền hạn để chạy Tool.
    5. Kết quả thực thi Tool trả về cho Agent, Agent tổng hợp thành câu trả lời.
    6. Gateway gửi câu trả lời ngược lại kênh giao tiếp ban đầu.
*   **Local-first Gateway & Control Plane:** Xử lý sự kiện và định tuyến multi-agent (workspace + per-agent sessions).
*   **Sandboxing (Cách ly thực thi):** Chạy môi trường cách ly (Docker, SSH, OpenShell) với phân quyền nghiêm ngặt để đảm bảo an toàn khi agent chạy code lạ.
*   **Multi-channel Inbox:** Tích hợp Slack, Discord, Telegram, WhatsApp...
*   **Hệ thống Skills & Tools:** Định nghĩa input/output chuẩn cho công cụ của AI.

**Đường dẫn (Paths) để nghiên cứu:**
*   `resources/openclaw/packages/`: Core logic, gateway, và thiết lập sandboxing.
*   `resources/openclaw/extensions/`: Plugin kết nối các kênh chat.
*   `resources/openclaw/skills/`: Khai báo kỹ năng để agent hiểu và gọi.

## 3. AI-SDLC Framework (Tự động hóa SDLC bằng AI)

**Mô tả:** Framework tập trung vào điều phối AI coding agent tự động trong toàn bộ vòng đời phát triển phần mềm, từ khi nhận issue đến khi tạo PR, với sự giám sát chặt chẽ qua Quality Gates.

**Tính năng nổi bật & Kiến trúc:**
*   **Autonomous SDLC Pipeline:** Vòng lặp `WATCH -> ASSESS -> ROUTE -> EXECUTE -> VALIDATE -> DELIVER -> LEARN`.
*   **Cross-Harness Review:** Hai hoặc nhiều AI Agents (vd: Claude và Codex) kiểm tra chéo code của nhau, ghi lại chứng thực (attestation).
*   **Worktree Isolation (Pattern-C):** Mỗi agent chạy trên một bản sao worktree riêng biệt để không gây xung đột.
*   **Governance & Definition of Ready (DoR):** Policy engine kiểm tra task trước khi cho AI code. Ghi lại kết quả vào Autonomy Ledger để đánh giá độ tin cậy của agent.

**Đường dẫn (Paths) để nghiên cứu:**
*   `resources/ai-sdlc/orchestrator/`: Trái tim quản lý state và pipeline.
*   `resources/ai-sdlc/spec/` & `resources/ai-sdlc/conformance/`: Định nghĩa Rule, Policy, Quality Gates.
*   `resources/ai-sdlc/dashboard/`: Giao diện TUI để theo dõi agent.

**Deep Dive Kỹ thuật (Technical Insights):**
*   **Main Flow (Luồng thực thi chính):**
    1. `WATCH`: Orchestrator nhận event (Jira/GitHub Issue tạo mới).
    2. `ASSESS`: Phân tích độ phức tạp của Issue và quét kiến trúc codebase ra `CodebaseProfile` (Context).
    3. `ROUTE`: Routing qua Policy Engine (Kiểm tra Definition of Ready). Nếu đạt, assign cho Agent phù hợp.
    4. `EXECUTE`: Agent chạy code trên một `Worktree` riêng biệt.
    5. `VALIDATE`: Gọi một AI khác (Cross-Harness) kiểm tra chéo đoạn code vừa viết.
    6. `DELIVER`: Nếu Pass, tạo PR. Nếu Fail, trả lại bước Execute (Fix agent).
    7. `LEARN`: Ghi nhận kết quả vào `AutonomyTracker` (sổ cái tự chủ) để đánh giá năng lực agent.
*   **Orchestrator Core (`orchestrator.ts`):** Quản lý luồng chạy qua các plugin. Tích hợp sẵn `AutonomyTracker` (đánh giá độ độc lập/tin cậy của agent) và `CostTracker` (quản trị chi phí token).
*   **State Store:** Sử dụng SQLite để lưu lại toàn bộ ngữ cảnh (pipeline run, episodic record) cho phép khôi phục pipeline (checkpointing) nếu server crash giữa chừng.
*   **Codebase Analyzer:** Tự động phân tích project ra đồ thị phụ thuộc (`moduleGraph`), phát hiện hotspots và các pattern kiến trúc, lưu vào `CodebaseProfile` để làm context cho AI.

## 4. 9router (Local AI Gateway)

**Vấn đề cần học hỏi:**
*   **Main Flow (Luồng thực thi chính):**
    1. AI Client (Cursor/Claude Code) gửi request đến Local Endpoint của 9router.
    2. Đi qua **RTK Token Saver**: tự động phát hiện `tool_result` (như git diff, ls) và nén nội dung lại.
    3. Định dạng lại Request (Format Translation) sao cho khớp với Provider đích.
    4. Gửi đến Tier 1 (Subscription). Nếu hết Quota hoặc lỗi, tự động Fallback xuống Tier 2 (Cheap) và Tier 3 (Free).
    5. Trả kết quả về Client như bình thường.
*   **Proxy Pattern:** Cách dựng server trung gian hứng request từ IDE (Cursor/Cline) để can thiệp vào payload.
*   **Token Saver (RTK):** Thuật toán cắt giảm token (git diff, grep, ls) trước khi gửi lên LLM mà không làm mất context.
*   **Fallback Routing:** Cơ chế tự động chuyển model khi hết tiền/lỗi.

**Đường dẫn (Paths) để nghiên cứu:**
*   `resources/9router/src/`: Core logic xử lý proxy, fallback model.
*   `resources/9router/open-sse/`: Implement Server-Sent Events.

**Deep Dive Kỹ thuật (Technical Insights):**
*   **RTK Token Saver:** Cực kỳ quan trọng. Nó hoạt động *trước* khi gửi lên LLM, giúp tiết kiệm 20-40% số lượng input token bằng cách tự nén các output dài dòng của terminal/tool.
*   **Format Translation:** Nó có khả năng dịch "on-the-fly" (thời gian thực) giữa OpenAI, Anthropic, Gemini, Cursor API... giúp bạn dùng bất kỳ model nào cho bất kỳ IDE nào.
*   **Smart 3-Tier Fallback:** Chiến lược chia làm 3 tầng: Đã đăng ký trả phí -> Rẻ -> Miễn phí. Giúp không bao giờ bị gián đoạn công việc (Zero downtime).

## 5. OpenSpec (Spec-driven Agent Orchestration)

**Vấn đề cần học hỏi:**
*   **Main Flow (Luồng thực thi chính):**
    1. Người dùng đưa ra yêu cầu cao cấp.
    2. System chuyển yêu cầu thành bản đặc tả (Spec) chi tiết, cấu trúc theo chuẩn JSON/YAML Schema.
    3. Dựa trên Spec, hệ thống chia nhỏ (Decompose) thành nhiều Sub-task.
    4. Điều phối song song (Parallel Dispatch) các sub-agent giải quyết từng phần độc lập.
    5. Sử dụng Merge Strategy an toàn để gộp kết quả của nhiều nhánh chạy song song lại.
    6. Xác thực (Validate) kết quả cuối cùng đối chiếu 1-1 với JSON Schema ban đầu trước khi nghiệm thu.
*   **Spec-driven Development:** Dùng Schema (JSON/YAML) chuẩn để giao nhiệm vụ. Ràng buộc cấu trúc input/output của agent.
*   **Parallel Execution & Merging:** Thiết kế chạy song song nhiều agent trên các module khác nhau và chiến lược hợp nhất kết quả an toàn.

**Đường dẫn (Paths) để nghiên cứu:**
*   `resources/OpenSpec/schemas/`: Định nghĩa cấu trúc dữ liệu bắt buộc (Schemas).
*   `resources/OpenSpec/src/`: Trình xử lý xác thực (Validation) kết quả.
*   `resources/OpenSpec/openspec-parallel-merge-plan.md`: Tài liệu thiết kế chạy song song và merge.

## 6. AgentMemory (Bộ nhớ cho AI)

**Vấn đề cần học hỏi:**
*   **Episodic Memory (Trí nhớ dài hạn):** Lưu trữ lịch sử và quyết định kiến trúc để agent không lặp lại lỗi cũ.
*   **RAG (Retrieval-Augmented Generation):** Trích xuất ngữ cảnh code (Vector Search) dựa trên yêu cầu.
*   **Main Flow (Luồng thực thi chính):**
    1. Hook `SessionStart` kích hoạt: Tải project profile (khái niệm chính, file, mẫu) -> Hybrid search -> Tính toán ngân sách token (Token budget) -> Bơm (Inject) vào context của cuộc hội thoại.
    2. Trong phiên (phiên dịch/code): Hook `PostToolUse` ghi nhận mọi hành động -> Xóa trùng lặp (SHA-256 dedup) -> Lọc dữ liệu nhạy cảm (Privacy filter) -> Lưu raw observation.
    3. Trích xuất nền (Background): LLM nén (compress) observation thành dạng cấu trúc (fact, concept, narrative) -> Chuyển thành Vector embedding -> Index vào BM25 & Vector DB.
    4. Khi phiên kết thúc (Hook `SessionEnd`): Tổng hợp phiên -> Trích xuất Knowledge Graph.

**Đường dẫn (Paths) để nghiên cứu:**
*   `resources/agentmemory/`: Mã nguồn gốc, xem `src/` để thấy kiến trúc (hooks, prompts, state).

**Deep Dive Kỹ thuật (Technical Insights):**
*   **Cơ chế 4-Tier Memory (Mô phỏng bộ nhớ con người):** 
    - *Working*: Các raw observation từ việc sử dụng tool (trí nhớ ngắn hạn).
    - *Episodic*: Các bản tóm tắt phiên đã được nén (biết "điều gì đã xảy ra").
    - *Semantic*: Các fact và pattern được trích xuất (biết "kiến thức").
    - *Procedural*: Quy trình và quyết định (biết "làm thế nào").
*   **Cơ chế quên (Auto-forgetting):** Dữ liệu phai nhòa theo thời gian (Ebbinghaus curve). Các ký ức truy cập nhiều sẽ được củng cố. Các mâu thuẫn tự động được phát hiện và giải quyết.
*   **Triple-stream Retrieval (Tìm kiếm 3 luồng):** Kết hợp BM25 (từ khóa), Vector (ngữ nghĩa) và Graph (đồ thị tri thức), được hợp nhất bằng thuật toán RRF (Reciprocal Rank Fusion) để RAG chính xác nhất (đạt tỷ lệ 95.2% Recall@5).

## 7. Superpowers (Methodology & Skills Framework)

**Vấn đề cần học hỏi:**
*   **Workflow Methodologies:** Các bước thực thi rõ ràng: Brainstorming -> Planning -> TDD -> Executing -> Code Review.
*   **Subagent-Driven Development:** Chia nhỏ task cho subagent, kết hợp review 2 bước.
*   **Test-Driven Development (TDD):** Ép buộc agent tuân thủ RED-GREEN-REFACTOR.
*   **Skill Plugins System:** Đóng gói kỹ năng bằng Markdown để tái sử dụng.

**Đường dẫn (Paths) để nghiên cứu:**
*   `resources/superpowers/skills/`: Các luồng kỹ năng (TDD, systematic-debugging) được định nghĩa chi tiết bằng Markdown.
*   `resources/superpowers/README.md`: Triết lý thiết kế (Quy trình chuẩn quan trọng hơn dự đoán).

**Deep Dive Kỹ thuật (Technical Insights):**
*   **Main Flow (Luồng thực thi chính - 7 bước nghiêm ngặt):**
    1. `brainstorming`: Mở đầu bằng việc Socratic Q&A với dev, thống nhất thiết kế và chốt Spec.
    2. `using-git-worktrees`: Tự động tạo nhánh mới, đảm bảo môi trường cách ly, chạy test baseline để chắc chắn codebase không bị lỗi trước khi sửa.
    3. `writing-plans`: Phân rã Spec thành các sub-task cực nhỏ (chỉ 2-5 phút code/task).
    4. `subagent-driven-development`: Giao các sub-task cho agent xử lý.
    5. `test-driven-development`: Ép buộc tuân thủ RED-GREEN-REFACTOR (viết test fail -> viết code qua test -> refactor). Nếu viết code trước test, bắt xóa đi làm lại.
    6. `requesting-code-review`: AI tự động kiểm tra chéo (Code Review) đối chiếu với Plan ban đầu.
    7. `finishing-a-development-branch`: Nghiệm thu, dọn dẹp worktree và đẩy Pull Request.
*   **Pipeline ép buộc:** Agent phải hoàn thành bước trước mới được đi tiếp. Không được nhảy cóc.
*   **Skill Plugins System:** Thay vì viết code logic, hệ thống định nghĩa skill hoàn toàn bằng Markdown (prompt engineering), giúp nó portable (chạy được trên Claude Code, Cursor, Copilot CLI, v.v.).

## 8. Hermes Agent (Self-improving AI Agent)

**Mô tả:** AI agent của Nous Research với vòng lặp tự học tập tích hợp.

**Vấn đề cần học hỏi:**
*   **Main Flow (Luồng thực thi chính):**
    1. Nhận yêu cầu từ Terminal hoặc Messaging (Telegram/Discord) / Lịch trình Cron.
    2. Tra cứu kiến thức (Memory / Skills).
    3. Agent lên plan, phân quyền cho Sub-agents hoặc gọi tool Python qua RPC.
    4. Hoàn thành task. Nếu task phức tạp, hệ thống kích hoạt **Autonomous Skill Creation** để viết một "kỹ năng" mới cho lần sau.
    5. Nén toàn bộ quá trình (trajectory) vào Database để tự cải thiện trong tương lai.
*   **Self-improving Learning Loop:** Khả năng tự học, tạo và cải thiện kỹ năng từ kinh nghiệm. Tự động duy trì kiến thức.
*   **Episodic Memory & Cross-session Recall:** Tìm kiếm (FTS5) lịch sử, tóm tắt bằng LLM để nhớ qua nhiều phiên.
*   **Multi-backend Execution:** Chạy trên nhiều môi trường (Local, Docker, Modal, Vercel Sandbox) và kênh chat.
*   **Delegation & Parallelization:** Sinh sub-agent chạy song song, giao tiếp qua RPC.

**Đường dẫn (Paths) để nghiên cứu:**
*   `resources/hermes-agent/`: Tìm hiểu kiến trúc vòng lặp tự học và hệ thống Agent.

**Deep Dive Kỹ thuật (Technical Insights):**
*   **Closed Learning Loop:** Đây là agent duy nhất có vòng lặp tự học hoàn chỉnh. Nó tự động tạo ra một skill (procedural memory) mới sau khi làm xong một task khó, tự động cải tiến skill đó ở các lần gọi sau và liên tục xây dựng một "model" (hồ sơ) về cách làm việc của User.
*   **Remote Execution & Cron:** Agent tách rời hoàn toàn khỏi máy tính cá nhân. Có thể chạy trên VPS 5$, chờ lệnh qua Telegram/Discord, và có thể lên lịch (Cron) tự động thực thi task ban đêm (VD: nightly backup).
*   **Subagent Parallelization:** Chạy các agent con trong các process/môi trường cách ly để làm task song song, gọi các Python RPC tool nhằm giảm tối đa lượng context LLM phải đọc.

## 9. Free Claude Code (Anthropic API Proxy Gateway)

**Vấn đề cần học hỏi:**
*   **Main Flow (Luồng thực thi chính):**
    1. Claude Code CLI gửi API Anthropic đến Proxy (thay vì gửi thẳng lên Anthropic).
    2. Proxy (`fcc-server`) hứng Request, chuẩn hóa lại format nếu cần.
    3. Định tuyến (Routing) sang Provider tương ứng: NVIDIA NIM, OpenRouter, hoặc LLMs chạy Local (Ollama/llama.cpp).
    4. Nhận kết quả và stream ngược lại cho Claude Code.
*   **Drop-in Proxy & Model Routing:** Điều hướng traffic của Claude Code tới nhiều Provider (Local LLMs, DeepSeek, NVIDIA NIM...).
*   **Protocol Normalization:** Chuẩn hóa định dạng từ OpenAI sang Anthropic Messages.
*   **Remote Coding Sessions:** Tích hợp bot Discord và Telegram để điều khiển lập trình từ xa.

**Đường dẫn (Paths) để nghiên cứu:**
*   `resources/free-claude-code/api/` & `resources/free-claude-code/providers/`: Định tuyến và xử lý proxy.
*   `resources/free-claude-code/core/`: Xử lý giao thức và SSE.
*   `resources/free-claude-code/messaging/`: Tích hợp bot nhắn tin.

**Deep Dive Kỹ thuật (Technical Insights):**
*   **Drop-in Proxy Gateway:** Kỹ thuật ghi đè môi trường (override `ANTHROPIC_BASE_URL`) để lừa Claude Code gọi vào Local Server mà không cần can thiệp mã nguồn gốc.
*   **Remote Coding Session & Voice:** Tích hợp Discord/Telegram Bot bọc lấy (wrap) phiên làm việc của Claude Code, cho phép User code từ xa bằng điện thoại. Thậm chí tích hợp Whisper (Speech-to-Text) để "nói" yêu cầu code thay vì gõ text.
*   **Trình tối ưu hóa Request:** Có khả năng chặn và tự xử lý các Request dò đường (Probes) nhỏ nhặt của Claude Code ngay tại local proxy để tiết kiệm latency và quota.

## 10. Prompt Base (Global Rules & Skill Registry)

**Mô tả:** Hệ thống quản lý Rule và Skill toàn cục (cài đặt ở `~/.gemini`), cung cấp các quy tắc ứng xử (behavioral rules) chung cho tất cả các project.

**Deep Dive Kỹ thuật (Technical Insights):**
*   **Tier 0 Universal Rules (`core/rules.md`):** Quy định nghiêm ngặt về Clean Code, File Dependency Awareness (buộc kiểm tra kiến trúc trước khi code), và Intellectual Integrity (yêu cầu agent phải suy nghĩ, phân tích rủi ro trước khi viết code).
*   **Token Efficiency Protocol:** Buộc agent dùng kỹ thuật Snippet-Only Reading (đọc theo StartLine/EndLine) thay vì tải toàn bộ file, và gỡ bỏ skill khỏi context ngay sau khi dùng xong (Minimal Viable Context).
*   **Tích hợp Auto Code OS:** Auto Code OS sẽ sử dụng Prompt Base như nguồn cung cấp Rule và Skill ban đầu để tự động "seed" vào database khi tạo một project mới.

---

## 11. Antigravity Awesome Skills (Thư viện Skill & Playbook cho Agent)

**Mô tả:** Antigravity Awesome Skills là một thư viện mã nguồn mở khổng lồ chứa hơn 1,470+ Agentic Skills (playbook dạng `SKILL.md`) được đóng gói sẵn để cài đặt cho các AI coding assistant (Claude Code, Cursor, Codex CLI, Gemini CLI, Antigravity...). Thư viện giúp các Agent thực hiện các tác vụ kỹ thuật lặp đi lặp lại với context rõ ràng, ràng buộc chặt chẽ và output chuẩn hóa.

**Tính năng nổi bật & Kiến trúc:**
*   **Skill Plugins System:** Định nghĩa kỹ năng hoàn toàn bằng Markdown (prompt engineering) giúp dễ dàng di chuyển và chạy trên nhiều IDE/Agent khác nhau mà không phụ thuộc vào code logic.
*   **Registry Manifest (`skills_index.json`):** Cung cấp một manifest cấu trúc dạng JSON ổn định (`id`, `path`, metadata) để các host agent tự động quét, định tuyến và load JIT (Just-In-Time) động khi cần.
*   **Phân loại rủi ro (Risk Taxonomy):** Phân loại kỹ năng theo các mức độ rủi ro (`unknown`, `none`, `safe`, `critical`, `offensive`) và danh mục (`development`, `backend`, `security`, `marketing`...) giúp lọc và cài đặt giới hạn an toàn.
*   **Cơ chế nén Context (Overload Recovery):** Hỗ trợ cài đặt chọn lọc (chỉ cài đặt một số bundle/category nhất định) hoặc thu gọn context để tránh Agent bị tràn token (token-budget limits).

**Đường dẫn (Paths) để nghiên cứu:**
*   `resources/antigravity-awesome-skills/skills/`: Thư mục chứa hàng ngàn skill đơn lẻ được tổ chức khoa học.
*   `resources/antigravity-awesome-skills/schemas/`: Chứa JSON Schema để validate manifest và index.
*   `resources/antigravity-awesome-skills/walkthrough.md`: Nhật ký bảo trì và cập nhật kỹ năng định kỳ.

**Gợi ý các nhóm Skill phù hợp với Auto Code OS:**
Dựa trên kiến trúc của Auto Code OS (Go backend, Next.js frontend, PostgreSQL + pgvector, Docker Sandbox), các nhóm skill sau cực kỳ phù hợp để đưa vào hệ thống:
1.  **Golang Development (`golang-pro`):** Quy định các best practices về concurrency, channel pattern, context cancellation, slog, net/http, clean architecture và table-driven testing cho backend Go.
2.  **Next.js & Frontend UI/UX (`nextjs-best-practices`, `frontend-design`, `ui-ux-pro-max`):** Hướng dẫn Agent code Next.js 16 App Router, React Server Components (RSC) và visual polish để tạo UI premium, mượt mà.
3.  **Database & Vector Search (`postgres-best-practices`, `postgresql-optimization`):** Ràng buộc cách thiết kế database, query optimization và sử dụng pgvector cho memory/RAG.
4.  **Sandbox & Devops (`docker-expert`, `bash-linux`):** Hướng dẫn Agent build Dockerfile tối ưu và chạy script an toàn trong sandbox.
5.  **Agent SDLC Workflow (`brainstorming`, `tdd-workflow`, `code-review-checklist`, `systematic-debugging`):** Thiết lập quy trình 7 bước nghiêm ngặt (RED-GREEN-REFACTOR, Socratic Gate, Cross-Harness Review) cho Agent khi code.
6.  **Security & Governance (`security-auditor`, `vulnerability-scanner`, `protect-mcp-governance`):** Ràng buộc kiểm tra mã nguồn, rò rỉ secret và phân quyền thực thi an toàn.

**Ứng dụng để khởi tạo dữ liệu mặc định (Roadmap §4.5):**
Chúng ta sẽ sử dụng danh sách các skill và rule này làm nguồn dữ liệu mẫu (seed data) ban đầu cho cơ sở dữ liệu Auto Code OS. Khi một Project mới được tạo ra, hệ thống sẽ tự động gieo (seed) các rule và skill mặc định này vào DB thông qua `SeederService` để Agent của project đó có thể truy vấn và sử dụng ngay lập tức.

---

## 12. Bài Học Kinh Nghiệm và Best Practices (Tổng hợp)

Khi xây dựng nền tảng AI-Native SDLC, cần lưu ý:

1.  **Kiến trúc mô-đun:** Đảm bảo các thành phần (Git, LLM, Sandboxing) độc lập, dễ dàng thay thế adapter.
2.  **Mô hình điều phối Agent rõ ràng:** Thiết lập workflow phân nhánh theo độ phức tạp (ví dụ: Easy vs Medium/Hard), áp dụng Subagent-Driven Development cho các task phức tạp.
3.  **Quản lý Context & RAG:** Cần hệ thống Knowledge Base trung tâm với cơ chế vector search để cung cấp đủ ngữ cảnh mã nguồn và lịch sử ra quyết định.
4.  **Hệ thống Skills có cấu trúc:** Áp dụng nguyên tắc "Progressive Disclosure" - chỉ cung cấp các skill cần thiết cho agent trong từng ngữ cảnh để tiết kiệm token và tránh ảo giác.
5.  **Bảo mật & Governance:**
    *   Sử dụng Worktree Isolation và Docker Sandboxing.
    *   Tích hợp Definition of Ready (DoR) gate trước khi chạy.
    *   Thực thi Policy Engine để giới hạn quyền ghi/xóa file của agent.
6.  **Human-in-the-Loop (HITL):** Xác định các điểm dừng cần thiết (phê duyệt spec, phê duyệt PR) trong workflow để đảm bảo chất lượng, có cơ chế hỏi đáp (Clarification Loop) khi agent thiếu thông tin.
