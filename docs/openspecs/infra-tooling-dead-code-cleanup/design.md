# Design: Infra/Tooling Dead Code Cleanup

## Issue 1-2, 6: Xoá thẳng (đã verify 0 caller)

Cùng quy trình `orchestrator-dead-code-cleanup/design.md`: re-verify grep ngay trước khi xoá → xoá → `go build`+`go vet`+`go test ./...`.

```bash
grep -rln "auto-code-os/server/internal/evals" server/          # kỳ vọng: chỉ file trong chính package
grep -rn "SkillExecutorAdapter\|SkillCall\b\|SkillResult\b" server/
grep -rn "\.CloneRepo(" server/ | grep -v "GitProvider\|gitOps\.CloneRepo\|adapter\.provider"
grep -rn "\bdatabase\.Connect(" server/
grep -rn "RepoRelativeToWorkspace\|IsWorkspaceInternalPath" server/
```

## Issue 3: `ParseGoBuildOutput` dùng chung

```go
// internal/tool/build_output.go (mới)
var goCompilerErrorRegex = regexp.MustCompile(`^([^:\s]+):(\d+):(?:\d+:)?\s*(.*)$`)

func ParseGoBuildOutput(output []byte) []Diagnostic {
	// thân hàm = hợp nhất logic hiện có ở compile_check.go + run_build.go,
	// giữ đúng field mapping hiện tại (không đổi shape Diagnostic)
}
```
`compile_check.go`'s `CompileCheckHook` và `run_build.go`'s `RunBuildTool` đều gọi hàm này thay vì tự parse — xoá 2 regex/2 vòng lặp trùng lặp.

## Issue 4: `pkg/graph` dùng chung cho 3 `ValidateDAG`

```go
// pkg/graph/cycle.go (mới)
package graph

// Node is anything with an ID and a list of dependency IDs — the minimal
// shape a cycle-detector needs, regardless of what the caller's real
// struct looks like (workflow.Definition step, governance.StepSpec,
// models.ExecutionUnit all differ beyond this).
type Node interface {
	ID() string
	Deps() []string
}

// DetectCycle returns the cycle path (as IDs) if one exists, or nil.
// Shared by internal/workflow, internal/governance, internal/policy —
// each keeps its own domain-specific checks (structural validation,
// cost thresholds) layered on top of this.
func DetectCycle(nodes []Node) []string { ... }
```
3 package gọi domain wrap quanh `Node` interface (adapter nhỏ per package, không đổi shape dữ liệu gốc của họ) rồi gọi `graph.DetectCycle`. Giữ nguyên các check khác của từng package (governance's 4 structural check, policy's cost threshold) — chỉ phần cycle-walking dùng chung.

**Không đổi tên hàm public `ValidateDAG` ở 3 package** (giữ API tương thích ngược cho caller hiện có) — chỉ đổi phần thân implementation.

## Issue 5: `governance.Extends` — cần quyết định trước

Không thiết kế cụ thể cho tới khi chọn hướng (a) hay (b) — đây là quyết định sản phẩm (có cần 2 preset thật sự khác nhau về step-shape không), không phải quyết định kỹ thuật thuần. Nếu (a): cần thiết kế riêng cho việc mỗi preset áp step-shape gì — vượt phạm vi "cleanup", nên nếu chọn hướng này, tách thành OpenSpec riêng thay vì làm trong lúc dọn dead code.

## Trade-offs

- **`pkg/graph` là refactor rủi ro cao nhất trong OpenSpec** (chạm 3 package đang hoạt động, không phải pure-delete) — làm sau cùng, sau khi các item pure-delete đã xong và review riêng.
- **Không tự chọn hướng cho `Extends`** — đây là quyết định sản phẩm, để user/team quyết, không đoán.
