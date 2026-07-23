---
sources:
  - "server/**"
---

# 10. Dashboard & Analytics

**Status:** 🟡 In Progress (baseline implemented; observability expansion planned)  
**Owner docs:** `docs/ARCHITECTURE.md`; `docs/features/product/01-unified-ai-gateway.md` for gateway telemetry  
**Code areas:** `server/internal/handler/analytics_dashboard.go`, `server/internal/service/analytics_dashboard.go`, `server/internal/handler/audit.go`, `web/src/` dashboard screens  
**Blocking decisions:** Langfuse/Helicone/OpenObserve integration depth vs custom telemetry only.  
**Acceptance criteria:** Dashboard shows project/task/agent status and key metrics: success rate, retries, token usage, cost, latency, and failure reasons.

**Mục tiêu:** Cung cấp giao diện tổng quan trực quan để theo dõi hiệu suất AI, trạng thái task/agent, và chi phí — giúp team quản lý và tối ưu hóa quy trình phát triển bằng AI.

---

## A. Dashboard Sections

| Section | Con người thấy gì |
|:--------|:------------------|
| **Overview** | Tổng quan: active tasks, open PRs, failed runs, agent status |
| **Task Analytics** | Tỷ lệ thành công, thời gian hoàn thành trung bình, phân bổ status |
| **Agent Performance** | Từng agent: success rate, retries, token dùng, chi phí, tốc độ |
| **Gateway Usage** | Requests/ngày, token consumption theo provider, chi phí breakdown, avg latency baseline (P50/P95 planned) |
| **Audit Trail** | Timeline sự kiện: user tạo task, agent commit code, PR approved, key rotated... |

## B. Key Metrics

| Metric | Ý nghĩa | Tại sao quan trọng |
|:-------|:---------|:-------------------|
| **Task Success Rate** | % task merged / (merged + failed) | Đo lường hiệu quả tổng thể của AI |
| **Avg Cycle Time** | Thời gian trung bình từ tạo task → hoàn thành | Task quá lâu = cần cải thiện prompt hoặc workflow |
| **Token Efficiency** | Tokens dùng / task hoàn thành | Cao quá = AI đang lãng phí token |
| **Cost per Task** | Chi phí trung bình mỗi task | Budget tracking — biết tiền đi đâu |
| **Agent Retry Rate** | % phải retry | Cao = agent hoặc prompt cần cải thiện |
| **Provider Uptime** | % request thành công | Đo độ ổn định provider → quyết định fallback strategy |

## C. Project Dashboard

Trang tổng quan cấp project:

*   **Task Board:** Phân bổ task theo status (todo → analyzing → ... → merged/failed) dạng kanban hoặc bar chart.
*   **Active Agents:** Agent đang chạy, task hiện tại, model level đang dùng.
*   **Open PRs:** PR chờ review kèm risk level và thời gian chờ.
*   **Recent Failures:** Top 5 task failed gần nhất với failure reason + link đến log.
*   **Cost Summary:** Tổng chi phí theo ngày/tuần/tháng, breakdown theo provider và model level.

## D. Agent Performance

Trang phân tích hiệu suất từng agent:

*   **Performance Over Time:** Line chart: success rate, cycle time, tokens/task qua thời gian.
*   **Task History:** Lịch sử task đã xử lý với kết quả, thời gian, số retry.
*   **Cost Tracking:** Chi phí tích lũy, so sánh giữa agents cùng role.
*   **(Planned) Skill Usage:** Thống kê skill được JIT-load nhiều nhất.

## E. Audit Log

Mỗi sự kiện trong hệ thống được ghi lại đầy đủ:

| Field | Ví dụ |
|:------|:------|
| `created_at` | `2026-06-17T11:00:00Z` |
| `user_id` | `user-123` (nếu do con người thao tác) |
| `agent_id` | `agent-backend-01` (nếu do AI thao tác) |
| `action` | `task.created`, `pr.approved`, `credential.rotated` |
| `entity_type` | `task`, `agent`, `provider_credential` |
| `entity_id` | `task-456`, `agent-backend-01` |
| `details` | `{"old_status": "coding", "new_status": "testing"}` |

## F. Mở Rộng Planned

| Feature | Mô tả |
|:--------|:------|
| **Langfuse/Helicone** | Tích hợp observability platform chuyên LLM — trace từng request, prompt debugging |
| **Custom Alerts** | Cảnh báo khi metric vượt ngưỡng (vd: cost/day > $50, failure rate > 30%) |
| **Export & Reporting** | Xuất báo cáo CSV/PDF cho management |
| **Comparative Analytics** | So sánh hiệu suất giữa model levels, providers, agent roles |

---

**Dự án tham khảo:**

| Dự án | Lý do tham khảo |
|:------|:----------------|
| Langfuse | AI observability toàn diện |
| Helicone | Phân tích và tối ưu hóa LLM |
| OpenObserve | Logging và dashboard linh hoạt |
