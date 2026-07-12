# 01. Retry, Patch & Context Hardening

**Status:** 🟢 Implemented (audited 2026-07-12 — see note below)
**Owner docs:** `docs/reports/git_parse_debug_report.md` — Gaps A–H
**Code areas:** `server/internal/orchestrator/worker.go`, `server/internal/orchestrator/steps/` (`code_backend.go`, `code_frontend.go`, `fix.go`), `server/internal/orchestrator/patch/` (`applier.go`, `validator.go`, `engine.go`), `server/internal/orchestrator/sandbox.go`, `server/internal/orchestrator/llmrunner/runner.go`, `server/internal/prompts/builder.go`, `server/internal/context/provider/provider.go`

**Mục tiêu:** Một cuộc điều tra sự cố duy nhất (`git_parse_debug_report.md`, gaps A–H) phát hiện ra rằng các lỗi retry/patch-apply/context-staleness quan sát được trong production đều bắt nguồn từ **cùng một chuỗi nguyên nhân liên hoàn**: worktree không có snapshot đáng tin cậy → retry loop không biết file nào đang lỗi → LLM không nhận được nội dung file mới nhất → patch sinh ra dựa trên ngữ cảnh cũ/sai → apply thất bại → retry lại tích lũy lỗi thay vì học từ lỗi trước. Tài liệu này gộp 4 nhóm giải pháp (trước đây là 4 file `5.18`–`5.21` riêng biệt) thành một sáng kiến duy nhất vì chúng chia sẻ chung một causal chain và phần lớn cùng chạm vào các file `code_backend.go` / `code_frontend.go` / `fix.go`.

> Đã sửa: bản gốc của 4 file trước khi gộp đều trỏ sai đường dẫn `docs/report/git_parse_debug_report.md` (thiếu "s"). Đường dẫn đúng là `docs/reports/git_parse_debug_report.md`.

> **Audit note (2026-07-12):** Toàn bộ 8 gap (A–H) mô tả bên dưới đã được xác nhận là **đã fix xong** trong code hiện tại — tài liệu này giờ đóng vai trò ghi lại investigation + rationale, không còn là danh sách việc cần làm. Bằng chứng cụ thể: `CreateGitCheckpoint`/`RestoreGitCheckpoint` (`repoutil/worktrees.go`) đã tạo checkpoint commit sau mỗi step và restore đúng khi resume; `applier.go:307,401` đã log lỗi revert thay vì nuốt âm thầm; `ResetRoleWorktrees` đã reset worktree trước mỗi retry (`patch_retry_loop.go`); `parseCompilerErrorFiles`/`updateAffectedFilesWithErrors` đã populate `AffectedFiles` từ compiler output; `sandbox.go:readAffectedFileContent` đã ưu tiên đọc từ worktree trước `Paths.Main`; `builder.go` đã bypass cache khi retry và boost RAG cho error files; `validator.go:ValidateHunkCounts` đã validate hunk line count; `patch_retry_loop.go` đã tự động chuyển sang Search/Replace sau 2 lần unified-diff fail. Các section Vấn Đề/Giải Pháp bên dưới được giữ nguyên làm tài liệu lịch sử của investigation.

---

## Bản Đồ Nguyên Nhân → Giải Pháp

