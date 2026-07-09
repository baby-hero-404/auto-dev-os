# Verified Architecture Findings
> Date: 2026-07-09 | Method: Source code inspection | Status: **Verified**

This report lists only findings **confirmed to exist** in the current codebase.

---

## Finding 1: Mutable Context Between Steps (Critical)

**Source**: `TaskAnalysis` is read fresh at every prompt assembly and mutated during coding.

| Evidence | File | Line |
|---|---|---|
| `updateTaskAnalysis` adds `AffectedFiles` during coding | `steps/code_backend.go` | L361-381 |
| `PromptAssembler.collect()` re-reads `TaskAnalysis` every call | `prompts/builder.go` | L541-543 |
| `SpecHash` only protects against full spec rewrites | `orchestrator/worker.go` | L261-264 |

**Risk**: Planner and later Coder steps may execute against different analysis states.

---

## Finding 2: Repository Context Rebuilt Per LLM Call (Medium)

**Source**: Multiple systems re-index the same repository data.

| Evidence | File | Line |
|---|---|---|
| ContextEngine indexed in ContextLoad step | `steps/context_load.go` | L136-142 |
| `RetrieveContext()` called again per prompt assembly | `prompts/builder.go` | L668 |
| `ScanDirectory()` called per coding step | `steps/code_backend.go` | L193-196 |
| RepoMap regenerated per prompt assembly | `prompts/builder.go` | L694-710 |

**Risk**: Increased latency and wasted tokens on redundant indexing.

---

## Finding 3: Parser Layers Are Coupled (Medium)

**Source**: `ParseJSONMarkdown` chains 6 fallback strategies in one function with no error classification.

| Evidence | File | Line |
|---|---|---|
| Sanitize → Unmarshal → Bracket Repair → Robust → Extract chain | `llmrunner/json.go` | L9-72 |
| `RobustParseLLMResponse` extracts known keys individually | `llmrunner/json.go` | L74-103 |
| On attempt 3, falls back to `raw_content` — all structure lost | `llmrunner/runner.go` | L113 |

**Risk**: Parse errors are non-diagnostic. Root cause (truncation, schema mismatch, format) is masked.

---

## Finding 4: Retry Regenerates Full LLM Call (Medium)

**Source**: Both parse and patch retries re-invoke the entire LLM with full prompt.

| Evidence | File | Line |
|---|---|---|
| Parse retry sends entire prev response + error, re-calls LLM | `llmrunner/runner.go` | L116-125 |
| Patch retry appends error to instruction, re-runs LLM step | `steps/code_backend.go` | L307-309 |

**Risk**: Expensive (token + latency) retries that could be repaired locally.

---

## Finding 5: `collect()` Function Is Overloaded (Low-Medium)

**Source**: The prompt assembly `collect()` is 300+ lines handling 7+ concerns.

| Concern | Lines |
|---|---|
| Base/Role/Step prompts | L406-427 |
| JIT Skills resolution | L430-438 |
| Layered rules (4 tiers) | L441-529 |
| Output rules | L532-537 |
| Context slices (reviewer vs general) | L540-711 |
| Semantic retrieval | L661-676 |
| RepoMap generation | L694-710 |

**Risk**: Hard to maintain, test, or optimize individual concerns.

---

## Finding 6: Dual Prompt Representation (Low)

**Source**: Both markdown AND JSON execution manifest are injected into the same prompt.

| Evidence | File | Line |
|---|---|---|
| ProposalMD, SpecsMD, DesignMD injected as markdown | `prompts/builder.go` | L585-605 |
| Same data injected as JSON ExecutionManifest | `prompts/builder.go` | L607-642 |

**Risk**: Token waste; LLM receives same information twice in different formats.

---

## Finding 7: Dead Code from simplify-repo-path Refactor (Low)

| Evidence | File | Line |
|---|---|---|
| Unused `branch` variable | `repoutil/paths.go` | L29-32 |
| Stale comment referencing `<defaultBranch>` | `patch/applier.go` | L327 |

---

## Finding 8: No Observability on Prompt Budget Pruning (Low)

**Source**: When `optimizeBudget()` drops sections to fit the 8192 token limit, nothing is logged.

| Evidence | File | Line |
|---|---|---|
| Sections dropped silently | `prompts/builder.go` | L750-796 |

**Risk**: Debugging prompt quality issues is difficult when sections are invisibly removed.
