# Tasks: CLI Execution Reliability & Tracing

> **For agentic workers:** Dùng subagent-driven-development hoặc thực thi tuần tự từng task. Checkbox để track tiến độ.

**Goal:** Khi CLI step fail, lý do thật phải thấy được ngay trong log (không cần query DB); lỗi auth-invalid không đốt hết 3 lần retry; CLI agent không được dừng lại hỏi người dùng giữa chừng — nếu buộc phải hỏi, hệ thống pause/resume tử tế thay vì timeout mù.

**Architecture:** Xem `design.md`. Thay đổi tập trung ở `engine/cli.go` (2 detector mới, mirror `cli_quota.go`), `cli_spec_step.go` (logging + pause), 3 file prompt, và generalize `TaskService.Clarify`.

**Tech Stack:** Go, regexp, GORM (jsonb), file-based prompt templates (`.md`).

---

## P0 — Critical (bug đã trace được từ task thật)

### Task 1.1: Log lỗi CLI step ra jsonl kèm lý do thật
> Links to: REQ-001

**Files:**
- Modify: `server/internal/orchestrator/cli_spec_step.go:140`
- Test: `server/internal/orchestrator/cli_spec_step_test.go`

- [x] **Step 1: Viết test fail trước**

```go
func TestRunCLIStep_LogsErrorLevelOnFailure(t *testing.T) {
    logs := &fakeLogger{}
    o := newTestOrchestratorWithLog(t, logs.Log) // helper đã có trong cli_spec_step_test.go, dùng lại
    runner := newCLIStepRunner(o)
    // engine trả về res.Success=false, res.Error/Output khác rỗng — dùng fakeEngine/mockRuntime
    // đã có sẵn trong package (xem cli_test.go: mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 1, Stderr: "Not logged in"}}})
    _, err := runner.RunCLIStep(context.Background(), task, agent, "job-1", "cli_analyze", "instruction", nil)
    if err == nil {
        t.Fatal("expected error")
    }
    var found bool
    for _, entry := range logs.entries {
        if entry.level == "error" && strings.Contains(entry.message, "Not logged in") {
            found = true
        }
    }
    if !found {
        t.Errorf("expected an error-level log entry containing the CLI failure reason, got: %+v", logs.entries)
    }
}
```

- [x] **Step 2: Chạy test, xác nhận fail**

Run: `go test ./server/internal/orchestrator/... -run TestRunCLIStep_LogsErrorLevelOnFailure -v`
Expected: FAIL — log entries chỉ có level "info"

- [x] **Step 3: Sửa `cli_spec_step.go:140`**

```go
if res.Success {
    r.o.log(ctx, task.ID, &jobID, "info", fmt.Sprintf("%s: cli engine finished (success=true)", stepID))
} else {
    reason := res.Error
    if reason == "" {
        reason = "unknown error"
    }
    r.o.log(ctx, task.ID, &jobID, "error", fmt.Sprintf("%s: cli engine failed: %s\n--- output (last 2000 chars) ---\n%s",
        stepID, reason, lastN(res.Output, 2000)))
}
```

Thêm helper `lastN` trong cùng file:

```go
// lastN returns the last n characters of s, or s unchanged if it's shorter.
func lastN(s string, n int) string {
    if len(s) <= n {
        return s
    }
    return s[len(s)-n:]
}
```

- [x] **Step 4: Chạy lại test, xác nhận pass**

Run: `go test ./server/internal/orchestrator/... -run TestRunCLIStep_LogsErrorLevelOnFailure -v`
Expected: PASS

- [x] **Step 5: Commit**

```bash
git add server/internal/orchestrator/cli_spec_step.go server/internal/orchestrator/cli_spec_step_test.go
git commit -m "fix: log CLI step failure reason at error level (REQ-001)"
```

### Task 1.2: Detector auth-invalid + phân loại permanent
> Links to: REQ-002

**Files:**
- Create: `server/internal/orchestrator/engine/cli_auth.go`
- Create: `server/internal/orchestrator/engine/cli_auth_test.go`
- Modify: `server/internal/orchestrator/engine/engine.go` (thêm field `AuthInvalid` vào `CodeStepResult`)
- Modify: `server/internal/orchestrator/engine/cli.go:311-335`

- [x] **Step 1: Viết test fail trước**

```go
package engine

import "testing"

func TestDetectAuthInvalid_ClaudeCode(t *testing.T) {
    if !detectAuthInvalid("claude_code", "Not logged in · Please run /login\n") {
        t.Error("expected match for claude_code 'Not logged in' message")
    }
}

func TestDetectAuthInvalid_NoMatch(t *testing.T) {
    if detectAuthInvalid("claude_code", "all good, task complete") {
        t.Error("expected no match for normal success output")
    }
}

func TestDetectAuthInvalid_FallbackRule(t *testing.T) {
    if !detectAuthInvalid("unknown_ref", "authentication required") {
        t.Error("expected fallback '*' rule to match unknown ref")
    }
}
```

- [x] **Step 2: Chạy test, xác nhận fail**

Run: `go test ./server/internal/orchestrator/engine/... -run TestDetectAuthInvalid -v`
Expected: FAIL với `undefined: detectAuthInvalid`

- [x] **Step 3: Tạo `cli_auth.go`** (nội dung đầy đủ ở `design.md`, copy nguyên — không rút gọn)

```go
package engine

import "regexp"

// CLIAuthInvalidRule mirrors CLIQuotaRule (cli_quota.go) but for a
// different failure class: the linked credential is present and marked
// active, yet the CLI itself reports it isn't authenticated. Unlike quota
// (transient, cools down and retries later), this is permanent until a
// human re-runs the CLI auth capture flow — retrying burns attempts for
// nothing since the same credential produces the same failure every time.
type CLIAuthInvalidRule struct {
    Patterns []*regexp.Regexp
}

var CLIAuthInvalidRules = map[string][]CLIAuthInvalidRule{
    "claude_code": {
        {Patterns: []*regexp.Regexp{
            regexp.MustCompile(`(?i)not logged in`),
            regexp.MustCompile(`(?i)please run /login`),
            regexp.MustCompile(`(?i)please run claude login`),
        }},
    },
    "openai_codex": {
        {Patterns: []*regexp.Regexp{
            regexp.MustCompile(`(?i)not authenticated`),
            regexp.MustCompile(`(?i)codex login`),
        }},
    },
    "antigravity": {
        {Patterns: []*regexp.Regexp{
            regexp.MustCompile(`(?i)not authenticated`),
            regexp.MustCompile(`(?i)please sign in`),
        }},
    },
    "*": {
        {Patterns: []*regexp.Regexp{
            regexp.MustCompile(`(?i)not logged in`),
            regexp.MustCompile(`(?i)please authenticate`),
            regexp.MustCompile(`(?i)authentication required`),
        }},
    },
}

func detectAuthInvalid(ref string, combined string) bool {
    rules, ok := CLIAuthInvalidRules[ref]
    if !ok {
        rules = CLIAuthInvalidRules["*"]
    }
    for _, rule := range rules {
        for _, p := range rule.Patterns {
            if p.MatchString(combined) {
                return true
            }
        }
    }
    return false
}
```

