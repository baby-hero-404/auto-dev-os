# Comprehensive Flow Review Report

*Date: 2026-07-23 (corrected & verified against source)*

This report documents a comprehensive deep-dive into the core execution flows of the Auto Code OS orchestrator (Worker loop, CLI engine, state machine, and PR steps). Every finding below has been re-verified against the current codebase; findings that were already fixed or turned out to be inaccurate have been removed or corrected.

> **Removed from the original report:**
> - *smart-llm-router blind retries* — fixed: `chatWithRetry` now short-circuits on `!IsTransientError(err)` (`server/pkg/llm/router.go`).
> - *Missing default group validation* — fixed: `NewGateway` now fails fast when `g.chains[g.defaultLevelGroup]` has no configured chain.
> - *Multi-repo git operations flaw* — **inaccurate**: `patch.Runner.GetChangedFiles`, `CaptureWorkspaceDiff`, and `CapturePRDiff` (`server/internal/orchestrator/patch/diff.go`) all handle the `task.RepositoryID == nil` case by iterating `ws.Repos`/`ListRepositories` and resolving per-repo paths; the workspace root is never used directly as a `git diff` target in the main execution path. (A cosmetic leftover remains: `repoutil.Validate` uses `m.WorkspaceRoot` as a dummy fallback base path for multi-repo patch validation — worth tidying, not a crash.)

## 1. 🚨 Critical Issue: CLI Engine `ARG_MAX` Vulnerability

**Location:** `server/internal/orchestrator/engine/cli.go` (`RunCodeStep`)

**Description:**
The CLI engine was designed to write the LLM instruction/prompt to a file (`.autocode/prompt.md`) to avoid passing massive strings directly as arguments to the user's CLI command. The code comment explicitly states this is "to avoid shell-escaping/length limits".

However, the implementation actually injects the base64-encoded prompt *directly into the bash script* that is passed to `bash -lc`:
```go
script := fmt.Sprintf(
    "cd %s && mkdir -p %s && printf '%%s' '%s' | base64 -d > %s && ...",
    ...
    encodedPrompt, // <--- Injected directly into the bash script string!
    ...
)
result, err := e.runtime.Run(ctx, sandbox.CommandRequest{
    Command: []string{"bash", "-lc", script}, // script is argv[2]
})
```

**Impact:**
The limit is even tighter than overall `ARG_MAX`: on Linux, a *single* argv string is capped at `MAX_ARG_STRLEN` (128KB). Since base64 inflates the payload by 4/3, a raw prompt of only ~96KB already exceeds the cap. With features like `repomap-mention-boost` and large contexts, prompts can reach this size, at which point `execve` fails with `E2BIG` and `RunCodeStep` crashes entirely.

**Recommendation:**
Since `req.HostWorkspace` is bind-mounted to `req.ContainerWorkDir`, the orchestrator has direct file-system access to the workspace. The engine should use standard Go `os.WriteFile` to write the prompt directly to the host directory (e.g., `<HostWorkspace>/.autocode/prompt.md`) *before* spawning the sandbox process, completely removing the prompt from the `bash -lc` command line.

---

## 2. 🚨 Critical Issue: Sandbox Policy Bypass - Secret Exfiltration

**Location:** `server/internal/sandbox/policy.go` (`validateExecutionPolicy`) & `server/internal/sandbox/docker.go` (`Run`)

**Description:**
The sandbox is designed to prevent data exfiltration by blocking network access if secrets are injected (`len(req.SecretEnv) > 0`).
However, there is a fatal logic mismatch between the policy validator and the actual Docker runner. In `policy.go`, if `req.NetworkMode == NetworkModeDefault`, the policy assumes the network will be `none` and allows secrets to be injected. But in `docker.go`, if `NetworkModeDefault` is used and the global orchestrator setting `DisableNetworking` is `false` (the standard setup to allow `npm install`), the runner actually assigns **`host` networking** to the container — not even the isolated `bridge` mode, but the host's own network namespace:

```go
networkMode := container.NetworkMode("none")
if req.NetworkMode == NetworkModeBridge || (req.NetworkMode == NetworkModeDefault && !r.config.DisableNetworking) {
    networkMode = "host"
}
```

**Impact:**
Data Exfiltration. A compromised agent or malicious code can request `NetworkModeDefault` while requesting injected secrets. The policy will mistakenly approve it, but Docker will give the container full **host** network access — including the ability to reach services bound to `localhost` on the host (databases, the orchestrator API itself) in addition to POSTing the injected secrets to an external attacker-controlled server.

