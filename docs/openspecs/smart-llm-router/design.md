# Design: Smart LLM Router

## Routing

```go
// nơi model-level hiện được resolve (grep DefaultModelLevel callers)
func ResolveStepModelLevel(projectLevel, taskComplexity, stepID string, retryAttempt int, smartRouting bool) string
```

Bảng matrix hằng số trong code v1 (per-project override để governance P4.2):

```go
var stepBaseLevel = map[string]string{
    workflow.StepContextLoad: "fast", workflow.StepAnalyze: "fast",
    "dor_check": "fast",
    workflow.StepPlan: "balanced", workflow.StepReview: "balanced",
    // code/fix: dùng projectLevel
}
```

Thứ tự áp: base(step) → min(base, projectLevel)? Không — code steps theo projectLevel, steps khác theo base nhưng không vượt projectLevel (user chọn balanced thì không có gì chạy powerful). Complexity=easy hạ 1 bậc; retryAttempt≥2 khôi phục về trước-hạ.

## token_usage

```sql
CREATE TABLE token_usage (
  id uuid PK, task_id uuid, job_id uuid, step_id text,
  provider text, model text,
  input_tokens int, output_tokens int, cache_read_tokens int, cache_write_tokens int,
  cost_estimate numeric(10,6), created_at timestamptz
);
CREATE INDEX ON token_usage (task_id); CREATE INDEX ON token_usage (created_at);
```

Ghi async (goroutine, best-effort) từ call-site usage log của `llmrunner`. Bảng giá: `config/model_prices.yaml` `{model: {input_per_mtok, output_per_mtok, cache_read_per_mtok, cache_write_per_mtok}}` — load 1 lần, model không có giá → cost null (vẫn ghi tokens).

## Aggregate API

Query GROUP BY model / step_id trên khoảng ngày; `cache_savings = Σ cache_read × (input_price − cache_read_price)`. UI card theo pattern các stat cards hiện có ở project page.

## Trade-offs

- Routing theo bảng tĩnh + complexity, không LLM-classifier chấm độ khó per-request (9Router có): deterministic, dễ giải thích, đủ ăn phần lớn savings. Classifier là enhancement khi có số liệu.
- Cost estimate là ước tính từ bảng giá tự bảo trì — chấp nhận lệch khi provider đổi giá; hiển thị label "estimated".
