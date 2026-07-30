# Tasks: CLI Platform Knowledge Injection (v2 — Context Materializer)

> **For agentic workers:** Use subagent-driven-development or executing-plans
> to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Materialize platform-only knowledge (relevant skills, learned_skills, task rules) as real files under `.autocode/context/` in the CLI task workspace, reusing the existing ephemeral `.autocode/` lifecycle, and point the CLI agent at it with one thin instruction paragraph — letting the agent decide what to read.

**Architecture:** See `design.md`. New `ContextFiles` field on `engine.CodeStepRequest`, new `PromptAssembler.MaterializeCLIContext()`, `RunCLIStep` gains a `contextFiles` parameter, `RunCodeStep` writes them + sets `AUTOCODE_CONTEXT_DIR`.

**Tech Stack:** Go, existing `PromptAssembler`/`resolveSkills` infrastructure, existing sandbox `.autocode/` convention.

---

## P0 — Critical (plumbing, must land together to keep the build green)

### Task 1.1: Add `ContextFiles` to `engine.CodeStepRequest`
> Links to: REQ-003

**Files:**
- Modify: `server/internal/orchestrator/engine/engine.go` (near `CaptureFiles`, ~line 55)

- [x] **Step 1: Add the field**

```go
// ContextFiles maps a path relative to .autocode/context/ to its content.
// Written by RunCodeStep alongside prompt.md, before the CLI subprocess
// spawns; torn down by the same .autocode/ cleanup. Nil/empty means no
// context/ directory is created at all (identical to today's behavior).
ContextFiles map[string]string
```

---

### Task 1.2: Write `ContextFiles` + set `AUTOCODE_CONTEXT_DIR` in `RunCodeStep`
> Links to: REQ-003

**Files:**
- Modify: `server/internal/orchestrator/engine/cli.go` (inside `RunCodeStep`, right after `hostAutocodeDir` is created at line ~255, before the deferred cleanup logic that follows)

- [x] **Step 1: Write context files under the existing autocode dir**

```go
// Right after: if err := os.MkdirAll(hostAutocodeDir, 0o755); err != nil { ... }
for relPath, content := range req.ContextFiles {
    target := filepath.Join(hostAutocodeDir, "context", filepath.FromSlash(relPath))
    if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
        return nil, fmt.Errorf("cli engine: create context dir for %s: %w", relPath, err)
    }
    if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
        return nil, fmt.Errorf("cli engine: write context file %s: %w", relPath, err)
    }
}
```

- [x] **Step 2: Set the env var, only when there is something to point at**

```go
// Alongside the existing env["CI"] / env["HOME"] assignments (~line 298-302):
if len(req.ContextFiles) > 0 {
    env["AUTOCODE_CONTEXT_DIR"] = autocodeDir + "/context"
}
```

- [x] **Step 3: Verify build**

Run: `go build ./server/internal/orchestrator/engine/...`
Expected: PASS

---

### Task 1.3: Extend `StepPromptLoader` interface with `MaterializeCLIContext`
> Links to: REQ-001, REQ-002, REQ-005

**Files:**
- Modify: `server/internal/orchestrator/steps/services.go:339-343`

- [x] **Step 1: Extend interface**

```go
type StepPromptLoader interface {
    LoadStepPrompt(stepID string) (string, error)
    MaterializeCLIContext(ctx context.Context, task models.Task, agent *models.Agent, stepID string) (map[string]string, error)
}
```

- [x] **Step 2: Add imports** (`context`, `models`) if not already present in `services.go`

---

### Task 1.4: Implement `MaterializeCLIContext` on `PromptAssembler`
> Links to: REQ-001, REQ-002

**Files:**
- Modify: `server/internal/prompts/assembler.go` (add `LearnedSkillsLister` interface, `learnedSkills` field, `WithLearnedSkillsLister` method, `learnedSkillsText` helper, and `MaterializeCLIContext`)
- Modify: `server/internal/orchestrator/steps/context_load.go` (refactor learned-skills formatting to use `prompts.RenderLearnedSkillsSection`)

- [x] **Step 1: Define `LearnedSkillsLister` interface and `RenderLearnedSkillsSection` helper**

In `server/internal/prompts/assembler.go` (or a helper file in `internal/prompts`), define:

```go
type LearnedSkillsLister interface {
    SearchActiveByText(ctx context.Context, projectID string, query string, limit int) ([]models.LearnedSkill, error)
}

func RenderLearnedSkillsSection(skills []models.LearnedSkill) string {
    if len(skills) == 0 {
        return ""
    }
    const learnedSkillsCharBudget = 8000
    var sb strings.Builder
    sb.WriteString("## Learned skills (from past tasks in this project)\n")
    for _, sk := range skills {
        section := fmt.Sprintf("### %s\n%s\n\n", sk.Title, sk.Content)
        if sb.Len()+len(section) > learnedSkillsCharBudget {
            break
        }
        sb.WriteString(section)
    }
    return sb.String()
}
```

