# Expected Behavior: CLI Orchestrator Update

## Scenario: Hard-coded parallel execution
**When:**
- The API Orchestrator starts the `IMPLEMENTING` phase for a task that includes both frontend and backend subtasks.

**Then:**
- It spawns two separate CLI agent processes (`frontend_cli` and `backend_cli`) concurrently.
- It waits for both processes to complete successfully before moving to the `REVIEWING` phase.

## Scenario: Independent Review and Fix Pipeline
**When:**
- The implementation agents finish and the API Orchestrator transitions to `REVIEWING` (or the equivalent workflow DB state).

**Then:**
- The API Orchestrator spawns a Review Agent in read-only mode.
- If the Review Agent outputs issues in `.autocode/review.json`, the Orchestrator transitions to `FIXING` and spawns a Fix Agent.
- If `.autocode/review.json` indicates success, it transitions to `TESTING` or `MERGED`.

## Scenario: Specialized Agent Profiles
**When:**
- A parallel-group CLI agent process is spawned (frontend or backend track).

**Then:**
- The Orchestrator resolves the agent's existing `AgentRole` (`models.AgentRoleFrontend`/`AgentRoleBackend`) exactly as the API-native flow does.
- It builds the CLI instruction via the same `PromptBuilder` role-prompt path (no separate YAML profile format), so the two flows can never define "frontend agent" differently.

## Scenario: Parallel Agents Don't Collide on Shared Files
**When:**
- The frontend and backend CLI agents run concurrently and both need to touch a shared file (e.g. `package.json`).

**Then:**
- Each agent runs in its own git worktree/branch, provisioned via the existing `repoutil.Manager.SetupRoleWorktrees` (already used for FE/BE role branches elsewhere in the orchestrator).
- Changes are only merged together when `cross_review` runs, after both agents have exited successfully — neither agent can observe or overwrite the other's uncommitted changes mid-run.

## Scenario: Pull-based Context Loading via MCP
**When:**
- The CLI agent needs to understand the codebase, rules, or dependencies to complete its task.

**Then:**
- Instead of relying solely on `grep` or reading the entire repository, the agent calls specific MCP tools provided by the local API Orchestrator.
- For example, it calls `ast.query("PaymentProvider")` and the MCP server returns only the exact class interfaces and locations.
- The agent successfully builds a precise mental model of the codebase without token bloat.
## Scenario: State Recovery
**When:**
- The API Orchestrator crashes or is manually interrupted during the `IMPLEMENTING` phase.

**Then:**
- The workflow engine resumes automatically upon server restart (or via task retry).
- The Orchestrator reads the task state from the database, sees the state is `IMPLEMENTING`, and restarts the frontend/backend agents without re-running the earlier phases.

## Rules
- **Dynamic Context Invalidation**: The MCP Server must check for file changes (via `mtime` or `fsnotify`) before responding to `ast.query` or `dependency.impact`. If the CLI agent modifies a file, the MCP Server must transparently re-parse the AST for that file before returning the query result to ensure the agent never receives stale context.
- The Review Agent must NEVER modify source files (enforced by read-only filesystem limits or strict tool suppression).
- The State Machine must atomically update the workflow job state in the database upon every successful phase transition.

## Constraints
- The system must still utilize external agent runtimes (like Claude Code) via subprocess execution, rather than rewriting the core intelligence engine.
