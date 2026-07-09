# Specs: Simplify Repository Workspace Paths

## Modified Requirements

### REQ-M01: Static Main Integration Path
> ❌ Status: Not Started

**Scenario:**
- WHEN the orchestrator generates paths for the main integration repository checkout
- THEN the path should unconditionally be `code/repos/<repo_name>/main`
- AND the dynamic branch parameter should not be accepted by path generation interfaces

### REQ-M02: Path Normalization
> ❌ Status: Not Started

**Scenario:**
- WHEN `RepoRelativeToWorkspace` is called
- THEN it should generate `code/repos/<repo_name>/main/<repo_path>` without requiring a branch name
- AND `WorkspaceToRepoRelative` should correctly parse `main` as the integration directory without relying on `FindRepoMainBranchDir`

## Removed Requirements

### REQ-R01: FindRepoMainBranchDir
- The utility function `FindRepoMainBranchDir` is removed as the main directory is now statically named `main`.
