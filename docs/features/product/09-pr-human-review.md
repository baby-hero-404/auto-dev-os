---
sources:
  - "server/**"
  - "web/src/app/projects/[id]/tasks/[taskID]/components/AuditPanel.tsx"
verified: 2026-07-23
---

# 09. PR & Human Review

**Status:** 🟡 In Progress (implemented; AI PR assistant planned)  
**Owner docs:** `docs/ARCHITECTURE.md`  
**Code areas:** `server/internal/handler/pr.go`, `server/internal/orchestrator/steps/`, `server/internal/service/task.go`, `web/src/` PR/review UI  
**Blocking decisions:** How much AI explanation should be generated automatically vs on reviewer request.  
**Acceptance criteria:** System creates PRs, persists PR metadata, routes tasks to `pr_ready` then `human_review`, supports bounded review-fix loops, and only marks task complete after explicit merge.

**Mục tiêu:** AI tự động tạo Pull Request khi hoàn thành code, kèm theo tóm tắt thay đổi, đánh giá rủi ro, và kết quả test. Con người giữ quyền quyết định cuối cùng trước khi merge.

---

## A. Luồng Vận Hành

```
1. Agent hoàn thành coding + review + fix (bounded) + test → tạo PR tự động
   ├── Branch: feature/{task_id}
   ├── Tiêu đề: "AutoCodeOS: {task.title}"
   ├── Body: task info + danh sách file thay đổi + risk assessment + risk domains
   ├── Body: full test results + lint + build verification
   ├── (Planned) Labels: auto-generated (risk level, risk domains, task type, agent role)
   ├── ⚠️ Cảnh báo nếu `review_limit_exceeded: true` (vượt max_review_fix_cycles)
   └── Task status → pr_ready (task CHƯA hoàn thành)

2. Reviewer nhận PR (task status → human_review)
   ├── Xem diff + AI-generated summary
   ├── Xem risk domain tags (auth, payment, security...) nếu có
   ├── Xem cảnh báo `review_limit_exceeded` nếu có
   ├── (Planned) Hỏi AI PR Assistant nếu cần context thêm
   └── Quyết định:
       ├── ✅ Approve → Merge → Task → merged (hoàn thành)
       └── ❌ Request Changes → Task → fixing
           → Agent nhận feedback → sửa code → chạy lại test → push commit mới
           → Vòng lặp giới hạn bởi max_review_fix_cycles (mặc định: 3)
           → Vượt giới hạn → task chuyển failed hoặc escalate
```

## B. PR Template

Mỗi PR được tạo tự động theo template chuẩn:

```markdown
## AutoCodeOS: {task.title}

**Task:** #{task_id}
**Agent:** {agent.role} ({model_level})
**Complexity:** {task.complexity}

### Summary
{AI-generated tóm tắt thay đổi}

### Changes
| File | Action | Lines Changed |
|------|--------|---------------|
| ... | Modified | +12 / -3 |

### Risk Assessment
- **Level:** {low/medium/high/critical}
- **Risk Domains:** {auth, payment, data-migration, security, permissions, public-api — nếu có}
- **Reason:** {giải thích lý do đánh giá risk}
- ⚠️ **Review Limit Warning:** {hiển thị nếu review_limit_exceeded = true}

### Test Results
- ✅ Unit tests (targeted): passed/failed
- ✅ Full test suite: passed/failed
- ✅ Lint: passed
- ✅ Build verification: passed
```

## C. Review Policy theo Complexity

**Baseline:** Duyệt/từ chối PR thủ công qua UI.

**Planned Target:**

| Complexity | Review Type | Reviewer tối thiểu | Auto-merge sau Approve |
|:-----------|:-----------|:-------------------|:----------------------|
| Easy | Lightweight | 1 | Có |
| Medium | Standard | 1 | Không (merge thủ công) |
| Hard | Deep Review | 2 (cross-review) | Không |

## D. AI PR Assistant (Planned)

*   **On-demand explanation:** Reviewer hỏi AI về bất kỳ đoạn code nào → AI giải thích dựa trên task spec và code context.
*   **Risk highlighting:** AI tự động đánh dấu thay đổi có risk cao (sửa logic thanh toán, thay đổi DB schema, xóa file).
*   **Suggestion mode:** AI đề xuất cải thiện code nhưng không tự áp dụng — quyết định thuộc về reviewer.

## E. Merge Policy (Planned)

Điều kiện để được merge:
*   Tất cả test pass (targeted + full suite + lint + build).
*   Đủ số reviewer tối thiểu approve (theo complexity).
*   Không có unresolved review comments.
*   PR không conflict với target branch.
*   Nếu `review_limit_exceeded: true` → reviewer phải explicitly acknowledge warning trước khi approve.

> **PR ≠ Task hoàn thành.** Task ở trạng thái `pr_ready` cho đến khi reviewer approve. Chỉ sau merge thành công, task mới chuyển sang `merged` (hoàn thành). Xem đầy đủ vòng đời tại §07 Task System — "Vòng Đời Task" (nguồn canonical).

## F. Attestation Audit Panel (Implemented)

Tính minh bạch và bảo mật mã nguồn do AI tạo ra được củng cố bằng chữ ký điện tử (DSSE - Dead Simple Signing Envelope) thông qua **Attestation Audit Panel** trên giao diện chi tiết tác vụ (Task Detail UI):
* **Hiển thị Audit Trail:** Danh sách các chữ ký đi kèm theo từng commit (bao gồm mã hash commit rút gọn, AI thực hiện, AI review, Key ID, và Timestamp).
* **Kiểm tra trực tiếp (Lazy Verification):** Cấp badge xác thực khi chữ ký hợp lệ thông qua lazy load.
* **Chi Tiết Envelope:** Cho phép người dùng nhấn "View envelope" để xem chuỗi JSON chuẩn DSSE Pretty-printed chứng minh nguồn gốc đoạn mã đó được mã hoá bởi Agent và có track chuỗi review chéo an toàn.

---

**Dự án tham khảo:**

| Dự án | Lý do tham khảo |
|:------|:----------------|
| Graphite | Workflow PR hiệu quả |
| Reviewpad | Review tự động và thông minh |
| Danger JS | Tự động hóa review trong CI |