| Gap | Vấn đề | Giải pháp | Section |
|:----|:-------|:----------|:--------|
| G | Checkpoint không snapshot file vật lý; revert lỗi bị nuốt âm thầm | Git commit mỗi checkpoint + `git reset --hard` trước retry + log revert errors | [A](#a-checkpoint--worktree-integrity-guard) |
| A | `AffectedFiles` là `null` trong retry loop → LLM nhận 0 byte context | Parse compiler/test output → populate `AffectedFiles` trước mỗi retry | [B](#b-retry-context-reconstruction) |
| D | File reader chỉ đọc `Paths.Main`, bỏ qua active worktree | Ưu tiên resolve path theo worktree trước khi fallback về main checkout | [B](#b-retry-context-reconstruction) |
| E | Context cache không refresh giữa các lần retry | Bypass cache, gọi `RetrieveContext` trực tiếp khi phát hiện retry | [C](#c-fresh-context-pipeline) |
| F | Fallback path rò rỉ absolute host path vào prompt | Drop snippet không resolve được thay vì fallback sang raw path | [C](#c-fresh-context-pipeline) |
| H | Retry instruction tích lũy toàn bộ lỗi qua các attempt (token bloat) | Sliding-window: chỉ giữ lỗi của attempt gần nhất | [D](#d-retry-optimization) |
| B+C | RAG scoring không ưu tiên file đang lỗi; hunk count không được validate | Boost RAG cho error files + validate hunk line counts trước `git apply` | [D](#d-retry-optimization) |
| — | Unified diff thất bại lặp lại không tự chuyển chiến lược | Auto-switch sang Search/Replace sau 2 lần unified diff fail | [D](#d-retry-optimization) |

---

## A. Checkpoint & Worktree Integrity Guard

**Acceptance criteria:**
- Mỗi step thành công tạo một Git commit checkpoint trong worktree.
- Resume/rollback task khôi phục worktree về đúng commit checkpoint tương ứng và re-index cache.
- Retry loop reset worktree về HEAD trước khi thử lại.
- Revert failures được log rõ ràng, không bị bỏ qua âm thầm.

### Vấn Đề Hiện Tại

**Micro-level: Silent Revert Failures.** Khi patch thất bại, `applier.go:307` (và `:398` cho multi-repo) cố gắng revert nhưng **bỏ qua lỗi âm thầm** (`_, _ = r.RunSandboxStepInWorktree(...)`). Chuỗi apply (`git apply` → `patch --batch`) cho phép `patch` apply từng hunk riêng lẻ — nếu một số hunk thành công và một số thất bại, file sẽ bị **truncated/corrupted**.

*Bằng chứng:* Trong workspace `tool_zentao`, call-002 dùng module `zentao.com/gitlab_sync`. Revert thất bại âm thầm, file bị giữ lại. Call-003 cố tạo `new file` trên file đã tồn tại, `patch` ghi đè một phần → file bị truncated, mất closing `}`.

**Macro-level: Không có File Snapshot tại Checkpoint.** Orchestrator lưu workflow state (JSON) vào database tại mỗi checkpoint, nhưng **không snapshot file vật lý** hay SQLite cache. Khi user resume từ step cũ, source code và cache vẫn ở trạng thái mới nhất — hoàn toàn lệch pha với workflow state.

### Giải Pháp: Git Commits + Re-indexing

**Macro-level — Step Checkpoints.** Khi một step hoàn thành thành công, orchestrator tự động tạo Git commit:

```bash
git add .
git commit -m "chore(auto-code-os): checkpoint [step_id]"
```

Khi resume hoặc rollback về một checkpoint cũ:

```text
[Resume từ Checkpoint X] → [git checkout <commit_hash>] → [git reset --hard && git clean -fd]
   → [Xóa workspace_cache.db] → [Gọi IndexWorkspace() → Re-build AST Cache]
   → [Tiếp tục từ Step X+1 với dữ liệu sạch]
```

*Tại sao không backup file `.db`?* Cache SQLite là **stateless** — sinh hoàn toàn từ source code. Xóa và re-index đảm bảo Code ↔ Cache luôn đồng bộ 100%, không lo lệch pha.

**Micro-level — Pre-Retry Worktree Reset.** Trong vòng lặp retry của `code_backend.go`, `code_frontend.go`, `fix.go`:

```go
// Trước mỗi lần retry (attempt >= 2):
if attempt > 1 {
    resetCmd := "git reset --hard HEAD && git clean -fd"
    _, err := s.sandbox.RunInWorktree(ctx, task, agent, stepID+"_retry_reset", resetCmd, worktreeSuffix)
    if err != nil {
        s.log.Log(ctx, task.ID, &jobID, "error",
            fmt.Sprintf("Cannot reset worktree before retry: %v. Aborting step.", err))
        return nil, fmt.Errorf("worktree corrupted, cannot retry: %w", err)
    }
}
```

*Tại sao không cần re-index DB ở đây?* DB chỉ được cập nhật khi step thành công. Các attempt thất bại không ghi gì vào cache.

**Failsafe — Log Revert Outcomes** (`applier.go`, lines 307 và 398 — cả single-repo và multi-repo):

```go
// BEFORE:
_, _ = r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_revert_patch", revertCmd, worktreeSuffix)

// AFTER:
_, revertErr := r.RunSandboxStepInWorktree(ctx, task, agent, stepID+"_revert_patch", revertCmd, worktreeSuffix)
if revertErr != nil && r.Log != nil {
    r.Log(ctx, task.ID, "error", fmt.Sprintf("REVERT FAILED for %s: %v", stepID, revertErr))
}
```

### Checklist

- [ ] Mỗi step thành công tạo ra một Git commit trong worktree
- [ ] Resume task từ checkpoint cũ checkout đúng commit tương ứng
- [ ] Resume task xóa `workspace_cache.db` cũ và trigger `IndexWorkspace()` mới
- [ ] Trong retry loop, `git reset --hard HEAD && git clean -fd` chạy trước attempt 2, 3, ...
- [ ] Revert errors trong `applier.go` được log với level `"error"` thay vì bị discard
- [ ] Test: simulate partial patch apply + failed revert → verify hard-reset khôi phục clean state

---

## B. Retry Context Reconstruction

**Acceptance criteria:**
- LLM luôn nhận được nội dung đầy đủ của các file bị lỗi compile/test trong prompt retry.
- `AffectedFiles` được cập nhật tự động từ compiler output trước mỗi lần retry.
- File reader ưu tiên đọc từ active worktree thay vì main checkout.

### Vấn Đề Hiện Tại

Khi coding step thất bại và retry, LLM nhận được **0 byte file contents** cho các file cần sửa. Ba gap liên hoàn gây ra việc này:

1. **`AffectedFiles` là `null` trong retry loop (Gap A).** `runner.go:54` kiểm tra `len(analysis.AffectedFiles) > 0` trước khi inject nội dung file. Nhưng `AffectedFiles` chỉ được populate **sau khi step hoàn thành** (`code_backend.go:373-393`) — trong lúc retry, nó luôn là `null`. *Bằng chứng:* `request.json` của call-004 hiển thị `"affected_files": null` trong Execution Manifest.
2. **File reader bỏ qua active worktree (Gap D).** Ngay cả khi `AffectedFiles` được populate, `readAffectedFileContent` (`sandbox.go:98-143`) resolve path theo `Paths.Main` (main checkout) và workspace root — **không bao giờ kiểm tra** `Paths.Worktrees["backend"]`. *Bằng chứng:* Main checkout chỉ chứa `.git/` metadata, 0 source files; toàn bộ code nằm trong `worktrees/backend/`.
3. **`fix.go` có cùng gap.** Retry loop (lines 158–267) append instruction theo cùng pattern nhưng **không có cơ chế inject file** nào.

### Giải Pháp

**1. Parse Compiler Output → Populate `AffectedFiles`** (trong retry loop của `code_backend.go`):

```go
// parseCompilerErrorFiles trích xuất file paths từ Go compiler output.
// Ví dụ: "internal/model/commit.go:21:1: syntax error: unexpected EOF"
func parseCompilerErrorFiles(errorOutput string) []string {
    re := regexp.MustCompile(`([a-zA-Z0-9_/.-]+\.go):\d+:\d+:`)
    matches := re.FindAllStringSubmatch(errorOutput, -1)
    seen := make(map[string]bool)
    var files []string
    for _, m := range matches {
        if !seen[m[1]] {
            seen[m[1]] = true
            files = append(files, m[1])
        }
    }
    return files
}
```

```text
[Test/Compile thất bại] → [Parse error output → Trích xuất file paths]
   → [Cập nhật analysis.AffectedFiles] → [runner.go:54 bây giờ inject file contents ✓]
   → [LLM nhận đầy đủ context → Sinh patch chính xác]
```

**2. File Reader Ưu Tiên Active Worktree** (`sandbox.go`, `readAffectedFileContent`) — thêm worktree resolution **trước** logic `Paths.Main` hiện tại:

```go
// MỚI: Kiểm tra active worktree trước
for _, repo := range ws.Repos {
    rel := stripRepoPrefix(file, repo.Name)
    for _, wtPath := range repo.Paths.Worktrees {
        root := filepath.Join(ws.Root, wtPath)
        safePath, err := paths.ResolveSafePath(root, rel)
        if err == nil {
            if content, readErr := paths.ReadLimitedFile(safePath, 20_000); readErr == nil {
                return content, true
            }
        }
    }
}
// HIỆN TẠI: Fall back to Paths.Main (giữ nguyên)
```

**3. Inject Full File Contents Trong Retry** (`code_backend.go`, `code_frontend.go`, `fix.go`):

```go
if attempt > 1 && len(errorFiles) > 0 {
    var buf strings.Builder
    buf.WriteString("\n\n### Current File Contents ###\n")
    for _, f := range errorFiles {
        if content, ok := readFileFromWorktree(ctx, task, f, worktreeSuffix); ok {
            buf.WriteString(fmt.Sprintf("\n--- %s ---\n```\n%s\n```\n", f, content))
        }
    }
    instruction += buf.String()
}
```

### Checklist

- [ ] Compiler output như `internal/model/commit.go:21:1: syntax error` được parse thành file paths
- [ ] Parsed file paths được thêm vào `AffectedFiles` trước retry
- [ ] `readAffectedFileContent` kiểm tra worktree paths trước `Paths.Main`
- [ ] Retry instructions bao gồm full file contents dưới header `### Current File Contents ###`
- [ ] `fix.go` retry loop cũng inject affected file contents
- [ ] Test: simulate compile error → verify file contents xuất hiện trong retry prompt
- [ ] Test: simulate worktree-only files → verify `readAffectedFileContent` trả về content

---

## C. Fresh Context Pipeline

**Acceptance criteria:**
- Coding retry steps luôn fetch semantic snippets mới (không dùng cache cũ).
- `IndexWorkspace` chỉ scan active worktree khi có `AgentPathContext`.
- Snippets không resolve được path sẽ bị loại bỏ (không leak absolute path).

### Vấn Đề Hiện Tại

1. **Stale Context Cache (Gap E).** `ContextLoadStep` chạy một lần duy nhất lúc khởi tạo task. Trong `builder.go:701`, nếu `cachedData != nil`, assembler bỏ qua `ctxEngine.RetrieveContext`. Cache chứa snippets từ trước khi code được viết.
2. **Host Absolute Path Leaks (Gap F).** `IndexWorkspace` scan toàn bộ `code/repos`. Khi `ToLogical` throw boundary error, fallback dùng raw absolute host path (`provider.go:434`), rò rỉ cấu trúc host filesystem cho LLM.

### Giải Pháp

| # | File | Thay đổi |
|:--|:-----|:---------|
| 1 | `builder.go:700-728` | Bypass static cache khi phát hiện retry — gọi `RetrieveContext` trực tiếp thay vì dùng `cachedData`. |
| 2 | `provider.go:321-373` | Scope `IndexWorkspace` theo `pathCtx.PhysicalRoot()` thay vì scan toàn bộ `code/repos`. |
| 3 | `provider.go:376-449` | `continue` (drop snippet) thay vì fallback `relPath = t.Filepath` khi không resolve được path. |
| 4 | `provider.go:232-318` | Scope `GetRepoMap` theo cùng nguyên tắc. |

### Checklist

- [ ] Retry steps fetch fresh snippets
- [ ] Non-retry steps vẫn dùng cache
- [ ] `IndexWorkspace` chỉ scan active worktree
- [ ] Unresolvable snippets bị drop
- [ ] Test: không có absolute host path trong prompt

---

## D. Retry Optimization

**Acceptance criteria:**
- Retry instruction chỉ chứa error mới nhất (không tích lũy).
- Hunk line count mismatches bị bắt trước khi `git apply`.
- Sau 2 lần unified diff thất bại, tự động chuyển sang Search/Replace format.
- RAG boost file bị lỗi + nâng snippet cap khi retry.

### Vấn Đề Hiện Tại

1. **Instruction Bloat (Gap H).** Mỗi retry append full error message vào instruction string. Ở attempt 3, instruction chứa error từ cả attempt 1 và 2. Pattern này tồn tại trong `code_backend.go` (lines 286, 320, 334), `code_frontend.go`, và `fix.go` (lines 177, 213, 221, 239).
2. **Thiếu Hunk Count Validation.** `validator.go:14` (`ValidateUnifiedDiff`) chỉ check `oldStart > len(fileLines)`. Không validate hunk header counts khớp với số dòng thực tế trong hunk body — đây chính xác là bug gây lỗi `malformed patch at line 44`.
3. **`SearchReplaceApplier` Không Được Auto-Select.** `engine.go` đã có `SearchReplaceApplier` nhưng chỉ chọn khi `preferredStrategy == "search_replace"`. Không bao giờ auto-select khi unified diff retry thất bại.
4. **RAG Scoring Sai Ưu Tiên (Gap B+C).** RAG keyword scoring ưu tiên file đầy đủ. File bị broken score thấp và bị loại bởi `maxSnippets = 4`.

### Giải Pháp

**1. Sliding Window Error — Chỉ Giữ Error Mới Nhất:**

```go
var lastError string
for attempt := 1; attempt <= maxRetries; attempt++ {
    currentInstruction := baseInstruction
    if lastError != "" {
        currentInstruction += fmt.Sprintf("\n\nAttempt %d failed:\n%s", attempt-1, lastError)
    }
    out, err = s.llm.RunLLMStep(ctx, ..., currentInstruction)
    if retryNeeded {
        lastError = latestErrorMsg  // chỉ giữ mới nhất
    }
}
```

**2. Validate Hunk Line Counts** — thêm `ValidateHunkCounts` vào `validator.go`: parse từng `@@ -old,oldCount +new,newCount @@` header, đếm số dòng thực tế (prefix `+`, `-`, ` `) trong hunk body, so sánh actual vs expected → trả `ValidationError` nếu mismatch. Gọi trong `ValidateUnifiedDiff` trước khi apply.

**3. Auto-Switch Strategy Khi Retry** — khi attempt ≥ 2 và patch apply thất bại, thêm instruction yêu cầu LLM dùng Search/Replace format:

```
IMPORTANT: Previous unified diff failed. Use SEARCH/REPLACE format instead:
<<<<<<< SEARCH
[exact content to find]
=======
[replacement content]
>>>>>>> REPLACE
```

**4. Boost RAG Cho Error Files** (`builder.go:717`) — khi retry: prepend error file paths vào search query để boost relevance score; nâng `maxSnippets` từ 4 lên 8.

### Checklist

- [ ] Retry instruction chỉ chứa error mới nhất, không tích lũy
- [ ] `ValidateUnifiedDiff` bắt hunk line count mismatch trước `git apply`
- [ ] Sau 2 lần unified diff fail, instruction chuyển sang Search/Replace format
- [ ] RAG query boost files từ compiler output khi retry
- [ ] `maxSnippets` nâng từ 4 lên 8 cho retry attempts
- [ ] Test: instruction size không grow quá initial + 1 error block qua 3 retries
- [ ] Test: hunk `@@ -1,3 +1,19 @@` nhưng có 20 dòng bị catch bởi validation

---

## Affected Files (Consolidated)

| File | Gaps touching it |
|:-----|:------------------|
| `server/internal/orchestrator/worker.go` (or Engine) | G — checkpoint commit + resume restore |
| `server/internal/orchestrator/steps/code_backend.go` | G, A, H — retry reset, parse errors → AffectedFiles, sliding-window error, auto-switch strategy |
| `server/internal/orchestrator/steps/code_frontend.go` | G, A, H — same as above |
| `server/internal/orchestrator/steps/fix.go` | G, A, H — same as above |
| `server/internal/orchestrator/patch/applier.go` | G — log revert errors (lines 307, 398) |
| `server/internal/orchestrator/sandbox.go` | D — worktree path resolution before `Paths.Main` (lines 98-143) |
| `server/internal/orchestrator/llmrunner/runner.go` | A — no change needed; activates automatically once `AffectedFiles` is populated (line 54) |
| `server/internal/prompts/builder.go` | E, B+C — bypass cache on retry (700-728); boost RAG + raise snippet cap (703-720) |
| `server/internal/context/provider/provider.go` | F — scope `IndexWorkspace`/`GetRepoMap` (232-373), drop unresolvable snippets (376-449) |
| `server/internal/orchestrator/patch/validator.go` | — hunk count validation (line 14) |
| `server/internal/orchestrator/patch/engine.go` | — auto-select `SearchReplaceApplier` on retry |
