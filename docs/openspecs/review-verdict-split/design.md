# Design: Review Verdict Split

## Output schema

```json
{
  "spec_compliance": {"verdict": "pass|fail", "violations": [{"requirement": "...", "observed": "...", "severity": "high|medium"}]},
  "code_quality":    {"verdict": "pass|fail", "issues": [{"file": "...", "line": 0, "issue": "...", "suggestion": "..."}]},
  "summary": "..."
}
```

Parser theo pattern `analyze_parser.go` (extract JSON block khỏi completion, tolerant với text bao quanh). Parse fail → `legacyVerdict(output)` giữ nguyên đường cũ (REQ-001 fallback) — đảm bảo deploy an toàn khi model trả format cũ.

## Routing (worker)

```
review done →
  parse ok?
    ├─ spec fail  → hasRepeatViolation(prev, cur)? → pause awaiting_review_escalation
    │                                else → fix(instructions = violations-first)
    ├─ quality fail only → fix(instructions = issues)   // như cũ
    └─ both pass  → test/pr
```

- `hasRepeatViolation`: normalize (lowercase, trim) rồi so token-set overlap ≥ 0.6 giữa violation.requirement của 2 cycle liên tiếp — lưu violations cycle trước trong step state (`state["review_violations"]`).
- Escalation reuse pause helper (phối hợp `definition-of-ready-gate` task 1.1).

## Fix instruction assembly

`coding_instruction.go` (đang assemble fix instruction): section mới

```
## Spec violations (MUST fix first)
1. <requirement> — observed: <observed>
## Quality issues
...
```

## UI

Review data đã lưu trong step state → task detail review panel đọc 2 verdicts; nếu state chỉ có format cũ → render single verdict (REQ-M01). Badge màu: Spec fail = đỏ đậm, Quality fail = vàng.

## Trade-offs

- Fuzzy repeat-violation detection có thể false-negative (LLM diễn đạt khác) → escalation muộn 1 cycle, chấp nhận; false-positive hiếm và chỉ gây pause sớm — an toàn.
- Cross-harness review (P3.1) sẽ đổi *ai* review; set này đổi *review nói gì* — độc lập, làm trước không cản.