**Recommendation:**
Fix `policy.go` so it has access to the global `DisableNetworking` config, or pass the *resolved* network mode into `validateExecutionPolicy` rather than calculating it independently in two different places. Separately consider whether `host` (vs `bridge`) is ever the right mode for sandboxed agent code.

---

## 3. 🚨 Critical Issue: GitHub Webhook Authentication Bypass & Forgery

**Location:** `server/internal/handler/webhook.go` (`WebhookHandler.GitHub`)

**Description:**
The GitHub webhook endpoint attempts to authenticate incoming payloads by checking a custom header: `X-AutoCodeOS-Webhook-Token`.
However, GitHub Webhooks *do not* and *cannot* send custom headers like this. GitHub standardizes payload authentication by sending an HMAC SHA-256 signature in the `X-Hub-Signature-256` header, derived from the payload and a shared secret.

**Impact:**
Because GitHub cannot send the custom token header, users will either (1) be forced to leave `WEBHOOK_SECRET` unset so GitHub can actually hit the endpoint, or (2) the webhook will just reject all real GitHub traffic.
If left unset, the endpoint is completely unauthenticated and open to the internet. An attacker can easily spoof payloads (e.g., sending fake `pull_request` closed events or `issues` opened events) to trick the orchestrator into executing rogue tasks, spawning jobs, or corrupting state without any authentication.

**Recommendation:**
Refactor the authentication logic in `WebhookHandler.GitHub` to read the `X-Hub-Signature-256` header, compute the HMAC SHA-256 of the raw request body using `WEBHOOK_SECRET` as the key, and use `crypto/subtle.ConstantTimeCompare` to verify the signature matches before parsing the JSON payload.

---

## 4. 🚨 Critical Issue: Task Analysis Race Condition & Data Loss

**Location:** `server/internal/orchestrator/patch/path_normalizer.go` (`appendNewAffectedFiles`) & `server/internal/orchestrator/setup.go`

**Description:**
While `code_backend.go` correctly uses the concurrency-safe `updateTaskAnalysis` wrapper (which fetches a fresh row under `analysisMu` before mutating), the patch applier path completely bypasses it. `appendNewAffectedFiles` reads the stale in-memory `task.Analysis` struct, appends the new affected files, and pushes the result directly to the database via the raw callback wired in `setup.go` (`o.repoutil.UpdateTaskAnalysis` → `tasks.Update`), with no fresh read and no lock.

**Impact:**
If two steps for the same task run concurrently (e.g., frontend and backend agents), one step's legitimate update to `AffectedFiles` (via the safe wrapper) will be silently overwritten and lost when the applier blindly pushes its stale in-memory state. This destroys the tracking of affected files, corrupting the analysis state.

**Recommendation:**
Route `appendNewAffectedFiles` (and any other applier-side analysis writes) through the same concurrency-safe wrapper as the rest of the steps, or better yet, migrate the whole `TaskAnalysis` update mechanism to atomic database operations (e.g., jsonb merge) instead of client-side read-modify-write patterns.

---

## 5. 🚨 Critical Issue: Zombie Workspaces / Resource Leak

**Location:** `server/internal/orchestrator/wkspace/cleanup.go` & `server/internal/sandbox/docker.go`

**Description:**
In `docker.go`, Docker containers are spawned without mapping the host user's UID/GID (no `User` field in `ContainerCreate`), so processes run as the image's default user — typically `root`. Any generated files (build artifacts, `node_modules`, new code) saved into the bind-mounted `/workspace` are then owned by `root:root`.
Later, when the host orchestrator (running as a non-root daemon user like `ubuntu`) attempts to delete the workspace using `os.RemoveAll(targetAbs)` in `cleanup.go`, it fails due to `Permission denied` — and the worktree-level removals explicitly discard the error (`_ = os.RemoveAll(...)`), so the failure is silent.

**Impact:**
Resource leak. These undeletable "Zombie Workspaces" persist permanently, eventually filling the host's disk and causing `No space left on device` failures that require manual admin intervention. (Applies to deployments where the sandbox image defaults to root and the orchestrator runs as non-root — the common configuration.)

**Recommendation:**
Map the host execution user into the Docker sandbox (`User: "1000:1000"` or resolved at runtime) so generated files have the correct ownership, or use a cleanup container running as `root` to purge the workspace directories. At minimum, log `os.RemoveAll` failures instead of discarding them.

---

## 6. ⚠️ Moderate Issue: Docker Language Cache Mounts Are Read-Only

