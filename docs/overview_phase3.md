# Phase 3 Implementation Strategy & Decisions

This document summarizes the decisions and strategy agreed upon for implementing Phase 3 of the Auto Code OS platform.

## 1. Phased Scope Approach
We are adopting **Option A** for the implementation scope:
- **Phase 3a (Current Focus)**: Focus exclusively on building the core execution infrastructure and sequential orchestration (Tasks 1–4).
  - Task 1: Sandbox Runtime (Docker) & Network Isolation
  - Task 2: Secret Vault Management
  - Task 3: Agent Manager & Job Queue
  - Task 4: Orchestrator Core & Checkpointing
- **Phase 3b (Future Focus)**: Postpone Tasks 5–8 (DAG Workflow Engine, Prompt Assembly, API, and UI integration) until Phase 3a is fully implemented, tested, and verified.

## 2. Sandbox Runtime Reference & Technology
For Task 1 (Sandbox Runtime), we will use the **Docker SDK for Go** (`github.com/docker/docker/client`) for programmatic container lifecycle management, resource constraints, network isolation, and logs/output streaming.
- We will study the `resources/openclaw/packages/openclaw-sandbox/` implementation to adopt their containerization and execution patterns.
- Trailing commands and agent environments will run strictly in isolated Docker containers with disabled default internet access.

## 3. Implementation and Testing Strategy
We will implement the **infrastructure and plumbing first with a mock/stub agent execution**.
- State transitions (TODO → ANALYZING → PLANNING → CODING → etc.), queue dispatching, sandbox runtime execution, and DB checkpointing will be fully wired and verified.
- The actual LLM integration via `pkg/llm` will be stubbed during the initial implementation to allow comprehensive testing of the execution pipeline without burning API tokens.

## 4. Hybrid Agent System
To balance multi-tenant flexibility with developer-friendly version control:
- **Database (Identity & Registry)**: Agent identity, status (`idle`/`busy`), project assignment, LLM Provider, and Model selection are stored and managed dynamically in the PostgreSQL database. This allows Org/Project admins to configure their agent fleet and switch models at runtime from the Web UI.
- **Organization-Scoped Staff Pool**: Agents will be created/hired at the **Organization** level (scoped to `org_id` instead of being bound to a single project). This forms a global pool of reusable AI "employees" (e.g., "Developer Alice", "Reviewer Bob") for the entire organization.
- **Project Assignment Mapping**: An agent can be assigned to projects under the organization via a many-to-many join table (`project_agents`). We support two assignment strategies per agent:
  - **Auto-Join (`AUTO_JOIN`)**: The agent is automatically assigned to all existing and newly created projects within the organization.
  - **Manual (`MANUAL`)**: The agent must be explicitly assigned to specific projects via the `project_agents` mapping.
- **Git-tracked Templates (Prompts & Behaviors)**: The core system prompt instructions and agent personas (e.g., Planner, Backend, Reviewer, QA) are loaded dynamically from Git-versioned Markdown templates in the filesystem (e.g., `resources/prompt_base/antigravity/agents/*.md` or `prompt_base/agents/*.md`).
- **Prompt Assembly**: When dispatching a task, the Orchestrator reads the Markdown prompt file matching the agent's role, merges it with database rules (global and local/project-scoped), and feeds the assembled prompt to the LLM Gateway.