Add `learnedSkills LearnedSkillsLister` field to `PromptAssembler` and `WithLearnedSkillsLister(ls LearnedSkillsLister) *PromptAssembler` builder method. Refactor `context_load.go:210-222` to use `prompts.RenderLearnedSkillsSection(skills)`.

- [x] **Step 2: Implement `MaterializeCLIContext` and `learnedSkillsText`**

```go
func (a *PromptAssembler) learnedSkillsText(ctx context.Context, task models.Task) string {
    if a.learnedSkills == nil {
        return ""
    }
    query := task.Title + "\n" + task.Description
    skills, err := a.learnedSkills.SearchActiveByText(ctx, task.ProjectID, query, 3)
    if err != nil || len(skills) == 0 {
        return ""
    }
    return RenderLearnedSkillsSection(skills)
}

const cliContextCharBudget = 12000

func (a *PromptAssembler) MaterializeCLIContext(ctx context.Context, task models.Task, agent *models.Agent, stepID string) (map[string]string, error) {
    files := make(map[string]string)

    skills, err := a.resolveSkills(ctx, task, agent, stepID)
    if err != nil {
        skills = nil // graceful degradation, not a hard failure (REQ-002)
    }

    var analysis models.TaskAnalysis
    _ = json.Unmarshal(task.Analysis, &analysis) // best-effort; zero value is fine if absent/invalid

    budget := cliContextCharBudget
    includedSkills := make([]string, 0, len(skills))
    for _, sk := range skills {
        if sk.Content == "" {
            continue
        }
        path := fmt.Sprintf("relevant/skills/%s.md", slugifySkillName(sk.Name))
        if budget-len(sk.Content) < 0 {
            break // lowest-scored (resolveSkills returns highest-score first) get dropped first
        }
        files[path] = sk.Content
        budget -= len(sk.Content)
        includedSkills = append(includedSkills, sk.Name)
    }

    if learned := a.learnedSkillsText(ctx, task); learned != "" && budget-len(learned) >= 0 {
        files["relevant/learned_skills.md"] = learned
        budget -= len(learned)
    }

    if len(analysis.TaskRules) > 0 {
        if raw, err := json.MarshalIndent(analysis.TaskRules, "", "  "); err == nil {
            files["relevant/task_rules.md"] = string(raw)
        }
    }

    if len(files) == 0 {
        return map[string]string{}, nil // REQ-002
    }

    manifest := map[string]any{
        "task":      map[string]any{"id": task.ID, "type": stepID},
        "available":  map[string]any{"skills": len(includedSkills), "learned_skills": files["relevant/learned_skills.md"] != "", "rules": len(analysis.TaskRules)},
        "sources":   sortedKeys(files),
    }
    manifestJSON, _ := json.MarshalIndent(manifest, "", "  ")
    files["manifest.json"] = string(manifestJSON)
    files["README.md"] = "Platform context has been materialized for this task.\n\nInspect `manifest.json` for what's available, then read whatever under `relevant/` is useful for the current step. You decide what to use — nothing here is mandatory.\n"

    return files, nil
}
```

*(`slugifySkillName`, `sortedKeys` are small helpers — implement inline or reuse existing string-sanitizing utilities already in `internal/prompts` if present; check before adding new ones.)*

- [x] **Step 3: Verify build**

Run: `go build ./server/internal/prompts/...`
Expected: PASS

---

### Task 1.5: Update `cliStepRunner.RunCLIStep` to thread `contextFiles` through
> Links to: REQ-003

**Files:**
- Modify: `server/internal/orchestrator/steps/services.go` (the `CLIStepRunner` interface, next to `RunCLIStep`)
- Modify: `server/internal/orchestrator/cli_spec_step.go:85` (`func (r *cliStepRunner) RunCLIStep(...)`)

- [x] **Step 1: Extend both the interface and implementation signature**

```go
RunCLIStep(ctx context.Context, task *models.Task, agent *models.Agent, jobID, stepID, instruction string, captureFiles []string, contextFiles map[string]string) (steps.CLIStepOutput, error)
```

- [x] **Step 2: Pass through to `engine.CodeStepRequest`**

In `cli_spec_step.go`, add `ContextFiles: contextFiles,` to the `engine.CodeStepRequest{...}` literal already being built in `RunCLIStep`.

- [x] **Step 3: Verify build**

Run: `go build ./server/internal/orchestrator/...`
Expected: PASS (will fail until Task 1.6 updates call sites — expected at this point)

---

### Task 1.6: Update mocks in ALL affected test files (do not skip any — this is the exact gap that broke v1)
> Links to: REQ-005

**Files:**
- Modify: `server/internal/orchestrator/steps/cli_analyze_test.go`
- Modify: `server/internal/orchestrator/steps/cli_spec_test.go`
- Modify: `server/internal/orchestrator/steps/cli_implement_test.go`
- Modify: `server/internal/orchestrator/steps/cli_spec_first_integration_test.go`
- Modify: `server/internal/orchestrator/engine/cli_test.go` (if it stubs `CodeStepRequest`/`CLIStepRunner` directly, adjust any struct literals that would otherwise fail to compile — check first, only touch what actually breaks)