- [x] **Step 4: Chạy lại test detector, xác nhận pass**

Run: `go test ./server/internal/orchestrator/engine/... -run TestDetectAuthInvalid -v`
Expected: PASS

- [x] **Step 5: Thêm field `AuthInvalid` vào `CodeStepResult` (`engine.go`, ngay cạnh `QuotaExceeded`)**

```go
	// AuthInvalid is true when the captured output matched a known
	// "not authenticated" signature for this CLI (see cli_auth.go). Unlike
	// QuotaExceeded, this marks the failure as permanent — the caller
	// should not spend remaining retry attempts on it.
	AuthInvalid bool
```

- [x] **Step 6: Viết test tích hợp ở `cli_test.go` — fail trước**

```go
func TestCLIEngine_RunCodeStep_AuthInvalid(t *testing.T) {
    rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 1, Stdout: "Not logged in · Please run /login\n"}}}
    e := NewCLIEngine(rt, nil)
    cfg := &models.CLIEngineConfig{Command: "claude", ProfileRef: "claude_code"}
    res, err := e.RunCodeStep(context.Background(), baseReq(cfg))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !res.AuthInvalid {
        t.Errorf("expected AuthInvalid=true, got %+v", res)
    }
    if res.Success {
        t.Errorf("expected Success=false")
    }
}
```

Run: `go test ./server/internal/orchestrator/engine/... -run TestCLIEngine_RunCodeStep_AuthInvalid -v`
Expected: FAIL — `res.AuthInvalid` luôn false (field chưa được set trong `RunCodeStep`)

- [x] **Step 7: Sửa `RunCodeStep` (`cli.go:319-334`) để set `AuthInvalid` và ưu tiên nó trước quota (REQ-002 scenario 3)**

```go
	killed := detectLoop(combined)
	authInvalid := detectAuthInvalid(cfg.ProfileRef, combined)
	quotaExceeded := !authInvalid && detectQuotaExceeded(cfg.ProfileRef, combined, result.ExitCode)

	res := &CodeStepResult{
		Success:       result.ExitCode == 0 && !killed,
		Output:        redactSecrets(combined),
		LoopKilled:    killed,
		QuotaExceeded: quotaExceeded,
		AuthInvalid:   authInvalid,
		Files:         capturedFiles,
	}
	switch {
	case killed:
		res.Error = "cli engine: repeated error output detected, killing step as a stuck loop"
	case authInvalid:
		res.Error = redactSecrets(fmt.Sprintf("cli engine: credential not authenticated (permanent, will not retry): %s", lastNonEmptyLine(combined)))
	case result.ExitCode != 0:
		res.Error = redactSecrets(fmt.Sprintf("cli exited with status %d", result.ExitCode))
	}
	return res, nil
```

Thêm helper `lastNonEmptyLine` trong `cli.go` (dùng lại ở Task 2.2 cho REQ-005 luôn, không viết trùng):

```go
// lastNonEmptyLine returns the last non-blank line of s, trimmed, or "" if
// every line is blank.
func lastNonEmptyLine(s string) string {
    lines := strings.Split(s, "\n")
    for i := len(lines) - 1; i >= 0; i-- {
        if t := strings.TrimSpace(lines[i]); t != "" {
            return t
        }
    }
    return ""
}
```

- [x] **Step 8: Chạy lại toàn bộ test package, xác nhận pass**

Run: `go test ./server/internal/orchestrator/engine/... -v`
Expected: PASS toàn bộ, bao gồm test cũ (`TestCLIEngine_RunCodeStep_NonZeroExit` vẫn phải pass — auth-invalid không match thì rơi về nhánh `result.ExitCode != 0` như cũ)

- [x] **Step 9: Wrap lỗi permanent để retry-loop nhận diện được — kiểm tra `worker.go`/state machine hiện đang check gì cho `ErrConfigInvalid`**

```bash
grep -rn "ErrConfigInvalid" server/internal/orchestrator/*.go
```

Nếu chưa có nơi nào check (khả năng cao — `ErrConfigInvalid` hiện chỉ dùng cho lỗi cấu hình lúc Preflight, chưa dùng cho lỗi runtime), thêm ở `cli_spec_step.go` (`RunCLIStep`, ngay sau `res, err := eng.RunCodeStep(...)`):

```go
	if res.AuthInvalid {
		return steps.CLIStepOutput{Output: res.Output}, fmt.Errorf("%w: %s", engine.ErrConfigInvalid, res.Error)
	}
```

Sau đó xác nhận state machine (nơi bắt lỗi để quyết định retry, tìm bằng `grep -rn "Retrying attempt" server/internal/orchestrator/`) đã có nhánh `errors.Is(err, engine.ErrConfigInvalid)` bỏ qua retry chưa — nếu chưa, đây là điểm cần thêm, viết task con cụ thể sau khi xác nhận vị trí thật (không đoán trước khi grep ra).

- [x] **Step 10: Commit**

```bash
git add server/internal/orchestrator/engine/cli_auth.go server/internal/orchestrator/engine/cli_auth_test.go server/internal/orchestrator/engine/engine.go server/internal/orchestrator/engine/cli.go server/internal/orchestrator/engine/cli_test.go server/internal/orchestrator/cli_spec_step.go
git commit -m "fix: classify CLI auth-invalid failures as permanent, skip retry (REQ-002)"
```

### Task 1.3: `auth_check_command` — investigation trước khi sửa
> Links to: REQ-003

**Files:**
- Modify (có điều kiện, sau investigation): `server/pkg/models/cli_profiles.go`

