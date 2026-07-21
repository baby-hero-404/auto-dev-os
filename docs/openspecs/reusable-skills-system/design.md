# Design: Reusable Skills System

## Data

```sql
CREATE TABLE skills (
  id uuid PK, project_id uuid NOT NULL,
  title text NOT NULL, trigger_keywords text[] NOT NULL DEFAULT '{}',
  content text NOT NULL,             -- markdown, ≤4k chars enforced
  status text NOT NULL DEFAULT 'draft', -- draft|active|disabled
  source_task_id uuid, usage_count int DEFAULT 0,
  success_count int DEFAULT 0,       -- success_rate = success_count/usage_count
  created_at, updated_at
);
```

## Extraction

Hook tại worker chỗ `DetectPatterns` (worker.go:566) nhưng trigger trên transition sang `merged` (webhook/PR-merge poll đã có? — khảo sát; nếu chưa có merged signal thì v1 dùng `Done` + note). Prompt input: step outputs tóm tắt + fix cycles + review feedback; output JSON `[{title, trigger_keywords, content}]` max 2. Dedup: so title+keywords với skills hiện có (token overlap) → trùng thì tăng evidence thay vì tạo mới.

## Loading

`context_load`: BM25 đơn giản trên `title + trigger_keywords` (dùng Postgres FTS, không cần memory infra) với query = task title+description; score threshold, top-3, render:

```
## Learned skills (from past tasks in this project)
### <title>
<content>
```

Ghi `state["skills_loaded"] = [ids]` để REQ-003 cập nhật khi kết thúc.

## Nudge (toolloop.go)

Thuần Go: đếm `map[toolName]failCount` + `map[hash(tool+args)]count` trong loop state. Tại iteration % 15 == 0 hoặc repeat-fail ≥3: append user-role message

```
[system note] Progress check: iterations=N. Failed calls: run_build ×3, search_replace ×2.
Same call repeated failing: <tool>(<args-summary>) ×3 — change approach instead of retrying.
```

Không LLM call phụ; message ngắn (<200 tokens). Đặt sau nơi tool result được append.

## Trade-offs

- FTS thay vì vector search cho skills: corpus nhỏ (chục records/project), keywords đủ; đổi sang memory infra khi corpus lớn.
- Extraction max 2 skills/task + dedup: chống skill-spam làm loãng context — chất lượng hơn số lượng (bài học Superpowers).
- Nudge cadence 15 cứng v1; nếu gây nhiễu → config sau.