- [x] **Step 1: Add `MaterializeCLIContext` to `mockStepPromptLoader` in each of the 4 `steps` package test files**

```go
func (m *mockStepPromptLoader) MaterializeCLIContext(ctx context.Context, task models.Task, agent *models.Agent, stepID string) (map[string]string, error) {
    return nil, nil
}
```

- [x] **Step 2: Update any mock/fake implementing `CLIStepRunner` to accept the new `contextFiles` parameter** (grep for `RunCLIStep(` across `_test.go` files in `internal/orchestrator/steps` and `internal/orchestrator` to find every call site and mock, not just the ones listed above — the list above is what's known now, verify it's exhaustive before marking this task done)

- [x] **Step 3: Verify build**

Run: `go build ./server/... && go vet ./server/...`
Expected: PASS

---

## P1 — High (wire the 3 CLI steps)

### Task 2.1: Update `cli_analyze.go`
> Links to: REQ-004

**Files:**
- Modify: `server/internal/orchestrator/steps/cli_analyze.go:56-64`

- [x] **Step 1: Materialize context and pass it through**

```go
base, err := s.prompts.LoadStepPrompt(workflow.StepCLIAnalyze)
if err != nil {
    return nil, fmt.Errorf("cli_analyze: load prompt: %w", err)
}
contextFiles, _ := s.prompts.MaterializeCLIContext(ctx, *s.rt.Task, s.rt.Agent, s.ID())
instruction := fmt.Sprintf("%s\n\n## Task\n\n### %s\n\n%s\n", base, s.rt.Task.Title, s.rt.Task.Description)
if len(contextFiles) > 0 {
    instruction += "\n" + cliPlatformContextPointer + "\n"
}

out, err := s.runner.RunCLIStep(ctx, s.rt.Task, s.rt.Agent, s.rt.JobID, s.ID(), instruction, []string{cliAnalysisCapturePath}, contextFiles)
```

- [x] **Step 2: Define the shared pointer constant** (put it in a shared file, e.g. `cli_analyze.go` or a new small `cli_context.go` in the `steps` package, since all 3 steps use the identical wording)

```go
const cliPlatformContextPointer = "## Platform Context\n\nPlatform context has been materialized for this task at `$AUTOCODE_CONTEXT_DIR`. Inspect `manifest.json` and `README.md` there, then read whatever is relevant before starting. Nothing there is mandatory — use your judgment."
```

---

### Task 2.2: Update `cli_spec.go`
> Links to: REQ-004

**Files:**
- Modify: `server/internal/orchestrator/steps/cli_spec.go:78-96`

- [x] **Step 1: Same pattern as Task 2.1** — call `MaterializeCLIContext`, append `cliPlatformContextPointer` if non-empty, pass `contextFiles` to `RunCLIStep`.

---

### Task 2.3: Update `cli_implement.go`
> Links to: REQ-004

**Files:**
- Modify: `server/internal/orchestrator/steps/cli_implement.go` (instruction-building section)

- [x] **Step 1: Same pattern as Task 2.1.**

---

## P2 — Medium

### Task 3.1: Full build and test verification

- [x] **Step 1:** `go build ./server/...` — Expected: PASS
- [x] **Step 2:** `go test ./server/internal/orchestrator/steps/... -run "TestCLI" -v -count=1` — Expected: all PASS
- [x] **Step 3:** `go test ./server/internal/prompts/... -v -count=1` — Expected: all PASS
- [x] **Step 4:** `go test ./server/internal/orchestrator/engine/... -v -count=1` — Expected: all PASS

### Task 3.2 (future, not in this iteration's scope): Layer 3 catalog/index
Deferred per `design.md`'s "Layer 3 explicitly deferred" decision — revisit only if Layer 2's top-5 relevant skills stop being sufficient (e.g. project skill count grows large enough that useful skills fall outside the top 5 often). Would add `catalog/skills.index`, `catalog/learned_skills.index` (names only) plus, later, a queryable convenience tool — see `design.md`'s note on why that tool must stay filesystem-backed, not a live API proxy.

---

## Self-Review Checklist

1. **Spec coverage:** Every REQ in `specs.md` maps to at least one task. ✅ (REQ-001→1.4, REQ-002→1.4, REQ-003→1.1/1.2, REQ-004→2.1-2.3, REQ-005→1.6)
2. **Placeholder scan:** No "TBD", "TODO", or vague descriptions remain. ✅
3. **Type consistency:** Types, method signatures, and names match across tasks. ✅
4. **File paths:** Every path is exact and verified against current code (line numbers as of 2026-07-29; re-check before implementing if the repo has moved on). ✅
5. **Test coverage gap from v1 explicitly closed:** all 4 `steps` package test files + `engine/cli_test.go` enumerated in Task 1.6, with an explicit instruction to grep for any call site not yet known. ✅
