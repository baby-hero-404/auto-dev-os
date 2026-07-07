# PLAN-phase18-global-path-manager.md

## 1. Objective
Refactor the Auto Code OS path management system by implementing a **Domain-Driven Path Registry** with **Interface Segregation**. 
This will prevent the "God Object" / "Service Locator" anti-pattern, replace raw strings with immutable semantic `Path` objects, enforce strict lifecycle boundaries (Runtime vs Build-time vs Resources), and completely isolate filesystem side-effects from domain logic for pure testability.

## 2. Design Principles
- **Value Objects are Immutable**: Paths (`Directory`, `File`) do not mutate and have no side effects.
- **No I/O in Value Objects**: Filesystem side effects (`Exists()`, `EnsureDir()`) belong to a `FileSystem` interface, not the Path object itself. Value objects must not import `os`.
- **Registry is for Composition Only**: The `PathRegistry` is ONLY used during dependency composition (e.g., in `main.go`). **Registry MUST NOT be injected into business services.**
- **Business Services Depend Only on Interfaces**: Services inject specific domains (e.g., `PromptPaths` or `WorkspacePaths`), never the whole registry.
- **Path Providers MUST NOT Expose Raw Strings**: Business code uses semantic types (`File`, `Directory`) rather than low-level string manipulation.

## 3. Current Architecture Flaws
1. **Directory as a Service**: Path objects perform filesystem operations (`os.Stat`, `os.MkdirAll`), violating DDD principles where Value Objects must be pure.
2. **Service Locator Risk**: Passing a monolithic `Manager` deep into the application creates a God Object and exposes implementation details.
3. **File vs Directory Blur**: `PromptPaths` returning `Directory` for files (like `planner.md` or `cache.db`) is semantically incorrect.
4. **Weak Root Invariants**: Without dedicated value objects, `filepath.Join("../../")` can silently escape intended workspace boundaries (Directory Traversal Vulnerability).
5. **Testing Friction**: Tightly coupled `os` calls force tests to run on real disks instead of taking advantage of Go 1.16+ `io/fs` and `fstest.MapFS`.

## 4. Proposed Architecture

### 4.1 Immutable Semantic Value Objects (Pure)
Instead of returning `string` or calling `os`, all path providers return pure, immutable value objects.
```go
package paths

type Path interface {
    String() string
}

type Directory struct { path string }
// Child MUST NEVER escape the root invariant. It must securely sanitize input.
func (d Directory) Child(elem ...string) Directory 
func (d Directory) File(name string) File
func (d Directory) String() string

type File struct { path string }
func (f File) String() string
```

### 4.2 The FileSystem Service (Side Effects)
All IO operations are delegated to a swappable interface (enabling `fstest.MapFS` for testing).
```go
type FileSystem interface {
    Exists(p Path) bool
    EnsureDir(d Directory) error
}
```

### 4.3 Domain-Driven Interfaces (ISP)
Consumers will only depend on the strict interface they need, returning semantic types.
```go
type WorkspacePaths interface {
    TaskRoot(taskID string) Directory
    RepoRoot(taskID, repo string) Directory
}

type PromptPaths interface {
    Root() Directory
    RolePrompt(role string) File
    StepPrompt(step string) File
}

type SkillPaths interface {
    Root() Directory
    GitRepo(repoName string) Directory
}

type DatabasePaths interface {
    CacheDB() File // SQLite cache is a file
}

type MigrationSource interface {
    Root() Directory // Abstracted to support go:embed easily later
}
```

### 4.4 The Path Registry (Composition Root ONLY)
A central registry acts as a dependency injection container.
```go
// ONLY used in main.go during initialization
type PathRegistry struct {
    Workspace WorkspacePaths
    Prompt    PromptPaths
    Log       LogPaths
    Skill     SkillPaths
    Migration MigrationSource
    FS        FileSystem
}
```

## 5. Implementation Strategy (Introduce ➔ Deprecate ➔ Delete)

### Phase 1: Core Definitions (Introduce)
- [x] 1. Create `server/pkg/paths/types.go` (Immutable `Directory` and `File` objects).
- [x] 2. Create `server/pkg/paths/fs.go` (The `FileSystem` interface and OS implementation).
- [x] 3. Create `server/pkg/paths/interfaces.go` (Domain interfaces: `WorkspacePaths`, `PromptPaths`).
- [x] 4. Create `server/pkg/paths/testing.go` (Leverage `io/fs` and `fstest.MapFS` for pure in-memory testing).

### Phase 2: Compatibility Layer (Deprecate)
- [x] 1. Do **NOT** move `orchestrator/workspace/pathmanager.go` or `service/skill_paths.go` yet.
- [x] 2. Update them to act as a **Compatibility Wrapper** around the new `paths` package. 
- [x] 3. Mark the legacy structs/functions with explicit tooling comments:
   `// Deprecated: since Phase18. Remove after Phase19. Use pkg/paths.WorkspacePaths instead.`

### Phase 3: Consumer Interface Injection
- [x] 1. Update `PromptAssembler` to accept `paths.PromptPaths` (No knowledge of the Registry).
- [x] 2. Update `SkillService` to accept `paths.SkillPaths`.
- [x] 3. Update `main.go` to construct the `PathRegistry`, and inject only the specific interface implementations into each service.

### Phase 4: Strong Verification
Introduce comprehensive test cases for the `Directory` object, `FileSystem`, and Registry:
- [x] **Root Invariants**: Guard against directory traversal (`Child("../../")` MUST safely confine to root).
- [x] **Concurrency (Race)**: Test `Concurrent EnsureDir()` to guarantee safety when scheduler and workers create directories simultaneously.
- [x] **Path Normalization**: Validate trailing slashes and redundant `/./`.
- [x] **OS Separators**: Guarantee correct handling of Windows `\` vs Linux `/`.
- [x] **Symlinks**: Test `Abs()` resolution behavior.
- [x] **Missing Directories**: Validate `FileSystem.EnsureDir()` and `FileSystem.Exists()`.

### Phase 5: Cleanup (Phase 19 - Future)
- [x] Once the entire system has successfully transitioned to injecting domain interfaces, safely delete the deprecated `pathmanager.go` and `skill_paths.go`.
