Analyze this task and output the proposed specification as a valid JSON object.
You have access to read-only native tools to retrieve more context about the workspace files before writing your final specification.

CRITICAL LANGUAGE REQUIREMENT:
You MUST write all the human-readable text and markdown fields in the JSON object (specifically "scope", "risks", "execution_plan", "clarification_questions", "proposal_md", "specs_md", "design_md", and "tasks") using the SAME language as the task title and description provided by the user. 
For example, if the user's task description or title is in Vietnamese, all of these fields in your output JSON MUST be generated in Vietnamese. If the user's task description is in English, generate them in English. Do not mix languages.

If you do not have enough context about the current workspace files (or if the task description is generic/vague), you MUST use the "list_files" tool first to inspect the repository structure, and then "read_file" to read the relevant source files before writing your specification or generating questions.
Once you have gathered enough information and are ready to provide the final specification, output the final specification JSON matching the expected format.

You must output ONLY a valid JSON object (or inside a ```json block).
The JSON object MUST have the following structure:
{
  "complexity": "easy" | "medium" | "hard",
  "complexity_details": {
    "architecture": "low" | "medium" | "high",
    "data_migration": true | false,
    "breaking_change": true | false
  },
  "primary_category": "frontend" | "backend" | "database" | "devops" | "qa" | "security" | "documentation", // use 'documentation' if task is purely editing/creating documentation, READMEs, or markdown files
  "scope": "A clear, detailed description of the scope of the change",
  "affected_files": [
    {
      "repo": "repository name (empty string for default workspace)",
      "file": "path/to/file",
      "confidence": 0.9,
      "reason": "why this file needs to change"
    }
  ],
  "risks": ["list", "of", "potential", "risks", "and", "challenges"],
  "risks_details": [
    {
      "risk": "description of the risk",
      "probability": "low" | "medium" | "high",
      "severity": "low" | "medium" | "high" | "critical",
      "owner": "agent role (e.g. backend, frontend, qa, reviewer, planner)",
      "mitigation": "how to mitigate the risk"
    }
  ],
  "risk_domains": ["list", "of", "risk", "domains", "touched", "(e.g., 'auth', 'payment', 'security', 'data_migration', 'infra', 'rbac', 'public_api')"],
  "execution_phases": [
    {
      "phase": "Name of the phase (e.g., Phase 1: Setup, Phase 2: Core Logic)",
      "tasks": ["Actionable step 1", "Actionable step 2"]
    }
  ],
  "execution_units": [
    {
      "id": "unique_unit_id",
      "objective": "Objective of this execution unit",
      "tasks": ["Actionable step 1", "Actionable step 2"],
      "execution_profile": {
        "agent": "backend" | "frontend" | "devops" | "qa",
        "skills": ["golang-best-practices", "sqlite"]
      },
      "constraints": {
        "parallelizable": true | false,
        "max_files": 4,
        "estimated_tokens": 6000,
        "max_risk": "low" | "medium" | "high"
      },
      "dependencies": ["another_unit_id"]
    }
  ],
  "execution_boundaries": [
    {
      "module": "repository",
      "root": "internal/repository",
      "repo_name": "repo-a",
      "repository_id": "repo-a-uuid",
      "capabilities": ["modify_existing", "create_test", "create_helper", "generate_mock", "modify_exports", "add_dependency"]
    }
  ],
  "acceptance_criteria": [
    {
      "id": "AC-1",
      "type": "api | ui | logic | performance",
      "description": "developer can create tasks via POST /tasks",
      "expected": "HTTP 201"
    }
  ],
  "clarification_questions": ["questions", "if", "more", "details", "are", "needed"],
  "required_skills": ["legacy list of skill names required for this task"],
  "required_skills_map": {
    "backend": ["list of skills for backend role, e.g. golang-best-practices, database-design"],
    "frontend": ["list of skills for frontend role, e.g. react-patterns, tailwind-patterns"],
    "reviewer": ["list of skills for reviewer role"],
    "qa": ["list of skills for qa/test-engineer role"]
  },
  "proposal_md": "Markdown for proposal.md (use the template below)",
  "specs_md": "Markdown for specs.md (use the template below)",
  "design_md": "Markdown for design.md (use the template below)",
  "tasks": [
    {
      "id": "task-1",
      "depends_on": ["task-0 (leave empty if no dependencies)"],
      "complexity": {
        "architecture": "low" | "medium" | "high",
        "data_migration": true | false,
        "breaking_change": true | false
      }
    }
  ]
}

=== OPENSPEC TEMPLATE: proposal.md ===
## Why
(1-2 sentences: what problem does this solve? Why now?)

## What Changes
(Bullet list of specific changes. Mark breaking changes with **BREAKING**.)

## Capabilities
### New Capabilities
- `<name>`: <brief description>

### Modified Capabilities
- `<existing-name>`: <what requirement is changing>

## Impact
(Affected code, APIs, dependencies, systems)

=== OPENSPEC TEMPLATE: specs.md ===
Use delta operations as section headers:
## ADDED Requirements
### Requirement: <name>
<Description using SHALL/MUST language>

#### Scenario: <scenario name>
- **WHEN** <condition>
- **THEN** <expected outcome>

## MODIFIED Requirements
(Same format, include full updated content)

## REMOVED Requirements
### Requirement: <name>
**Reason**: <why removed>
**Migration**: <how to migrate>

=== OPENSPEC TEMPLATE: design.md ===
## Context
(Background, current state, constraints)

## Goals / Non-Goals
**Goals:** ...
**Non-Goals:** ...

## Decisions
(Key technical choices with rationale)

## Risks / Trade-offs
(Known limitations, format: [Risk] → Mitigation)

## Open Questions
(Outstanding decisions or unknowns)

=== GRANULARITY & COST RULES FOR EXECUTION UNITS ===
You MUST structure the execution_units array following these strict guidelines:
1. **Rule of Isolation**: Never mix frontend and backend tasks in the same unit. Keep them isolated.
2. **Phase Sizing (Rule of 3-5)**: Aim to divide the implementation into 2 to 5 units. Avoid making too many tiny units or single large monolith units.
3. **Context and File Limits**: Ensure each unit touches a maximum of 3-4 files. If a logic phase requires modifying or creating 5 or more files, split it into separate units.
4. **DAG Dependencies**: Correctly populate the `dependencies` array for each unit (e.g. database setup should be completed before creating repository endpoints).
5. **Estimate Constraints**:
   - `max_files`: Provide an accurate number of files modified/created (typically 1 to 4).
   - `max_risk`: Specify LOW, MEDIUM, or HIGH risk depending on the files changed (high risk for migrations, configs, major API exports).
   - `estimated_tokens`: Base it on the files involved (typically 4000-8000 tokens).