- [x] **Step 1: Investigation — chạy thử từng CLI binary thật (có sẵn trên máy), tìm lệnh check-login không side-effect**

Đã chạy `claude --help`, `codex --help`, `antigravity --help` + các subcommand con thật (không phải đoán) trên máy có cài sẵn cả 3 CLI.

- [x] **Step 2: Kết quả thật (đã điền)**

| CLI | Lệnh check-login tìm được | Side-effect? | Quyết định |
|---|---|---|---|
| claude | `claude auth status` (subcommand `auth` có `login`/`logout`/`status`) — trả JSON `{"loggedIn": bool, ...}`, **exit code luôn 0 bất kể trạng thái** (verified: chạy thật trả `exit 0` + `loggedIn: true`) | Không (read-only, chỉ đọc credential đã lưu) | Dùng lệnh này + thêm content-check (không thể chỉ dựa exit code) |
| codex | `codex login status` (subcommand `login` có `status`) — in "Logged in using ChatGPT" khi đã login, exit 0 (verified chạy thật) | Không | Dùng lệnh này; hành vi lúc **chưa** login chưa verify được (xem ghi chú rủi ro bên dưới) |
| antigravity | Không tìm thấy — `antigravity auth`/`antigravity run --help` đều fallback về help chung (không phải subcommand thật), không có bất kỳ lệnh `auth`/`login`/`whoami`/`status` nào trong `--help` | N/A | Giữ nguyên `antigravity --version` — không đoán mò |

**Phát hiện phụ ngoài phạm vi REQ-003 (ghi nhận, KHÔNG tự sửa)**: `cli_profiles.go`'s `antigravity` profile có `Args: []string{"run", "--yes", "{prompt_file}"}` — nhưng `antigravity run --help` không tồn tại như 1 subcommand thật (rơi về help chung của `antigravity [options] [paths...]`), nghĩa là **lệnh invocation thật của antigravity trong `RunCodeStep` có thể đang sai/không hoạt động như kỳ vọng**, độc lập với vấn đề auth_check_command. `antigravity --help` cho thấy subcommand thật là `chat` ("Pass in a prompt to run in a chat session..."). Đây là 1 bug tiềm ẩn khác, ngoài scope REQ-003 — cần OpenSpec/investigation riêng trước khi sửa `Args`, không tự ý đổi ở đây.

- [x] **Step 3: Cập nhật `auth_check_command` cho claude_code và openai_codex; giữ nguyên antigravity**

`server/pkg/models/cli_profiles.go`:
```go
"claude_code": {
    ...
    AuthCheckCommand: "claude auth status", // was "claude --version"
    ...
},
"openai_codex": {
    ...
    AuthCheckCommand: "codex login status", // was "codex --version"
    ...
},
"antigravity": {
    ...
    AuthCheckCommand: "antigravity --version", // unchanged — no side-effect-free check command found
    ...
},
```

**Rủi ro đã biết, chưa verify được**: cả `claude auth status` và `codex login status` là lệnh thông tin (informational) — với `claude` đã **verify trực tiếp** là exit code luôn 0 bất kể trạng thái login (chỉ khác ở nội dung JSON `loggedIn: true/false`). Do đó `engine/cli.go`'s `Preflight` được sửa thêm content-check (tái dùng `detectAuthInvalid` từ Task 1.2) thay vì chỉ dựa `ExitCode != 0` — xem Step 3b. Với `codex`, **không thể verify hành vi lúc chưa login** vì việc đó đòi hỏi logout credential thật đang dùng trên máy (destructive, không tự ý làm) — pattern `"not authenticated"`/`"not logged in"` được thêm vào `CLIAuthInvalidRules["openai_codex"]` như best-effort, cần verify thật khi có credential test riêng (không phải credential thật của người dùng).

- [x] **Step 3b: Sửa `Preflight` (`engine/cli.go`) để check nội dung output, không chỉ exit code**

```go
	if authRes.ExitCode != 0 {
		return "", fmt.Errorf("cli engine: auth check command exited %d: %s", authRes.ExitCode, redactSecrets(strings.TrimSpace(authRes.Stderr)))
	}
	authOutput := authRes.Stdout
	if strings.TrimSpace(authRes.Stderr) != "" {
		if authOutput != "" {
			authOutput += "\n"
		}
		authOutput += authRes.Stderr
	}
	if detectAuthInvalid(cfg.ProfileRef, authOutput) {
		return "", fmt.Errorf("cli engine: auth check command reports not authenticated: %s", redactSecrets(strings.TrimSpace(authOutput)))
	}
	return "", nil
```

Thêm pattern JSON vào `CLIAuthInvalidRules["claude_code"]` (`cli_auth.go`, Task 1.2): `` `(?i)"loggedIn"\s*:\s*false` `` — verified đúng shape output thật của `claude auth status`.

- [x] **Step 4: Chạy test**

Run: `go test ./internal/orchestrator/engine/... -run TestCLIEngine_Preflight -v`
Result: PASS (bao gồm test mới `TestCLIEngine_Preflight_AuthCheckExitsZeroButReportsNotLoggedIn`)

### Task 1.4: `antigravity` profile — fix subcommand sai + thêm display server
> Links to: REQ-008

Phát hiện phụ từ Task 1.3 (ghi nhận lúc đó, không sửa vì ngoài scope REQ-003): `antigravity` là VSCode-fork GUI binary, không phải headless CLI. `cli_profiles.go` đang cấu hình `Args: ["run", "--yes", "{prompt_file}"]` nhưng `run` không phải subcommand thật (verified: `antigravity run --help` rơi về help chung). Subcommand thật là `antigravity chat [prompt] --mode agent`, và vì đây là GUI app nên cần X display để khởi động — `docker/Dockerfile.sandbox` chưa có display server nào. Người dùng đã quyết định: thêm Xvfb thay vì tắt candidate này.

**Files:**
- Modify: `server/pkg/models/cli_profiles.go` (antigravity profile)
- Modify: `docker/Dockerfile.sandbox` (thêm gói `xvfb`, `xauth`)
- Test: `server/pkg/models/cli_profiles_test.go`

- [x] **Step 1: Viết test fail trước**

