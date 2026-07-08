# PLAN-phase19-planner-prompt-optimization.md - Planner Pipeline & Prompt Optimization

## 1. Objective
Refactor the Auto Code OS orchestrator's planning phase by moving away from a monolithic "Spec Writer" Planner towards a **multi-agent pipeline** (Planner ➔ Architecture ➔ Spec Writer ➔ Task Generator). We will implement metadata-driven rule filtering, prompt composition (avoiding duplicate prompt content), multi-dimensional complexity rubrics, and explicit DAG dependencies for generated tasks.

## 2. Current Architecture Flaws & User Feedback
1. **String Contains Rule Filtering**: Using `strings.Contains(rule, "run tests")` is fragile. A change in wording breaks the filter.
2. **Role Prompt Duplication**: `system_prompt_backend.md`, `system_prompt_frontend.md`, etc., have 70% duplicate boilerplate.
3. **Overly Simple Complexity Rubric**: Counting files (Easy <=3, Medium 4-15) fails to capture actual complexity like Database migrations, cross-module changes, or breaking API changes.
4. **Weak Risk Assessment**: Outputting `priority` and `impact` without `probability/likelihood` and `severity`.
5. **Missing Explicit DAG**: Tasks are output in "topological order" implicitly but lack a strict `depends_on: []` schema for parallel execution.
6. **Guessed Affected Files**: Planners guess affected files without utilizing the confidence scores from semantic search contexts.
7. **JSON Bloat**: Outputting massive Markdown chunks (`proposal.md`, `design.md`, `tasks.md`) within a single JSON response burns through context tokens and creates parsing instability.
8. **Agent Overload**: The Planner currently acts as a Business Analyst, System Architect, Spec Writer, and Task Generator simultaneously.

## 3. Proposed Architecture

### 3.1 Metadata-Driven Rule Filtering
Instead of string matching, rules will be defined with YAML frontmatter specifying applicable roles.
```yaml
---
id: run-tests
roles:
  - coder
  - reviewer
---
Execute automated tests before merging.
```
The `rules.go` engine will only inject rules where the current `agent.Role` matches the `roles` array.

### 3.2 Prompt Composition
System prompts will be assembled dynamically, preventing boilerplate duplication.
**Prompt = `core.md` + `roles/<role>.md` + `steps/<step>.md` + `workspace.md` + `rules.md`**

### 3.3 Planner Pipeline Separation
Split the massive JSON output into a sequence of focused agent steps:
1. **Planner**: Outputs High-level summary and identifies required artifacts.
2. **Architecture**: Outputs `design.md` and ADRs.
3. **Spec Writer**: Outputs `spec.md` (Acceptance Criteria, NFRs).
4. **Task Generator**: Reads the artifacts and outputs the exact `tasks.json` DAG.

### 3.4 Upgraded JSON Schemas

**Artifact Identification (Planner Output):**
```json
{
    "summary": "...",
    "required_artifacts": ["design", "spec", "tasks"]
}
```

**Task Generator DAG Output:**
```json
{
  "tasks": [
    {
      "id": "task-1",
      "depends_on": [],
      "complexity": {
        "architecture": "high",
        "data_migration": true,
        "breaking_change": false
      }
    },
    {
      "id": "task-2",
      "depends_on": ["task-1"]
    }
  ]
}
```

**Affected Files Schema (Multi-Repo Aware):**
```json
{
  "repo": "api-service",
  "file": "internal/sync/service.go",
  "confidence": 0.92,
  "reason": "Contains the core SyncService logic requiring update."
}
```
*Note: Following the Phase 18 path manager refactor, planners must output explicit repository targets or repo-relative paths (`repo/file.go`) so `paths.WorkspaceToRepoRelative` can correctly map them to the proper repository checkout without ambiguity.*

**Risk Assessment:**
```json
{
  "risk": "Data loss during migration",
  "probability": "medium",
  "severity": "critical",
  "owner": "database-architect",
  "mitigation": "Backup DB before running up.sql"
}
```

**Dynamic Skill-Role Assignment Schema:**
```json
{
  "required_skills_map": {
    "backend": ["database-design", "golang-best-practices"],
    "frontend": ["react-patterns", "ux-ui-pro-max"],
    "reviewer": ["code-review-checklist"]
  }
}
```
*Note: The planner defines which skill is injected into which agent role. During prompt assembly, the tool loader maps the current agent's role to the skills declared in `required_skills_map`.*

## 4. Implementation Steps

### Phase 1: Rule Engine Refactor
- [x] 1. Update `models.Rule` to parse and store YAML frontmatter metadata (`roles`, `id`).
- [x] 2. Update `PromptAssembler` and `rules.go` to filter based on `rule.Roles` array instead of step IDs or string matching.

### Phase 2: Prompt Composition Engine (Powered by Phase 18 Path Interfaces)
- [x] 1. Remove massive `system_prompt_*.md` files.
- [x] 2. Create `prompts/core.md`.
- [x] 3. Create `prompts/roles/planner.md`, `coder.md`, etc. (containing ONLY role-specific instructions).
- [x] 4. Update `paths.PromptPaths` interface (introduced in Phase 18) to include methods like `CorePrompt() File`.
- [x] 5. Update `assembler.go` to safely load and concatenate `core + role + step` utilizing the immutable `File` value objects to ensure directory traversal safety.

### Phase 3: Update LLM JSON Schemas
- [x] 1. Redefine `TaskAnalysis` Go structs in `models` to match the new DAG `depends_on` structure, probability/severity risks, detailed complexity multi-dimensions, and `RequiredSkillsMap map[string][]string` for role-to-skill mapping.
- [x] 2. Update `prompts/steps/analyze.md` to instruct the LLM on the new multi-dimensional schema and how to allocate skills to specific roles.
- [x] 3. Update `toolDefinitionsForAgent` in `server/internal/orchestrator/prompt/tools.go` to extract and inject skills based on the agent's role from the planner's dynamic `RequiredSkillsMap` (e.g. key `backend`, `frontend`, etc.). If `RequiredSkillsMap` is not present, fall back to `isSkillMatchingRole`.

### Phase 4: Pipeline Split (Future Phase Setup)
1. Modify the `analyze` workflow step to sequence the execution (Planner -> Architect -> Spec -> Task). *Note: The full DAG pipeline execution engine changes will be detailed in a subsequent workflow plan, but the prompt schemas must be ready now.*

## 5. Verification
- `rules_test.go` confirms that `run-tests` is completely hidden from the `planner` role.
- LLM outputs `depends_on` arrays for tasks correctly.
- Prompt construction is verified to be compositional without duplicating core instructions.
