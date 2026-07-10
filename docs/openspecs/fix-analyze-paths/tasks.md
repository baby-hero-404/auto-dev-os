# Tasks: Fix Analyze Tool Path Resolution

## P0 — Critical

### Task 1.1: Refactor `listAnalyzeFiles` and `grepAnalyzeFiles`
> Links to: REQ-M01, REQ-M02

**Acceptance Criteria:**
- [x] Remove the `s.sandbox.RunCommand` mock execution for `find .`
- [x] Use `WorkspaceLoader` and `pkg/paths.OSWorkspacePaths` to resolve all repos.
- [x] Ensure only `repo.Paths.Main` files are iterated and output.

### Task 1.2: Refactor `AnalyzeStep.executeAnalyzeTool`
> Links to: REQ-M01

**Acceptance Criteria:**
- [x] Ensure `workspacePath` uses the `paths` abstraction.
- [x] Stop doing manual `filepath.Join` on raw strings that can result in `.` prefixes.

### Task 1.3: Update Tests
> Links to: REQ-M02

**Acceptance Criteria:**
- [x] Modify `analyze_test.go` to mock the file system state by using `code/repos/repo-a/main` properly.
- [x] Validate test success against the actual native filesystem operations instead of relying on sandbox invocation tracking.

### Task 1.4: Fix Agent Role in `AnalyzeStep` Prompt Assembly
> Links to: REQ-M01

**Acceptance Criteria:**
- [x] Intercept `s.rt.Agent` in `AnalyzeStep.Execute` and override its Role to `models.AgentRolePlanner` to ensure the `# Planner Role` text is appended to the system prompt, regardless of the user-assigned task agent role.