```go
func TestProfileOrEmpty_Antigravity_WrappedInXvfb(t *testing.T) {
	p, ok := ProfileOrEmpty("antigravity")
	if !ok {
		t.Fatal("expected antigravity to be known")
	}
	if p.Command != "xvfb-run" {
		t.Errorf("Command = %q, want %q (antigravity needs a display to launch)", p.Command, "xvfb-run")
	}
	joined := strings.Join(p.Args, " ")
	if !strings.Contains(joined, "antigravity") {
		t.Errorf("Args %v: expected the antigravity binary to be the wrapped command", p.Args)
	}
	if !strings.Contains(joined, "chat") {
		t.Errorf("Args %v: expected the real `chat` subcommand, not `run` (which doesn't exist)", p.Args)
	}
}
```

- [x] **Step 2: Chạy test, xác nhận fail**

Run: `go test ./pkg/models/... -run TestProfileOrEmpty_Antigravity -v`
Result: FAIL — `Command = "antigravity"`, `Args = [run --yes {prompt_file}]`

- [x] **Step 3: Sửa `cli_profiles.go`**

```go
"antigravity": {
    Command:            "xvfb-run",
    Args:               []string{"-a", "antigravity", "chat", "--mode", "agent", "{prompt_file}"},
    AuthCheckCommand:   "antigravity --version",
    TimeoutMinutes:     30,
    CredentialProvider: "cli:antigravity",
},
```

`AuthCheckCommand` giữ nguyên `antigravity --version` (gọi binary trực tiếp, không qua `xvfb-run`) vì `--version` không mở cửa sổ, không cần display.

- [x] **Step 4: Thêm `xvfb`/`xauth` vào `docker/Dockerfile.sandbox`'s apt-get install list**, kèm comment giải thích vì sao (mỗi lần `sandbox.Runtime.Run()` tạo container mới — không có container dài hạn để gắn 1 Xvfb service, nên `xvfb-run` per-invocation là lựa chọn đúng thay vì chạy Xvfb nền).

- [x] **Step 5: Chạy lại test, xác nhận pass**

Run: `go test ./pkg/models/... -run TestProfileOrEmpty -v`
Result: PASS

- [x] **Step 6: Build + full test suite**

Run: `go build ./... && go test ./...`
Result: PASS toàn bộ, không regression

**Giới hạn chưa verify được** (ngoài scope task này, cần Task 2.5 hoặc investigation riêng khi có điều kiện chạy thật): chưa xác nhận `antigravity chat --mode agent` có thật sự block tới khi agent xong việc rồi trả exit code phản ánh đúng kết quả hay không — hành vi "hoàn thành" của 1 GUI app agent khác về bản chất so với 1 process CLI thuần, `xvfb-run` chỉ giải quyết được vấn đề "khởi động được", không đảm bảo "báo cáo kết quả đúng".

## P1 — High (clarification handling cho CLI mode)

### Task 2.1: Prompt cấm CLI agent hỏi lại giữa chừng
> Links to: REQ-004

**Files:**
- Modify: `server/internal/prompts/steps/cli_analyze.md`
- Modify: `server/internal/prompts/steps/cli_spec.md`
- Modify: `server/internal/prompts/steps/cli_implement.md`
- Test: `server/internal/prompts/cli_steps_test.go` (đã tồn tại — thêm assertion mới)

- [x] **Step 1: Viết test fail trước (golden-file style, xem test hiện có trong `cli_steps_test.go` để theo đúng convention)**

```go
func TestCLIStepPrompts_ForbidClarifyingQuestions(t *testing.T) {
    for _, step := range []string{"cli_analyze", "cli_spec", "cli_implement"} {
        content, err := os.ReadFile(filepath.Join("steps", step+".md"))
        if err != nil {
            t.Fatalf("read %s: %v", step, err)
        }
        if !strings.Contains(string(content), "Do not ask clarifying questions") {
            t.Errorf("%s.md missing 'Do not ask clarifying questions' instruction", step)
        }
    }
}
```

- [x] **Step 2: Chạy test, xác nhận fail**

Run: `go test ./server/internal/prompts/... -run TestCLIStepPrompts_ForbidClarifyingQuestions -v`
Expected: FAIL — instruction chưa tồn tại

- [x] **Step 3: Thêm section vào cuối mỗi file (giữ nguyên nội dung cũ, chỉ append)**

```markdown

## Do not ask clarifying questions

You are running non-interactively — there is no user available to answer a
question mid-run. If something is ambiguous or underspecified, make the most
reasonable assumption, proceed, and record the assumption explicitly (under
Risks for cli_analyze, under "## Implementation Notes" for cli_spec/
cli_implement). Never stop and wait for input.
```

- [x] **Step 4: Chạy lại test, xác nhận pass**

Run: `go test ./server/internal/prompts/... -run TestCLIStepPrompts_ForbidClarifyingQuestions -v`
Expected: PASS

- [x] **Step 5: Chạy toàn bộ test package prompts (golden files có thể cần regenerate nếu test so khớp toàn văn bản)**

Run: `go test ./server/internal/prompts/... -v`
Expected: PASS — nếu có golden test fail do nội dung dài hơn, regenerate theo hướng dẫn có sẵn trong package (tìm flag `-update` hoặc tương tự trong `testdata/golden/`)

- [x] **Step 6: Commit**

```bash
git add server/internal/prompts/steps/cli_analyze.md server/internal/prompts/steps/cli_spec.md server/internal/prompts/steps/cli_implement.md server/internal/prompts/cli_steps_test.go
git commit -m "feat: instruct CLI agent to assume instead of asking clarifying questions (REQ-004)"
```

### Task 2.2: Detector "awaiting-input"
> Links to: REQ-005

**Files:**
- Create: `server/internal/orchestrator/engine/cli_question_detect.go`
- Create: `server/internal/orchestrator/engine/cli_question_detect_test.go`
- Modify: `server/internal/orchestrator/engine/engine.go` (thêm field `AwaitingInput`)
- Modify: `server/internal/orchestrator/engine/cli.go` (gọi detector trong `RunCodeStep`)

- [x] **Step 1: Viết test fail trước**

```go
package engine

import "testing"

func TestDetectAwaitingInput_YesNoPrompt(t *testing.T) {
    if !detectAwaitingInput("Proceed with deletion? (y/n)") {
        t.Error("expected match for (y/n) prompt")
    }
}

func TestDetectAwaitingInput_DoYouWantTo(t *testing.T) {
    if !detectAwaitingInput("Do you want to overwrite the existing config?") {
        t.Error("expected match for 'Do you want to...?' prompt")
    }
}

func TestDetectAwaitingInput_NormalOutput(t *testing.T) {
    if detectAwaitingInput("Task completed successfully.") {
        t.Error("expected no match for normal completion message")
    }
}
```

