# Design: Fix Analyze Tool Path Resolution

## Context
The `AnalyzeStep` provides LLM planners with workspace visibility. Previously, the tool used manual `filepath.Join` and executed mock sandbox commands (`find .`) which returned arbitrary or out-of-scope metadata files, cluttering the LLM's context.

## Goals
- Ensure all tool read/list/search actions respect strict repository boundaries.
- Reduce LLM prompt noise.

## Decisions
- Used `pkg/paths.OSWorkspacePaths` to resolve paths consistently.
- Overridden `agent.Role` to `models.AgentRolePlanner` during `AssembleForAgent` in `AnalyzeStep` to ensure the `# Planner Role` instruction is always injected regardless of the primary assignee of the task.

## Security & Execution Boundaries
| Agent | Allowed Paths | Permissions |
|-------|---------------|-------------|
| Planner | `code/repos/<repo>/main/` | Read |
