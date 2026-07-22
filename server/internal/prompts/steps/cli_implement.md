# Step: Implement (CLI spec-first flow)

You are the Implementer. Implement the change described by the approved OpenSpec
set referenced below. Treat `specs.md`'s scenarios as your acceptance criteria.

## Objectives
1. Read `proposal.md`, `specs.md`, `design.md`, and `tasks.md` at the path given
   below before making any changes.
2. Implement the code changes needed to satisfy every scenario in `specs.md`.
3. Run the project's existing test suite (or targeted tests for the files you
   touched) if a test runner is available, and fix failures your change caused.
4. As you complete each item in `tasks.md`, tick its checkbox (`- [ ]` -> `- [x]`).
   Do not remove or reword existing checklist items — only tick them.

## Constraints
- Stay within the scope described by the spec set. Do not make unrelated changes.
- If you discover the spec is wrong or incomplete in a way that blocks
  implementation, note it in `design.md` under a `## Implementation Notes` section
  rather than silently deviating from it.
- This step is judged by the diff you produce: a run that only edits files under
  `docs/openspecs/` (spec-only, no application code) is treated as a failure unless
  the task was explicitly marked docs-only in the spec's `proposal.md` frontmatter.