- [x] **Step 2: Chạy test, xác nhận fail**

Run: `go test ./server/internal/orchestrator/engine/... -run TestDetectAwaitingInput -v`
Expected: FAIL — `undefined: detectAwaitingInput`

- [x] **Step 3: Tạo `cli_question_detect.go`**

```go
package engine

import "regexp"

// awaitingInputPatterns match a CLI's last output line when it looks like
// the process stopped to ask a yes/no or open-ended confirmation question.
// Checked only against the last non-empty line (see detectAwaitingInput) —
// these tools legitimately print a lot of "?"-ending progress text, so
// matching mid-transcript would false-positive constantly.
var awaitingInputPatterns = []*regexp.Regexp{
    regexp.MustCompile(`(?i)\(y/n\)\s*$`),
    regexp.MustCompile(`(?i)do you want to\b.*\?\s*$`),
    regexp.MustCompile(`(?i)please confirm\b`),
    regexp.MustCompile(`(?i)waiting for (user )?input`),
    regexp.MustCompile(`(?i)\bwhich (option|approach|one)\b.*\?\s*$`),
}

// detectAwaitingInput reports whether lastLine (the last non-empty line of
// a finished/killed CLI run's combined output) looks like the process was
// blocked waiting for an answer that never came (no stdin is ever attached
// to sandboxed CLI runs — see RunCodeStep).
func detectAwaitingInput(lastLine string) bool {
    for _, p := range awaitingInputPatterns {
        if p.MatchString(lastLine) {
            return true
        }
    }
    return false
}
```

- [x] **Step 4: Chạy lại test, xác nhận pass**

Run: `go test ./server/internal/orchestrator/engine/... -run TestDetectAwaitingInput -v`
Expected: PASS

- [x] **Step 5: Thêm field `AwaitingInput` vào `CodeStepResult` (`engine.go`, cạnh `AuthInvalid` từ Task 1.2)**

```go
	// AwaitingInput is true when the CLI's last output line looks like it
	// was blocked waiting for a clarifying answer (see
	// cli_question_detect.go). The caller (RunCLIStep) turns this into a
	// workflow.PauseError instead of a plain failure (REQ-006).
	AwaitingInput bool
```

- [x] **Step 6: Viết test tích hợp — fail trước**

```go
func TestCLIEngine_RunCodeStep_AwaitingInput(t *testing.T) {
    rt := &mockRuntime{results: []*sandbox.CommandResult{{ExitCode: 1, Stdout: "Analyzing repo...\nProceed with deletion? (y/n)"}}}
    e := NewCLIEngine(rt, nil)
    cfg := &models.CLIEngineConfig{Command: "claude", ProfileRef: "claude_code"}
    res, err := e.RunCodeStep(context.Background(), baseReq(cfg))
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if !res.AwaitingInput {
        t.Errorf("expected AwaitingInput=true, got %+v", res)
    }
}
```

Run: `go test ./server/internal/orchestrator/engine/... -run TestCLIEngine_RunCodeStep_AwaitingInput -v`
Expected: FAIL — field chưa được set

- [x] **Step 7: Sửa `RunCodeStep` để set field (tiếp nối đoạn đã sửa ở Task 1.2 Step 7)**

```go
	killed := detectLoop(combined)
	authInvalid := detectAuthInvalid(cfg.ProfileRef, combined)
	quotaExceeded := !authInvalid && detectQuotaExceeded(cfg.ProfileRef, combined, result.ExitCode)
	awaitingInput := !killed && !authInvalid && detectAwaitingInput(lastNonEmptyLine(combined))

	res := &CodeStepResult{
		Success:       result.ExitCode == 0 && !killed,
		Output:        redactSecrets(combined),
		LoopKilled:    killed,
		QuotaExceeded: quotaExceeded,
		AuthInvalid:   authInvalid,
		AwaitingInput: awaitingInput,
		Files:         capturedFiles,
	}
	switch {
	case killed:
		res.Error = "cli engine: repeated error output detected, killing step as a stuck loop"
	case authInvalid:
		res.Error = redactSecrets(fmt.Sprintf("cli engine: credential not authenticated (permanent, will not retry): %s", lastNonEmptyLine(combined)))
	case awaitingInput:
		res.Error = redactSecrets(fmt.Sprintf("cli engine: process appears to be waiting for input: %s", lastNonEmptyLine(combined)))
	case result.ExitCode != 0:
		res.Error = redactSecrets(fmt.Sprintf("cli exited with status %d", result.ExitCode))
	}
	return res, nil
```

- [x] **Step 8: Chạy lại toàn bộ test, xác nhận pass**

Run: `go test ./server/internal/orchestrator/engine/... -v`
Expected: PASS toàn bộ

- [x] **Step 9: Commit**

```bash
git add server/internal/orchestrator/engine/cli_question_detect.go server/internal/orchestrator/engine/cli_question_detect_test.go server/internal/orchestrator/engine/engine.go server/internal/orchestrator/engine/cli.go server/internal/orchestrator/engine/cli_test.go
git commit -m "feat: detect CLI runs stuck awaiting clarifying input (REQ-005)"
```

### Task 2.3: Pause CLI step + append ClarificationRound khi `AwaitingInput`
> Links to: REQ-006 (phần 1 — pause)

**Files:**
- Modify: `server/internal/orchestrator/steps/cli_analyze.go`
- Modify: `server/internal/orchestrator/steps/cli_spec.go`
- Modify: `server/internal/orchestrator/steps/cli_implement.go`
- Modify: `server/internal/orchestrator/cli_spec_step.go` (propagate `AwaitingInput` qua `steps.CLIStepOutput`)
- Test: file test tương ứng của từng step

- [x] **Step 1: Thêm field `AwaitingInput bool` vào `steps.CLIStepOutput` (tìm định nghĩa struct — cùng chỗ với `Output`/`Files`/`ChangedFiles`)**

```bash
grep -n "type CLIStepOutput struct" -A 10 server/internal/orchestrator/steps/*.go
```

