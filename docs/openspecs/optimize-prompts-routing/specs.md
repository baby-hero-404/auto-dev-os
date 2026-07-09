# Specs: Prompt Optimization and JIT Skill Routing

## ADDED Requirements

### REQ-001: Modular Section Assembly Pipeline
> ✅ Status: Implemented

**Scenario:**
- **WHEN** assembling a prompt
- **THEN** it must use the `collect() -> sort() -> render()` pipeline using `PromptSection` structs.

### REQ-002: Prompt Budget Enforcement
> ✅ Status: Implemented

**Scenario:**
- **WHEN** the total token count exceeds the system max limit
- **THEN** the `BudgetOptimizer` must truncate or omit sections starting from the lowest priority (highest `Priority` integer) that are NOT marked as `IsImmutable = true` until it fits within the defined limits (e.g. Base: 300, Skills: 2500, Context: 7000).

### REQ-003: Dynamic Skill Resolver
> ✅ Status: Implemented

**Scenario:**
- **WHEN** resolving skills for a step
- **THEN** it must determine the 3-5 most relevant skills based on a combination of Role + Step + Specific Task identifiers (e.g., grpc, redis).

### REQ-004: Prompt Versioning
> ✅ Status: Implemented

**Scenario:**
- **WHEN** attempting to load `plan_v2.md`
- **THEN** if it exists, it is loaded; otherwise it seamlessly falls back to `plan.md`.

### REQ-005: Layered Rule Precedence
> ✅ Status: Implemented

**Scenario:**
- **WHEN** compiling rules for prompt generation
- **THEN** it must layer them in sequence: Global Rules -> Agent Role Constraints -> Project Rules -> Task Rules.
- **AND** it must flag Global Rules and Agent Role Constraints as `IsImmutable = true`.

### REQ-006: Global & Local Skill Merging
> ✅ Status: Implemented

**Scenario:**
- **WHEN** loading available skills for the `SkillResolver`
- **THEN** it must merge the central Global Skill Registry with the project-local skills found in `[ProjectRoot]/skills/`.

### REQ-007: Dynamic Skill Tool Registration
> ✅ Status: Implemented

**Scenario:**
- **WHEN** JIT skills are resolved for a step
- **THEN** it must extract allowed tool definitions dynamically from the frontmatter metadata inside the resolved `SKILL.md` files.
- **AND** it must register these dynamically extracted tools to be exposed to the LLM Gateway, overriding static role-based templates.

## MODIFIED Requirements

### REQ-M01: Step/Capability-Aware Context Pruning
> ✅ Status: Implemented

**Scenario:**
- **WHEN** the `ContextBuilder` assembles context for `code_backend`
- **THEN** it includes Semantic Snippets and API Contracts.
- **AND WHEN** it assembles context for `fix_backend`
- **THEN** it includes the Diff and Reviewer feedback instead of Semantic Snippets.

### REQ-M02: Strict Reviewer Context
> ✅ Status: Implemented

**Scenario:**
- **WHEN** generating context for the `reviewer` role
- **THEN** it MUST construct a context containing ONLY: Requirement, Acceptance Criteria, Coding Standards, Security Checklist, Performance Checklist, and Diff.
- **AND** it must strictly exclude Planner reasoning and original task decomposition.

## Removed Requirements
- None
