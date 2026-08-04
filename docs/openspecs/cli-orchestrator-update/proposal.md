# Proposal: CLI Orchestrator Update

## Problem
Currently, the CLI flow acts as a simple "agent executor" where a single black-box agent session (e.g., Claude Code) handles the entire task end-to-end. This approach suffers from several gaps compared to the API-native flow:
- **No parallel execution**: Tasks cannot be split into concurrent frontend and backend tracks.
- **No independent review/fix loop**: The coding agent evaluates its own work, which lacks objectivity.
- **No specialized profiles**: A single general-purpose prompt is used instead of specialized agents.
- **Weak context management**: The agent relies on its own search heuristics rather than having access to structured AutoCodeOS knowledge (AST, dependency graphs, conventions).
- **No state machine persistence**: A failure in the middle of execution forces a full restart.

## Goal
Elevate the CLI flow to parity with the API-native flow by introducing a **CLI Orchestrator** layer. This layer will manage state, context, multi-agent profiles, and parallel execution, while still leveraging the speed and powerful native runtime of CLI tools like Claude Code.

## Success
The CLI flow is capable of running an independent review/fix loop, loading structured context dynamically via MCP, spawning parallel agent processes, and resuming gracefully from failures, all orchestrated via the existing AutoCodeOS API Server.

## Decisions
- **CLI DAG Runner (MVP)**: Hard-code a parallel group (`frontend` and `backend` concurrent execution) before merging to a `review` step, rather than building a fully dynamic DAG engine immediately.
- **Independent Agent Pipeline**: Separate the `implement` phase from the `review` phase. The Review Agent runs strictly in read-only mode, outputs a `.autocode/review.json`, and feeds issues back to the Fix Agent.
- **Agent Profiles**: Reuse the existing `AgentRole` constants (`pkg/models/agent.go`: `backend`, `frontend`, `reviewer`, `qa`, ...) and `PromptBuilder` instead of a new YAML profile format. The CLI path maps a role to its instruction/system-prompt exactly like the API-native path already does, so "what a frontend agent is" never diverges between the two flows.
- **Context Intelligence Layer (MCP)**: Instead of "pushing" 100s of files into the prompt (which wastes tokens and overloads reasoning), we expose an AutoCodeOS MCP Server that **bridges the AST/dependency-graph/skill engines that already exist and already power the API-native flow's `context_load` step** (`internal/context/parser`, `internal/context/symbol`, `internal/context/repomap`, `internal/context/provider`, `LearnedSkillReader`) — not a new intelligence engine. The CLI agent "pulls" context on-demand via 6 MVP tools (`repo.search`, `ast.query`, `dependency.impact`, `skill.search`, `architecture.query`, `quality.check`), each a thin wrapper over one of those existing packages. This is the **highest priority** update, as one smart agent with great context outperforms multiple agents without it.
- **State Machine**: Leverage the existing AutoCodeOS database (`tasks`, `workflow_jobs`) to track transitions (INIT → CONTEXT_READY → IMPLEMENTING → REVIEWING) and allow resuming, instead of local JSON state files.

## Trade-offs
- Adds complexity to the API Orchestrator to manage subprocesses like Claude Code as distinct DAG nodes.
- Requires standardizing CLI output contracts (e.g., `.autocode/review.json`) so the orchestrator can parse agent outputs.
- Retains the performance advantage of CLI agents but requires exposing an internal MCP server from the API to bridge AutoCodeOS intelligence to the black-box agent.

## Out of Scope
- Fully dynamic, user-defined, arbitrary DAG parsing (sticking to a hard-coded standard parallel template for Phase 1).
- Multi-agent peer-to-peer chat negotiation.

## Impact
- No new CLI binary (`server/cmd/cli` is explicitly NOT created).
- Modifications to `workflow` and `runners` on the backend to trigger the new orchestration flow instead of raw single-agent execution.
