# Step: Author Spec (CLI spec-first flow)

You are the Spec Author. Your only job in this step is to write an OpenSpec set
describing how you will implement this task — you must NOT write or modify any
application code yet. Base your spec on `.autocode/analysis.md` from the previous
step (read it first) and the task description below.

## Required output

Write exactly these 4 files into `docs/openspecs/<task-slug>/` (the task slug is
given below — use it exactly, do not invent your own):

- `proposal.md` — Why this change is needed and what changes at a high level.
- `specs.md` — Added/Modified/Removed requirements, each as a testable scenario
  (WHEN/THEN). These scenarios are the acceptance criteria the implement step will
  be judged against.
- `design.md` — The technical approach: key files/modules touched, data model or
  API changes, sequencing, and trade-offs.
- `tasks.md` — A checklist of concrete implementation steps using `- [ ]` markdown
  checkboxes. You must include at least one checkbox.

## Critical rule: docs-only tasks

If — and only if — this task requires writing specs/documentation ONLY and no
application code changes at all (e.g. "analyze and write an OpenSpec for feature X,
do not implement it"), you MUST include YAML frontmatter at the very top of
`proposal.md`:

```yaml
---
type: documentation
---
```

Do not add this frontmatter for any task that requires actual code changes — doing
so would incorrectly let an unimplemented task pass validation.

## Constraints
- Do not implement any code changes in this step.
- Only write files under `docs/openspecs/<task-slug>/`. Do not touch any other path.
