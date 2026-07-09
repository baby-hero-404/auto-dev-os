# Proposal: Prompt Optimization and JIT Skill Routing

## Why
Currently, the prompt assembler acts as a monolithic script that simply concatenates text, leading to massive token bloat and context pollution. It lacks dynamic context slicing (treating all context as one blob), hardcodes skill assignments by role instead of task needs, has no explicit token budget enforcement per layer, and does not support versioning or A/B testing of prompt files. This leads to anchoring bias in reviewers, hallucination in coders, and high inference costs.

## What Changes

### Issue 1: Modular Context Slicing (Context Builder)
- Instead of a single "Filtered Context", context will be segmented into slices: `Requirement`, `Architecture`, `Repo Summary`, `Relevant Files`, `Semantic Snippets`, `API Contracts`, `Existing Decisions`, and `Diff`.
- The `ContextBuilder` will selectively assemble these slices based on the **Agent Role + Workflow Step** (e.g., a Reviewer only sees Requirement, Acceptance Criteria, Checklists, and Diff, avoiding the Planner's reasoning).

### Issue 2: Dynamic Skill Resolver
- Skills will no longer be statically mapped just by Agent Role.
- The `SkillResolver` will determine required skills based on a combination of **Role + Step + Task Characteristics** (e.g., Task A requires `grpc`, Task B requires `redis`), injecting 3-5 relevant markdown skills (JIT Knowledge) into the prompt.

### Issue 3: Section-based Object Assembly
- The prompt assembly process will shift to an object-oriented pipeline: `collect() -> sort() -> render()`.
- Each piece of the prompt becomes a `PromptSection` struct with a `Name`, `Body`, and `Priority`. This allows seamless extension later (e.g., adding Few-shot examples or Memory) without rewriting the assembler.

### Issue 4: Prompt Budget Optimizer
- Introduce hard token budget constraints for each layer (e.g., Base: 300, Role: 400, Step: 500, Skills: 2500, Rules: 300, Context: 7000, Task: 1000).
- The `BudgetOptimizer` will truncate or drop lower-priority sections if the budget is exceeded.

### Issue 5: Prompt Versioning
- Add support for versioned step/role prompts (e.g., `plan_v2.md`, `plan_claude.md`) to allow easy A/B testing and model-specific tuning.

## Capabilities

### New Capabilities
- `PromptSection Pipeline`: Extensible array of prompt building blocks.
- `ContextBuilder`: Fine-grained context slice routing.
- `SkillResolver`: Context-aware JIT skill injection.
- `BudgetOptimizer`: Strict token management per section.
- `Prompt Versioning`: Fallback-based file loading (`<name>_<version>.md` -> `<name>.md`).

### Modified Capabilities
- `AssembleForAgent`: Re-architected into a pipeline: Base -> Role -> Step -> Skill Resolver -> Rules -> Context Builder -> Task Builder -> Budget Optimizer -> Render.

### Removed Capabilities
- None

## Impact

| Area | Files Affected |
|------|----------------|
| Prompts | `server/internal/prompts/assembler.go` |
| Prompts | `server/internal/prompts/helpers.go` |
| Prompts | `server/internal/prompts/builder.go` (new) |
| Prompts | `server/internal/prompts/steps/*.md` |
