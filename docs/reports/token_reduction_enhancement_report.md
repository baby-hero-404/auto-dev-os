# Report: Enhancing Prompt Base Context Engine with Advanced Token Reduction Techniques

## 1. Overview and Current State
The current `prompt_base` architecture has already taken a massive leap forward with the **Automated Context Memory Engine** (using `sqlite-vec`, `fastembed`, and hook-driven Session/PreCompact triggers). This solves the *macro* problem of long-term memory and session bloat.

However, based on the "Awesome LLM Token Reduction" list, there are significant *micro* optimizations missing. The current system still sends raw JSON, full AST-unaware code snippets, and verbose AI outputs, which rapidly consume token budgets and increase latency.

## 2. Key Learnings from the "Awesome" List

We can categorize the potential enhancements into four distinct pillars that complement the existing Context Memory Engine:

### A. Data Format Optimization (The "JSON Tax")
* **Inspiration:** `TOON` (Token-Oriented Object Notation), `Tooner`
* **Why it matters:** AI agents communicate with tools almost exclusively via JSON. JSON is notoriously token-heavy due to repeated keys, quotes, braces, and whitespace.
* **Application:** When the agent calls tools (like `grep_search` or `list_dir`), the response often contains massive arrays of JSON objects. Converting these tool schemas and outputs from JSON to a token-efficient format like TOON (or YAML) before passing them to the LLM can cut tool-call token usage by 30-60%.

### B. AST-Aware Context Reduction
* **Inspiration:** `sigmap`, `claw-compactor`, `token-reducer`
* **Why it matters:** Currently, when the `Context Memory Engine` chunks session data or code files, it likely uses naive semantic or character-based chunking. This can split functions in half or lose scope context.
* **Application:** Integrate AST (Abstract Syntax Tree) awareness into the `PreCompact` phase and file-reading tools. Instead of sending an entire file, an AST proxy can send just the function signatures (the "skeleton") and only reveal the function body when explicitly requested. `sigmap` claims up to 95% token reduction using this method.

### C. Algorithmic Prompt Compression
* **Inspiration:** `LLMLingua`, `claude-shorthand`, `leanctx`
* **Why it matters:** Human-written prompts, PR descriptions, and documentation are full of "stop words" and grammatical filler that LLMs don't actually need to understand the intent.
* **Application:** Introduce an optional "Shorthand" pipeline. Before feeding a massive `ARCHITECTURE.md` or retrieved ADR to the context, run it through a lightweight, local LLMLingua-2 model. This compresses the text by removing redundant tokens while preserving the semantic meaning, allowing us to pack 2x-3x more ADRs into the same `SessionStart` budget.

### D. Output Token Compression (Generation Cost)
* **Inspiration:** `caveman`, `scrooge-mode`, `squeez`
* **Why it matters:** Input tokens are cheap, but *Output tokens* (generation) are expensive and slow. When an agent is thinking or writing code, unnecessary verbosity costs time.
* **Application:** Implement an enforceable "Terse Mode" or "Scrooge Mode" in `core/system_prompt.md` or as a dynamic behavior. When the agent is performing repetitive tasks or background tool loops, it should drop conversational filler ("Certainly! I will now do X") and use extreme shorthand.

---

## 3. Actionable Integration Plan for `prompt_base`

To enhance the current project without breaking the existing architecture, I recommend prioritizing the following implementations:

### Phase 1: Tool Output Minimization (High Impact, Low Effort)
- **Action:** Intercept tool outputs (especially from commands like `list_dir` or `grep_search`) and convert them from JSON to a flatter, token-optimized format (like TOON or simplified Markdown tables) before injecting them into the context.
- **Where:** `tools/` wrapper scripts or the orchestrator's tool handler.

### Phase 2: "Scrooge Mode" Output Hook (Medium Impact, Low Effort)
- **Action:** Update the System Prompt or create a new `behavioral-mode` called "caveman" or "scrooge" for autonomous sub-agent loops. Force the LLM to output only raw tool calls and single-sentence thoughts when user interaction is not required.
- **Where:** `antigravity/skills/core/behavioral-modes/SKILL.md` or `core/system_prompt.md`.

### Phase 3: AST-Aware File Reading (High Impact, Medium Effort)
- **Action:** Enhance the file-reading tools to support AST skeletonization. If a file is > 200 lines, automatically return the AST skeleton (class names, function signatures) instead of the full file, along with instructions on how to query specific line ranges.
- **Where:** Create a new tool or modify the existing codebase explorer.

