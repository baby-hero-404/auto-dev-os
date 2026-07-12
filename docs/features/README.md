# Feature Docs Index

This folder specifies Auto Code OS's features. It's organized by **genre**, because the three genres serve different readers and change at different rates:

| Folder | Genre | Audience | Content style |
|:-------|:------|:---------|:---------------|
| [`product/`](product/) | Shipped/planned user-facing features | Product, customers, onboarding | Vietnamese body, English headers, "Dự án tham khảo" prior-art table |
| [`engineering/`](engineering/) | Internal architecture RFCs | Engineers designing/reviewing the change | Vietnamese body, numbered `## N.` sections, no prior-art table |
| [`hardening/`](hardening/) | Incident-driven remediation plan | Engineers fixing a specific, investigated failure mode | Vietnamese body, Vấn Đề/Giải Pháp/Checklist per gap |

**Numbering is local to each subfolder** — each starts at `01` independently (`product/01`...`product/13`, `engineering/01`...`engineering/05`, `hardening/01`). Numbers are **not** unique across the whole `features/` tree, so a cross-reference must say which folder it means:
- **Same-folder reference:** bare `§NN` (e.g. a `product/` doc referencing another `product/` doc as `§07`).
- **Cross-folder reference:** always spell out the folder, e.g. `product/08` or `docs/features/product/08-workflow-engine.md` — never a bare `§NN` when pointing outside your own folder.

## Status Legend

| Badge | Meaning |
|:-----:|:--------|
| 🟢 **Implemented** | Shipped and stable; no major rework currently planned. |
| 🟡 **In Progress** | Baseline implemented; hardening or enhancements ongoing/planned. |
| 🔵 **Proposed** | Designed, not yet built. |
| ⚪ **Deferred** | Intentionally paused until other work stabilizes. |

Each doc states its own status once in its header (`**Status:**` line) — this legend is the single place that defines what the badges mean; individual docs should not restate the definitions.

## Product Features (`product/`)

| # | Feature | Status |
|:--|:--------|:------:|
| 01 | [Unified AI Gateway](product/01-unified-ai-gateway.md) | 🟡 In Progress |
| 02 | [Rule System](product/02-rule-system.md) | 🟡 In Progress |
| 03 | [Skill System (Git-Sync Architecture)](product/03-skill-system.md) | 🟢 Implemented |
| 04 | [Agent System (Role-Based Capability Agents)](product/04-agent-system.md) | 🟡 In Progress |
| 05 | [Git Integration](product/05-git-integration.md) | 🟢 Implemented |
| 06 | [Project System](product/06-project-system.md) | 🟡 In Progress |
| 07 | [Task System](product/07-task-system.md) | 🟡 In Progress |
| 08 | [Workflow Engine](product/08-workflow-engine.md) | 🟡 In Progress |
| 09 | [PR & Human Review](product/09-pr-human-review.md) | 🟡 In Progress |
| 10 | [Dashboard & Analytics](product/10-dashboard-analytics.md) | 🟡 In Progress |
| 11 | [Multi-Channel Interaction (Remote Coding Sessions)](product/11-multi-channel-interaction.md) | ⚪ Deferred |
| 12 | [Repository Profile Cache](product/12-repository-profile-cache.md) | 🔵 Proposed |
| 13 | [Patch Engine Abstraction](product/13-patch-engine-abstraction.md) | 🟢 Implemented |

**Canonical sources** (avoid restating these elsewhere — cross-reference instead):
- Task lifecycle (12 states) + "PR ≠ Task hoàn thành" completion policy → **product/07** Task System.
- High-risk domains (auto-approve gating) + review-fix cycle mechanics + complexity→workflow-topology mapping → **product/08** Workflow Engine.
- `max_review_fix_cycles` / `default_autonomy` / other project-level config defaults → **product/06** Project System.

## Engineering RFCs (`engineering/`)

| # | Feature | Status |
|:--|:--------|:------:|
| 01 | [Context Management Engine (Repository Map)](engineering/01-context-management.md) | 🟢 Implemented |
| 02 | [Context Pruning & Harness Independence](engineering/02-context-pruning-and-harness-independence.md) | 🟢 Implemented |
| 03 | [Global Path Management System](engineering/03-global-path-manager.md) | 🟢 Implemented |
| 04 | [Semantic Boundaries & Filesystem RBAC](engineering/04-semantic-boundaries.md) | 🟢 Implemented |
| 05 | [Execution Unit & Dynamic Scheduler DAG](engineering/05-execution-unit-dag.md) | 🟢 Implemented |

> All five were audited against the current codebase on 2026-07-12 and found **already fully built** — every mechanism each doc "proposes" already exists in `server/`. They're kept as engineering RFCs (design rationale) rather than deleted, but their status reflects reality, not their original "Proposed" framing at authoring time.

## Hardening (`hardening/`)

One root-cause investigation (`docs/reports/git_parse_debug_report.md`, gaps A–H) produced four tightly-coupled remediation plans that used to be separate files; they're merged into one document since they share a single causal chain and mostly touch the same files (`code_backend.go`, `code_frontend.go`, `fix.go`).

| # | Feature | Status |
|:--|:--------|:------:|
| 01 | [Retry, Patch & Context Hardening](hardening/01-retry-patch-context-hardening.md) | 🟢 Implemented |

> Audited 2026-07-12: every gap (A–H) this doc describes as an open problem is already fixed in the current code (git checkpoint commits, worktree-first file reads, cache bypass on retry, sliding-window retry errors, hunk validation, auto-switch to Search/Replace all confirmed present). Kept as a record of the investigation and fix rationale, not as an active work item.

## Conventions

- **Language:** body prose is Vietnamese; headers, field labels, code identifiers, and file paths are English. This matches the majority of existing docs — don't introduce a third language pattern.
- **Header block:** every doc opens with `**Status:**`, `**Owner docs:**`, `**Code areas:**`, and (product docs only) `**Acceptance criteria:**`/`**Blocking decisions:**`, followed by a one-paragraph **Mục tiêu**.
- **Don't duplicate policy across files.** If two docs would describe the same mechanism (a status table, a config default, a gating rule), pick the doc that *owns* that concept and have the other cross-reference it — see "Canonical sources" above.
- **Numbering:** never split a doc into `NNa`/`NNb` siblings — give each its own sequential number instead (e.g. Rule System and Skill System are `02` and `03`, not `02a`/`02b`). When a doc is added or removed, renumber the affected subfolder rather than leaving a gap or reusing a letter suffix.

## History

- This folder previously used `5.1`–`5.21` numbering tied to `docs/ROADMAP.md` §5, with sub-lettered `5.2a`/`5.2b` siblings. Both were dropped (2026-07): the ROADMAP coupling because ROADMAP's own list had already drifted out of sync independent of any doc reorg, and the letter-suffix scheme because a folder-local index doesn't need to preserve a legacy numbering's history — each subfolder now just counts up from `01`. `docs/ROADMAP.md` §5 now links here instead of maintaining its own duplicate list.