**Location:** `server/internal/sandbox/docker.go` (cache mount setup)

**Description:**
To speed up execution, the orchestrator mounts host language cache directories (`~/.npm`, `~/.cache/pip`, `~/.m2`, `~/.gradle`, `~/.cargo/registry`) into the sandbox container with `ReadOnly: true` for everything except Go's `/go/pkg/mod`.

**Impact:**
Package managers differ in how they handle a read-only cache:
- **npm** writes to its cache (`cacache`) during install and will fail with `EROFS` — broken.
- **maven/gradle** must download artifacts *into* `~/.m2`/`~/.gradle` — dependency resolution of anything not already cached fails — broken.
- **pip** detects the unwritable cache, emits a warning, and gracefully continues without caching — degraded but functional.

So sandboxed dependency installation is effectively broken for JavaScript and JVM projects (not Python, as the original report claimed).

**Recommendation:**
Make the npm/maven/gradle mounts writable (accepting host-cache mutation from sandbox runs), or keep them read-only and configure the package managers inside the sandbox image to use a container-local cache directory as overflow.

---

## 7. ⚠️ Moderate Issue: Global Mutex Contention in `updateTaskAnalysis`

**Location:** `server/internal/orchestrator/steps/services.go` (`analysisMu`)

**Description:**
`updateTaskAnalysis` uses a global, package-level `sync.Mutex` (`analysisMu`) to serialize read-modify-write updates to a task's analysis field. Because the critical section wraps two database round-trips (`tasks.GetByID` and `tasks.Update`), the lock is held for the full duration of both queries.

**Impact:**
All concurrent workflows *that update task analysis* serialize on this single process-wide lock, even when they belong to completely unrelated tasks. This does **not** stall the entire orchestrator (steps that don't touch analysis are unaffected — the original report overstated this), but under load with a slow database it becomes a real throughput bottleneck, and it scales with job concurrency.

**Recommendation:**
Replace the global mutex with per-task locking (e.g., a keyed mutex on `taskID`), or eliminate the client-side read-modify-write entirely via atomic jsonb updates / optimistic concurrency in the database layer. Note this must be fixed together with Issue 4 — a per-task lock only helps if *all* writers go through it.

---

## 8. ⚠️ Moderate Issue: GitHub Token's Base64 Form Not Redacted

**Location:** `server/internal/gitops/github.go` & `server/internal/gitops/client.go` (`sanitizeToken`)

**Description:**
When passing the GitHub token to Git commands, the orchestrator encodes it as `base64("x-access-token:" + token)` and passes it via `http.extraHeader=AUTHORIZATION: basic <b64>`. On failure, `sanitizeToken(value, token)` scrubs error output — but it only replaces the *literal* token string, never the base64-encoded form.

**Impact:**
Defense-in-depth gap rather than a proven leak: git's own error output does not normally echo its `-c` arguments, so the base64 header is unlikely to appear in captured stderr (the original report overstated this as a critical, guaranteed leak). However, the encoded credential is visible in process listings (`ps`) while git runs, and any future code path that logs the constructed command line would leak a trivially-decodable token.

**Recommendation:**
One-line hardening: make `sanitizeToken` also redact `base64("x-access-token:" + token)`. Longer-term, prefer passing the credential via `GIT_ASKPASS`/credential helper or an environment variable rather than argv.

---

## 9. ✅ Architecture Praise: Worker Panic Recovery & Locks

**Location:** `server/internal/orchestrator/worker.go` (`run`)

**Description:**
The main job execution loop uses a very robust `defer` block that catches panics, prints the stack trace, and guarantees that:
1. The assigned Agent is explicitly released (`o.agents.Release`).
2. The Workspace lock is explicitly released (`o.releaseWorkspaceLock`).
3. The job and task statuses are correctly marked as `failed`.

This implementation is highly resilient and ensures that panics (like nil pointer dereferences in deep steps) will not cause distributed deadlocks by holding onto agents or workspace locks forever.

---

## 10. ✅ Architecture Praise: Fallback Safe State Mutation

**Location:** `server/internal/orchestrator/steps/analyze.go` (Definition of Ready Gate)

**Description:**
The implementation of the `definition-of-ready-gate` handles bypasses seamlessly. When the DoR is bypassed, `hasClarifications` is safely overridden to `false`, allowing the pipeline to proceed. Importantly, if the task was otherwise eligible for auto-approval, it explicitly downgrades the status to `TaskSpecStatusReadyWithWarnings`, ensuring an accurate audit trail of the bypass without compromising high-risk lock checks.
