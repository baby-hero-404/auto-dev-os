# Design: Pluggable Execution Engine

## 1. Architecture

Theo mẫu `SubagentSpawner` của ai-sdlc (strategy/adapter qua DI — `docs/references/README.md:138`):

```
orchestrator/worker.go
    │  resolveEngine(task, project) ─── task.execution_engine ?? project.execution_engine ?? "api_native"
    ▼
engine.ExecutionEngine (interface)
    ├── apiNativeEngine   → delegate llmrunner tool-loop (hiện trạng, không đổi)
    └── cliEngine         → sandbox.CommandRequest spawn CLI trong worktree
```

```go
// server/internal/orchestrator/engine/engine.go
type ExecutionEngine interface {
    // Name returns "api_native" or "cli".
    Name() string
    // Preflight validates the engine can run for this task (CLI existence, config sanity).
    Preflight(ctx context.Context, task *models.Task) error
    // RunCodeStep executes one coding unit; result is judged by worktree diff, not parsed output.
    RunCodeStep(ctx context.Context, req CodeStepRequest) (*CodeStepResult, error)
}

type CodeStepRequest struct {
    Task           *models.Task
    Agent          *models.Agent
    StepID         string
    Prompt         string // full assembled prompt/instruction
    WorktreeSuffix string
}

type CodeStepResult struct {
    Stdout, Stderr string
    ExitCode       int
    HasChanges     bool // from git status --porcelain in worktree
}
```

**Quan trọng**: chỉ **code steps** (code_backend/code_frontend/fix) đi qua interface này. Các step khác (context_load, analyze, review, test, pr) giữ nguyên API-native trong Phase 1 — CLI-mode flow riêng là Phase 2 (`cli-spec-first-flow/`).

## 2. cliEngine execution

1. Ghi prompt: `Write(worktree/.autocode/prompt.md)` qua host path (`repoutil.HostWorktreePath`).
2. Render command: template substitution trên `args` — `{prompt_file}` → container path của prompt file, `{workdir}` → container worktree path. Command + args chạy qua `bash -lc` với `cd <worktree>` (dùng mẫu `cd ... 2>/dev/null; ...` như sandbox.go:67 — nhưng ở đây worktree bắt buộc tồn tại nên guard `[ -d ]` fail sớm).
3. Spawn qua `o.runtime.Run(sandbox.CommandRequest{ NetworkMode: bridge, Timeout: cfg.TimeoutMinutes, Env: cfg.Env })` — **cần mở rộng `CommandRequest` thêm field `Env map[string]string`** nếu chưa có.
4. Sau khi process kết thúc: `git -C <worktree> status --porcelain` → `HasChanges`. Xóa `.autocode/` trước khi checkpoint để không commit prompt file (thêm `.autocode/` vào checkpoint exclude hoặc `rm -rf` trước `git add -A`).
5. Checkpoint/PR path tái dùng nguyên si `repoutil.CreateGitCheckpoint`.

## 2b. Auth preflight & loop-kill

**Token expiry freeze**: CLI dùng OAuth session (Claude Code login) có thể hết hạn giữa chừng → CLI treo chờ browser login trong sandbox (không bao giờ có). Phòng 3 lớp:
1. `auth_check_command` (optional, vd `claude auth status`): preflight chạy trong sandbox, timeout 30s, fail → message rõ ràng trước khi tốn spawn thật.
2. Spawn thật luôn set `CI=1` + đóng stdin — với Docker SDK cụ thể là `OpenStdin: false, Tty: false` trong container config, để CLI cố đòi tty/input sẽ fail ngay thay vì chờ interactive.
3. Khuyến nghị chính trong docs: dùng API key qua `env` cho sandbox execution (subscription OAuth phù hợp host-mode tương lai hơn).

**Loop-kill monitor**: đọc stream log (đã có để stream ra UI); normalize mỗi dòng match error pattern (reuse `errorLine` regex của `tool-output-filtering` khi có, tạm nội bộ trước đó): strip số/hex/paths → hash.

Thuật toán: **frequency-in-window, KHÔNG phải consecutive**. Lỗi compile thực tế (Go/TS/Rust) in stack trace nhiều dòng A-B-C-D-E; vòng lặp cho pattern `A B C D E A B C D E...` — check "cùng hash ×10 liên tiếp" (`A A A...`) sẽ không bao giờ bắt được. Implement trong `cli.go`:

```go
// ring buffer 50 dòng error-hash gần nhất + frequency map
// khi bất kỳ hash nào đạt count ≥10 trong buffer → kill
type loopDetector struct {
    ring  [50]uint64; idx int
    freq  map[uint64]int
}
// push: freq[evicted]--, freq[new]++; return freq[new] >= 10
```

Chỉ đếm dòng match error pattern (progress/info không vào buffer — tránh false positive). Kill → fail step nêu dòng lỗi gốc + count. Tiết kiệm tới ~29m compute/token mỗi lần CLI kẹt compile-error loop.

## 3. Data model

```sql
-- migration
ALTER TABLE projects ADD COLUMN execution_engine text NOT NULL DEFAULT 'api_native';
ALTER TABLE projects ADD COLUMN cli_engine_config jsonb NOT NULL DEFAULT '{}';
ALTER TABLE tasks    ADD COLUMN execution_engine text; -- nullable = inherit
```

```go
type CLIEngineConfig struct {
    Command        string            `json:"command"`         // e.g. "claude"
    Args           []string          `json:"args"`            // e.g. ["-p", "--dangerously-skip-permissions", "{prompt_file}"]
    Env            map[string]string `json:"env,omitempty"`   // masked in GET responses
    TimeoutMinutes int               `json:"timeout_minutes"` // default 30, max 120
    AuthCheckCommand string          `json:"auth_check_command,omitempty"` // e.g. "claude auth status"
}
```

Validation (API layer): engine ∈ {api_native, cli}; nếu engine=cli thì `command` non-empty; args không chứa shell metachars nguy hiểm ngoài placeholder (quote từng arg bằng `paths.QuoteShellArg`).

## 4. Env/secret handling

- `cli_engine_config.env` lưu jsonb; GET project mask values thành `***` (giữ keys để UI hiển thị).
- PATCH semantics: client gửi map mới đầy đủ; value `"***"` nghĩa là "giữ giá trị cũ".
- Không bao giờ đưa env values vào log/step output; stderr của CLI được scan qua `secretPatterns` redaction có sẵn trước khi lưu log (reuse `internal/service/memory.go` patterns — cân nhắc extract ra package chung).

## 5. UI

- `web/src/lib/types.ts`: thêm `execution_engine`, `cli_engine_config` vào `Project`; `execution_engine?: string` vào `Task` + create input.
- Project settings (`project-profile.tsx`): section mới, follow pattern các setting hiện có (`default_model_level`…).
- Task create dialog: select 3 options, default Inherit → gửi `null`.
- Task detail (`tasks/[taskID]/`): badge engine trong `TaskHeroCards` hoặc `TaskSidebar`.

## 6. Trade-offs

- **Black-box tool-loop**: chấp nhận mất per-tool-call events cho CLI mode; timeline UI chỉ hiển thị start/log-stream/end cho code step.
- **Generic command thay vì preset**: linh hoạt tối đa, đổi lại user tự chịu trách nhiệm cú pháp CLI; preset (P3 backlog) sẽ là lớp mỏng đổ sẵn command/args.
- **Không parse structured output CLI**: tránh phụ thuộc format từng CLI; mọi đánh giá dựa trên git diff — nhất quán với triết lý "runtime-centric completion" đã có của project.