Thêm field theo đúng convention đã tìm được (không đoán tên file trước khi grep).

- [x] **Step 2: `cli_spec_step.go` — propagate field từ `res.AwaitingInput`**

```go
	out := steps.CLIStepOutput{Output: res.Output, Files: res.Files, AwaitingInput: res.AwaitingInput}
	if res.AwaitingInput {
		// Not a failure in the usual sense — the caller step decides
		// whether to pause for clarification (REQ-006). Return the
		// output so the step can extract the question, but no error.
		return out, nil
	}
	if !res.Success {
		errMsg := res.Error
		if errMsg == "" {
			errMsg = "cli engine: step failed"
		}
		return out, fmt.Errorf("%s", errMsg)
	}
```

- [x] **Step 3: Viết test fail trước cho `cli_analyze.go`**

```go
func TestCLIAnalyzeStep_PausesForClarification(t *testing.T) {
    runner := &fakeCLIStepRunner{output: steps.CLIStepOutput{Output: "Proceed with deletion? (y/n)", AwaitingInput: true}}
    step := NewCLIAnalyzeStep(rt, tasks, runner, prompts, log) // dùng lại test helper đã có sẵn trong package
    _, err := step.Execute(context.Background(), stepCtx)
    var pauseErr workflow.PauseError
    if !errors.As(err, &pauseErr) {
        t.Fatalf("expected workflow.PauseError, got: %v", err)
    }
    if pauseErr.Step != workflow.StepCLIAnalyze {
        t.Errorf("expected pause on cli_analyze step, got %q", pauseErr.Step)
    }
}
```

- [x] **Step 4: Chạy test, xác nhận fail**

Run: `go test ./server/internal/orchestrator/steps/... -run TestCLIAnalyzeStep_PausesForClarification -v`
Expected: FAIL

- [x] **Step 5: Sửa `cli_analyze.go` (`Execute`, ngay sau `out, err := s.runner.RunCLIStep(...)`)**

```go
	out, err := s.runner.RunCLIStep(ctx, s.rt.Task, s.rt.Agent, s.rt.JobID, s.ID(), instruction, []string{cliAnalysisCapturePath})
	if err != nil {
		return nil, fmt.Errorf("cli_analyze: %w", err)
	}
	if out.AwaitingInput {
		return s.pauseForClarification(ctx, out.Output)
	}
```

Thêm helper dùng chung — đặt trong `cli_analyze.go` (step đầu tiên của flow, các step khác gọi qua interface `TaskUpdater`/`tasks` đã có sẵn, không cần export mới):

```go
// pauseForClarification appends a ClarificationRound built from the CLI's
// last output line and pauses the workflow, mirroring the API-native flow's
// clarification_required convention (analyze.go) — reused, not reinvented,
// so the frontend's existing clarification UI keeps working unchanged.
func (s *CLIAnalyzeStep) pauseForClarification(ctx context.Context, cliOutput string) (StepResult, error) {
	var rounds []models.ClarificationRound
	if len(s.rt.Task.Clarifications) > 0 {
		_ = json.Unmarshal(s.rt.Task.Clarifications, &rounds)
	}
	rounds = append(rounds, models.ClarificationRound{
		Round:     len(rounds) + 1,
		Timestamp: time.Now(),
		Questions: []string{lastNonEmptyLine(cliOutput)},
	})
	raw, err := json.Marshal(rounds)
	if err != nil {
		return nil, fmt.Errorf("cli_analyze: marshal clarifications: %w", err)
	}
	specStatus := models.TaskSpecStatusClarificationRequired
	if s.tasks != nil {
		if _, err := s.tasks.Update(ctx, s.rt.Task.ID, models.UpdateTaskInput{
			Clarifications: raw,
			SpecStatus:     &specStatus,
		}); err != nil {
			return nil, fmt.Errorf("cli_analyze: persist clarification: %w", err)
		}
	}
	s.rt.Task.Clarifications = raw
	s.rt.Task.SpecStatus = specStatus
	return nil, workflow.PauseError{Step: s.ID(), Reason: "workflow paused for human task clarification (cli)"}
}
```

`lastNonEmptyLine` đã thêm ở `engine/cli.go` (Task 1.2) — cần 1 bản export hoặc duplicate nhỏ trong package `steps` (2 package khác nhau, không import chéo không cần thiết) — duplicate 4 dòng còn rẻ hơn tạo dependency mới:

```go
func lastNonEmptyLine(s string) string {
    lines := strings.Split(s, "\n")
    for i := len(lines) - 1; i >= 0; i-- {
        if t := strings.TrimSpace(lines[i]); t != "" {
            return t
        }
    }
    return ""
}
```

- [x] **Step 6: Chạy lại test, xác nhận pass**

Run: `go test ./server/internal/orchestrator/steps/... -run TestCLIAnalyzeStep_PausesForClarification -v`
Expected: PASS

- [x] **Step 7: Lặp lại Step 3-6 tương tự cho `cli_spec.go` và `cli_implement.go`** (cùng pattern `pauseForClarification`, `s.ID()` tự động đúng theo step đang chạy — đây chính là điểm khác biệt với flow cũ: `s.ID()` thay vì hard-code `workflow.StepAnalyze`)

- [x] **Step 8: Chạy toàn bộ test package**

Run: `go test ./server/internal/orchestrator/steps/... -v`
Expected: PASS toàn bộ, không phá test cũ của `analyze.go` (không đụng file đó ở task này)

- [x] **Step 9: Commit**

```bash
git add server/internal/orchestrator/steps/cli_analyze.go server/internal/orchestrator/steps/cli_spec.go server/internal/orchestrator/steps/cli_implement.go server/internal/orchestrator/cli_spec_step.go server/internal/orchestrator/steps/*_test.go
git commit -m "feat: pause CLI steps for human clarification instead of failing (REQ-006 part 1)"
```

### Task 2.4: Generalize `TaskService.Clarify` để resume đúng step đã pause
> Links to: REQ-006 (phần 2 — resume)

**Files:**
- Modify: `server/pkg/models/task.go` (field mới trên `Task`/`UpdateTaskInput`)
- Modify: `server/internal/service/task.go:215-250`
- Create: `server/migration/000021_add_task_paused_step.up.sql`, `.down.sql`
- Test: `server/internal/service/task_test.go`