### Phase 4: LLMLingua Integration for Archival (Experimental)
- **Action:** Since you are already managing a local Python environment with `fastembed`, you could add `LLMLingua-2` to the `venv`. During the `PreCompact` hook, compress the transcript before archiving it.
- **Where:** `tools/memory_compact.py`

## Conclusion
Your current `Context Memory Engine` is a fantastic **macro-level** memory system. By adopting the techniques from the "Awesome LLM Token Reduction" list, you can build a **micro-level** token firewall. 

Prioritizing **Data Format Optimization (TOON)** for tool responses and **AST-aware file reading** will give you the highest immediate return on investment for `prompt_base`.


full:
Awesome LLM Token Reduction Awesome PRs Welcome License: CC0-1.0
A curated list of techniques, tools, and research for reducing LLM token usage — with a focus on AI coding assistants like Claude Code, OpenAI Codex, and GitHub Copilot.

Every prompt and response costs tokens, and coding agents burn through them fast: large files, tool output, logs, and long sessions all inflate the context window. This list collects the drop-in tools, libraries, data formats, and papers that cut tokens while keeping answers intact.

Contents
Surveys & Background
Coding-Assistant Token Savers
Prompt Compression Libraries
Token-Efficient Data Formats
Context & Memory Management
Output Compression
Research & Methods
Star History
Surveys & Background
Start here for the lay of the land before picking a technique.

Prompt Compression for Large Language Models: A Survey - Taxonomy of hard- and soft-prompt compression methods, mechanisms, and open problems.
Coding-Assistant Token Savers
Drop-in proxies, plugins, hooks, and MCP servers that cut tokens for Claude Code, Codex, Copilot, Cursor, and Aider.

claude-rolling-context - Claude Code plugin that compresses old messages while keeping recent context verbatim. Stars
claude-shorthand - LLMLingua-2 prompt-compression hook for Claude Code. Stars
ClaudeShrink - Claude Code skill that shrinks large prompts and files with LLMLingua to save tokens. Stars
engram - Local-first context compression for AI coding tools, deduping redundant tokens across calls. Stars
entroly - Local proxy that compresses context for Claude Code, Codex, Cursor, and Aider. Stars
headroom - Compresses tool output, logs, files, and RAG chunks before they reach the LLM. Stars
llmtrim - Provider-agnostic Rust proxy that compresses input, output, and cache with no extra model calls. Stars
rtk - CLI proxy that cuts LLM token use 60-90% on common dev commands, single Rust binary. Stars
sigmap - Zero-dependency MCP server for AST-based code context reduction across 31 languages. Stars
token-optimizer-mcp - Claude Code MCP server reaching 95%+ token reduction through caching and optimization. Stars
token-reducer - Local-first Claude Code context compression using hybrid RAG and AST chunking. Stars
TokenTamer - Drop-in proxy that compresses bloated code context in real time to cut API costs. Stars
tokless - Unified CLI to install and update token-saving plugins for Claude Code, Codex, and OpenCode. Stars
Prompt Compression Libraries
General-purpose SDKs you call directly to compress prompts in any LLM app.

claw-compactor - 14-stage reversible, AST-aware pipeline for LLM token compression with zero inference cost. Stars
leanctx - Drop-in prompt-compression SDK for production LLM apps, built on LLMLingua-2. Stars
LLMLingua - Microsoft toolkit compressing prompts and KV-cache up to 20x with minimal quality loss. Stars
llmlingua-2-js - JavaScript/TypeScript implementation of LLMLingua-2 for browser and Node. Stars
Token-Efficient Data Formats
Compact, LLM-friendly encodings that pass the same data in fewer tokens than JSON.

TOON - Token-Oriented Object Notation, a lossless JSON encoding that cuts tokens ~30-60% for uniform data. Stars
Tooner - MCP proxy that converts JSON tool responses to TOON before they reach the model. Stars
Context & Memory Management
Persist and retrieve only what matters, so sessions stay short instead of replaying everything.

codex-agent-mem - Local-first MCP memory layer for Codex and Claude with compact, token-saving context packs. Stars
mnemosyne - Zero-dependency knowledge compression, ingestion, and hybrid retrieval engine. Stars
Zep - Context engineering platform that assembles relationship-aware context from a temporal knowledge graph. Stars
Output Compression
Reduce generation tokens — the part you pay the most for — without losing the answer.

caveman - Claude Code skill that rewrites output in terse "caveman speak" to cut ~65% of tokens. Stars
scrooge-mode - Output-compression skill for Claude Code and Codex measured on real session output tokens. Stars
squeez - Squeezes verbose LLM agent tool output down to only the relevant lines. Stars
