# Tasks: Prompt Optimization and JIT Skill Routing

## P0 — Critical

### Task 1.1: Core Pipeline & PromptSection Object
> Links to: REQ-001

**Acceptance Criteria:**
- [x] Define `PromptSection` struct in `server/internal/prompts/builder.go` including the `IsImmutable` field.
- [x] Implement `collect()`, `sort()`, and `render()` pipeline in `Assembler`.

### Task 1.2: Context Builder & Sliced Contexts
> Links to: REQ-M01, REQ-M02, REQ-005

**Acceptance Criteria:**
- [x] Implement `ContextBuilder` that generates separate slices (Requirement, Architecture, Diff, etc.).
- [x] Wire specific routing matrices for Planner, Backend/Frontend Coder, Reviewer, Tester, and PR Agent.
- [x] Ensure Reviewer context is strictly limited to Requirement, AC, Checklists, and Diff.
- [x] Implement layered Rules compiling (Global -> Role Constraints -> Project -> Task) and mark Global / Role Constraints as immutable.

### Task 1.3: Skill Resolver & JIT Injection
> Links to: REQ-003, REQ-006, REQ-007

**Acceptance Criteria:**
- [x] Implement `SkillResolver` that selects skills based on Step + Role + Task Keywords.
- [x] Merge global skill registries with project-local `skills/` during selection.
- [x] Extract allowed tool definitions dynamically from frontmatter of resolved JIT `SKILL.md` files.
- [x] Inject dynamic tool definitions to Gateway/LLM runners to override static templates.

### Task 1.4: Budget Optimizer
> Links to: REQ-002

**Acceptance Criteria:**
- [x] Define default token budgets per layer.
- [x] Implement logic to truncate or drop sections based on priority if the budget is exceeded, ensuring sections with `IsImmutable = true` are NEVER dropped or truncated.

## P1 — High

### Task 2.1: Step-Specific Prompt Files & Versioning
> Links to: REQ-004

**Acceptance Criteria:**
- [x] Create `plan.md`, `code_backend.md`, `code_frontend.md`, `review.md`, `test.md`, `pr.md`, `fix_review.md`, `fix_test.md`, `summarize.md` in `server/internal/prompts/steps/`.
- [x] Implement version fallback reading (`_vX.md` -> `.md`).
- [x] Add an Example Prompt to the project documentation (`docs/ai/example_prompt.md`).

### Task 2.2: Unit Tests
> Links to: REQ-001, REQ-002, REQ-003, REQ-005, REQ-006, REQ-007

**Acceptance Criteria:**
- [x] Test the pipeline sorting and rendering.
- [x] Test budget optimizer truncation logic and immutability enforcement.
- [x] Test context builder isolating slices per capability.
- [x] Test rule layering and skill merging logic.
- [x] Test dynamic tool extraction from `SKILL.md` frontmatter.

## P2 — Medium
*(none)*

## P3 — Low
*(none)*

## Docs sync

- [x] Update corresponding `docs/features/` as specified in feature-docs-sync/design.md — N/A: this spec set is not in feature-docs-sync/design.md's 14-set mapping table, no docs/features/ target specified