- [x] **Step 1: Viết test fail trước — clarify từ 1 task pause ở `cli_implement` phải resume đúng status cho step đó, không phải luôn `Analyzing`**

```go
func TestTaskService_Clarify_ResumesPausedCLIStep(t *testing.T) {
    task := &models.Task{ID: "t1", Status: models.TaskStatusCoding, SpecStatus: models.TaskSpecStatusClarificationRequired, PausedStep: workflow.StepCLIImplement}
    repo := &fakeTaskRepo{tasks: map[string]*models.Task{"t1": task}}
    svc := NewTaskService(repo, nil)
    updated, err := svc.Clarify(context.Background(), "t1", models.ClarifyTaskInput{Context: "use option A"})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if updated.Status != models.TaskStatusCoding {
        t.Errorf("expected resume status TaskStatusCoding (cli_implement's StatusOnResume), got %q", updated.Status)
    }
}

func TestTaskService_Clarify_AnalyzeFlowUnchanged(t *testing.T) {
    // Regression: task cũ không có PausedStep set (flow API-native, chưa từng
    // dùng field mới) phải resume y hệt hôm nay — về TaskStatusAnalyzing.
    task := &models.Task{ID: "t2", Status: models.TaskStatusAnalyzing, SpecStatus: models.TaskSpecStatusClarificationRequired}
    repo := &fakeTaskRepo{tasks: map[string]*models.Task{"t2": task}}
    svc := NewTaskService(repo, nil)
    updated, err := svc.Clarify(context.Background(), "t2", models.ClarifyTaskInput{Context: "answer"})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if updated.Status != models.TaskStatusAnalyzing {
        t.Errorf("expected TaskStatusAnalyzing (unchanged legacy behavior), got %q", updated.Status)
    }
}
```

- [x] **Step 2: Chạy test, xác nhận fail**

Run: `go test ./server/internal/service/... -run TestTaskService_Clarify -v`
Expected: FAIL — `PausedStep` chưa tồn tại trên `models.Task`

- [x] **Step 3: Thêm field vào `models.Task` (`task.go:62-83`, ngay sau `SpecStatus`)**

```go
	// PausedStep records which workflow step raised the clarification pause
	// (REQ-006) — empty for the legacy API-native flow, where clarification
	// always originates at, and resumes to, the "analyze" step. Set only by
	// the CLI spec-first steps (cli_analyze/cli_spec/cli_implement).
	PausedStep string `json:"paused_step,omitempty" gorm:"default:''"`
```

Và trong `UpdateTaskInput` (`task.go:99-115`, cạnh `SpecStatus`):

```go
	PausedStep *string `json:"paused_step,omitempty"`
```

- [x] **Step 4: Migration**

`server/migration/000021_add_task_paused_step.up.sql`:
```sql
ALTER TABLE tasks ADD COLUMN paused_step TEXT NOT NULL DEFAULT '';
```

`server/migration/000021_add_task_paused_step.down.sql`:
```sql
ALTER TABLE tasks DROP COLUMN paused_step;
```

- [x] **Step 5: Task 2.3's `pauseForClarification` (mỗi step) set thêm `PausedStep: &stepID`**

Sửa lại call `Update` trong `pauseForClarification` (cả 3 file `cli_analyze.go`/`cli_spec.go`/`cli_implement.go` từ Task 2.3) để truyền thêm:

```go
	pausedStep := s.ID()
	if _, err := s.tasks.Update(ctx, s.rt.Task.ID, models.UpdateTaskInput{
		Clarifications: raw,
		SpecStatus:     &specStatus,
		PausedStep:     &pausedStep,
	}); err != nil {
```

- [x] **Step 6: Sửa `TaskService.Clarify` (`service/task.go:215-250`)**

```go
func (s *TaskService) Clarify(ctx context.Context, id string, input models.ClarifyTaskInput) (*models.Task, error) {
	if strings.TrimSpace(input.Context) == "" {
		return nil, ErrValidation("context is required")
	}
	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var rounds []models.ClarificationRound
	if len(task.Clarifications) > 0 {
		_ = json.Unmarshal(task.Clarifications, &rounds)
	}

	var analysis models.TaskAnalysis
	if len(task.Analysis) > 0 {
		_ = json.Unmarshal(task.Analysis, &analysis)
	}

	newRound := models.ClarificationRound{
		Round:     len(rounds) + 1,
		Timestamp: time.Now(),
		Questions: analysis.ClarificationQuestions,
		Response:  input.Context,
	}
	rounds = append(rounds, newRound)
	clarificationsBytes, _ := json.Marshal(rounds)

	specStatus := models.TaskSpecStatusNone
	status := models.TaskStatusAnalyzing
	if task.PausedStep != "" {
		// CLI spec-first flow: resume at the step that actually paused,
		// not always "analyze" (REQ-006). workflow.StatusForStep mirrors
		// each step's own StatusOnResume — reuse it instead of duplicating
		// the step->status mapping here.
		status = workflow.StatusForStep(task.PausedStep)
	}
	clearedPausedStep := ""
	return s.repo.Update(ctx, id, models.UpdateTaskInput{
		SpecStatus:     &specStatus,
		Status:         &status,
		Clarifications: json.RawMessage(clarificationsBytes),
		PausedStep:     &clearedPausedStep,
	})
}
```

- [x] **Step 7: Kiểm tra `workflow.StatusForStep` đã tồn tại chưa — nếu chưa, tạo hàm tra cứu nhỏ dựa trên `StatusOnResume` của từng step**

```bash
grep -rn "StatusOnResume" server/internal/orchestrator/steps/*.go | grep -v _test
```

Nếu mỗi step's `StatusOnResume` là method trên instance (cần khởi tạo step để gọi, không tiện dùng từ `service/task.go` vốn không có dependency tới `orchestrator/steps`), thay bằng 1 map tĩnh đơn giản hơn trong package `workflow`:

```go
// StatusForStep returns the task status a paused-and-resumed job should be
// set to, keyed by which step raised the pause. Mirrors each step's own
// StatusOnResume without importing the steps package (workflow sits below
// orchestrator/steps in the dependency graph).
func StatusForStep(step string) string {
    switch step {
    case StepCLIAnalyze:
        return models.TaskStatusAnalyzing
    case StepCLISpec:
        return models.TaskStatusAnalyzing
    case StepCLIImplement:
        return models.TaskStatusCoding
    default:
        return models.TaskStatusAnalyzing
    }
}
```

