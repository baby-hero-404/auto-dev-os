# Design: Feature Docs Sync

## Frontmatter convention

```markdown
---
sources:
  - server/internal/workflow/**
  - server/internal/orchestrator/steps/**
verified: 2026-07-21   # optional — set khi người review xác nhận doc đúng dù code đổi
---
```

`verified` cho phép "code đổi nhưng doc vẫn đúng" mà không phải sửa doc giả tạo: script coi `max(doc commit date, verified)` là mốc doc.

## Script logic (`scripts/docs_freshness.py`)

```
for doc in docs/features/**/*.md:
    fm = parse_frontmatter(doc)
    if not fm.sources: report.untracked.append(doc); continue
    doc_ts   = max(git_last_commit_ts(doc), fm.verified or 0)
    code_ts  = max(git_last_commit_ts(glob) for glob in fm.sources)
    if code_ts - doc_ts > THRESHOLD (30d):
        report.stale.append(doc, commits_since(doc_ts, fm.sources)[:5])
exit 0 luôn (warning mode); --strict để exit 1 (bật sau khi trả nợ untracked)
```

`--update-readme`: regenerate cột Last-verified trong bảng index của `features/README.md` (marker comments `<!-- freshness:begin/end -->` để không phá phần tay viết).

## Docs mapping cho 13 spec sets (điền vào tasks.md từng set)

| Spec set | docs/features tác động |
|----------|------------------------|
| pluggable-execution-engine | **mới** `product/14-execution-engine.md`; sửa `product/08-workflow-engine.md`, `product/06-project-system.md` |
| cli-spec-first-flow | sửa `product/08`, `product/07-task-system.md`; mới section trong `product/14` |
| llm-prompt-caching | sửa `product/01-unified-ai-gateway.md` |
| memory-hardening | sửa `engineering/01-context-management.md` (phần memory) |
| search-replace-fuzzy-fallback | sửa `product/13-patch-engine-abstraction.md` |
| definition-of-ready-gate | sửa `product/07`, `product/08` |
| review-verdict-split | sửa `product/09-pr-human-review.md`, `product/08` |
| tool-output-filtering | sửa `product/01`; note trong `engineering/02-context-pruning` |
| cross-harness-review | sửa `product/09`, `product/01` |
| repomap-mention-boost | sửa `engineering/01` hoặc `product/12-repository-profile-cache.md` (soi nội dung khi làm) |
| smart-llm-router | sửa `product/01`, `product/10-dashboard-analytics.md` |
| reusable-skills-system | sửa `product/03-skill-system.md`, `product/04-agent-system.md` |
| declarative-governance-schemas | sửa `product/08`, `product/06` |
| attestation-audit-trail | sửa `product/09`, `product/05-git-integration.md` |

## CI wiring

Job nhẹ trong workflow hiện có (chỉ cần git history — `fetch-depth: 0`) chạy script, in report vào summary. `checklist.py` gọi script ở Audit stage (theo Final Checklist Protocol có sẵn).

## Trade-offs

- Heuristic theo commit-time, không semantic diff: rẻ, zero-LLM, đủ bắt "code đổi nhiều mà doc đứng yên"; false positive xử bằng `verified:` field.
- Warning-mode trước, strict sau: tránh chặn merge khi 19 docs hiện tại chưa có frontmatter.