(Xác nhận `workflow` package có được phép import `models` chưa trước khi viết — nhiều khả năng có, vì `PauseError`/step ID constants đã ở đó; nếu có cycle, đặt hàm này ở `models` package thay vì `workflow`.)

- [x] **Step 8: Chạy lại test, xác nhận pass**

Run: `go test ./server/internal/service/... ./server/pkg/models/... ./server/internal/workflow/... -v`
Expected: PASS toàn bộ, bao gồm mọi test cũ của `Clarify`/`analyze.go` (regression check bắt buộc — đây là service dùng chung 2 flow)

- [x] **Step 9: Commit**

```bash
git add server/pkg/models/task.go server/internal/service/task.go server/internal/workflow/*.go server/migration/000021_* server/internal/orchestrator/steps/cli_analyze.go server/internal/orchestrator/steps/cli_spec.go server/internal/orchestrator/steps/cli_implement.go server/internal/service/task_test.go
git commit -m "feat: resume paused CLI steps at their own status, not always Analyzing (REQ-006 part 2)"
```

### Task 2.5: Investigation — flag non-interactive thật của từng CLI
> Links to: REQ-007

**Chưa thực hiện — cần người vận hành làm thủ công.** Việc này đòi hỏi chạy thật 1 CLI agent (`claude -p`/`codex exec`/`antigravity`) trong sandbox với 1 task cố ý mơ hồ để quan sát hành vi — đây là lời gọi thật tới các dịch vụ AI trả phí, tốn quota/token thật và có thể mất vài phút, nên không tự ý chạy khi implement OpenSpec này (khác với Task 1.3, chỉ cần `--help` — không tốn quota). Để lại nguyên trạng cho người dùng chạy khi tiện, theo đúng tinh thần "Investigation trước, code sau" — không đoán kết quả.

- [ ] **Step 1: Chạy thử `claude`/`codex`/`antigravity` trong sandbox thật (`docker/`), `CI=1`, không stdin, với 1 task cố ý mơ hồ** (vd "cải thiện performance" không nói rõ file/module nào)
- [ ] **Step 2: Ghi lại quan sát vào bảng dưới, cập nhật `awaitingInputPatterns` (Task 2.2) nếu phát hiện pattern hỏi lại thật chưa được cover**

| CLI | Có treo chờ tty không? | Pattern hỏi lại quan sát được | Đã thêm vào `awaitingInputPatterns`? |
|---|---|---|---|
| claude | | | |
| codex | | | |
| antigravity | | | |

- [ ] **Step 3: Nếu phát hiện pattern mới, thêm vào `cli_question_detect.go` + test tương ứng, lặp lại chu trình test-first của Task 2.2**

## P2 — Medium
(none — Issue 8 từ `proposal.md`, "worktree reset giữa các lần retry", cố ý chưa viết task cụ thể tại đây; cần điều tra riêng trước khi có đủ bằng chứng để viết task không suy đoán, xem `proposal.md` Issue 8)

## P3 — Low
(none)

---

## Self-Review Checklist

1. **Spec coverage:** REQ-001→008 đều có ít nhất 1 task map ngược (Task 1.1→REQ-001, 1.2→REQ-002, 1.3→REQ-003, 1.4→REQ-008, 2.1→REQ-004, 2.2→REQ-005, 2.3+2.4→REQ-006, 2.5→REQ-007). REQ-008 phát sinh giữa chừng (Task 1.3's phát hiện phụ), không có trong bản `specs.md` gốc lúc mới viết OpenSpec — thêm sau khi user quyết định hướng xử lý.
2. **Placeholder scan:** Task 1.3/2.5 có bảng để điền — đây là investigation task hợp lệ (theo Authoring Decision Matrix, "Investigation" là 1 bước rõ ràng, không phải "TBD" mù), không phải placeholder che giấu việc chưa nghĩ ra code.
3. **Type consistency:** `CodeStepResult.{AuthInvalid,AwaitingInput}`, `models.Task.PausedStep`, `steps.CLIStepOutput.AwaitingInput` — tên nhất quán xuyên suốt design.md/specs.md/tasks.md.
4. **File paths:** Mọi path tham chiếu tới file/dòng đã xác nhận tồn tại tại thời điểm viết OpenSpec này (xem trace trong `proposal.md`) — Task 2.3 Step 1 và Task 2.4 Step 7 cố ý yêu cầu `grep` xác nhận lại trước khi code, vì tên struct/hàm chính xác chưa được đọc trực tiếp lúc viết OpenSpec này.

## Implementation notes (deviations từ plan gốc, ghi lại sau khi code thật)

- **Task 2.3**: thay vì viết `pauseForClarification` như 1 method riêng lặp lại 3 lần trên `*CLIAnalyzeStep`/`*CLISpecStep`/`*CLIImplementStep` (như draft ban đầu), gộp thành 1 hàm dùng chung `pauseForClarification(ctx, tasks, task, stepID, cliOutput)` trong file mới `server/internal/orchestrator/steps/cli_clarification.go` — cả 3 step đều ở `package steps` nên dùng chung được, tránh lặp code 3 lần.
- **Task 2.4**: `TaskRepo.Update` tự gọi lại `GetByID` bên trong (không thấy rõ khi viết plan) — mọi test `Clarify` qua sqlmock cần khai báo 2 lần `ExpectQuery` cho SELECT (1 lần trong `Clarify` trực tiếp, 1 lần trong `Update`), theo đúng thứ tự gọi thật, không phải 1 lần như phác thảo ban đầu.
- **Task 1.3 (REQ-003)**: đã verify được lệnh thật (`claude auth status`, `codex login status`) bằng CLI binary có sẵn trên máy — nhưng cả 2 lệnh này exit code luôn 0 bất kể trạng thái login, nên `Preflight` (`engine/cli.go`) được mở rộng thêm content-check tái dùng `detectAuthInvalid` (Task 1.2), không chỉ đổi tên lệnh như dự kiến ban đầu. Phát hiện phụ: `antigravity` profile's `Args: []string{"run", ...}` có thể sai (không tìm thấy subcommand `run` thật) — ghi nhận, không tự sửa (ngoài scope REQ-003).
- **Task 2.5 (REQ-007)**: không thực hiện — đòi hỏi chạy CLI agent thật (tốn quota/token), để lại cho người vận hành.
