# Sandbox Manager v2 — Runtime Orchestration Layer

## Current state (already exists, do not rebuild)

`server/internal/sandbox/` (1743 LOC):
- `Runtime` interface (`Run`, `RunInteractive`, `Prewarm`) — the seam the Orchestrator already codes against (`orchestrator/sandbox.go`, `orchestrator/orchestrator.go`). **This interface is the "Command Executor" from the proposal — it already exists.** Adding a `KubernetesRuntime`/`PodmanRuntime` later means implementing this interface, not inventing a new one.
- `DockerRuntime` — one fixed image (`auto-code-os-sandbox:latest`, built from `docker/Dockerfile.sandbox`) with **every** language preinstalled (node22, python3, JDK+maven, go via builder stage). No per-project image selection today.
- Workspace mount: already bind-mount, not copy (`WorkspacePath` + `/workspace` bind).
- Cache mounts: already exist but are **hardcoded and unconditional** — `docker.go` lines ~263-288 mount `~/.npm`, `~/.cache/pip`, `~/.m2`, `~/.gradle`, `~/.cargo/registry`, Go mod cache into every container regardless of project type.
- `policy.go` — command blocklist (destructive `rm`, `mkfs`, fork bombs) + secret/network-egress exclusion. This is "Bước 2 security" already, separate from the runtime-selection concern.
- `activity.go` — idle-timeout + loop-detection watchdog (kills stalled/looping containers). No equivalent in the reference designs — keep as-is.
- Credential injection (CLI OAuth files), session mounts, real-time log streaming — all present, all must keep working.

## Gap vs. the proposed architecture

| Proposed piece | Status | Where it plugs in |
|---|---|---|
| Project Detector | Missing | New — reads workspace root for marker files |
| Runtime Registry (manifest) | Missing | New — replaces hardcoded cache-dir map |
| Image Builder (layered base→runtime images) | Missing | Large — current image is a single monolith; switching to per-runtime images is a real infra change, not additive |
| Container Lifecycle | Exists | `DockerRuntime.Run`/`RunInteractive` |
| Cache Manager | Exists but hardcoded | Generalize using manifest's `cache:` list |
| Command Executor | Exists | `sandbox.Runtime` interface |
| Env Hooks (setup.sh/teardown.sh) | Missing | New — run manifest's `setup:` commands after container start, before agent command |
| Health Check | Missing | New — run manifest's healthcheck before handing off to agent |

## Decision: evolve, don't replace

`DockerRuntime` and the `Runtime` interface stay. A new `SandboxManager` sits **between** `Orchestrator` and `Runtime`:

```
Orchestrator
    │  (same call sites: runSandboxStep / runSandboxStepInWorktree)
    ▼
SandboxManager   (new: server/internal/sandbox/manager.go)
    ├── Detector   (detector.go)   — marker files → RuntimeID
    ├── Registry   (registry.go)   — loads runtimes/*/manifest.yaml
    ├── (image resolution: manifest.image, built by `make sandbox-images` — NOT built on-demand at request time)
    └── delegates actual exec to the existing sandbox.Runtime
    ▼
DockerRuntime.Run(CommandRequest{Image: resolved, ExtraCacheMounts: ..., SetupCmd: ...})
```

Key calls:
1. **No on-demand `docker build`.** Building images inside the hot request path adds minutes of latency and a new failure mode. Runtime images are built by a `Makefile`/CI step (`docker/runtimes/<id>/Dockerfile`) ahead of time, same as `Dockerfile.sandbox` today. The manager only *selects* an already-built image tag.
2. `CommandRequest` gains an `Image string` field (empty = current fixed-image behavior, so nothing breaks for callers that don't opt in) and `ExtraCacheMounts map[string]string` (host→container, additive to the existing hardcoded set, not a replacement — removing the hardcoded mounts would need auditing every existing caller first).
3. Setup/health-check commands run as an extra `Run()` invocation before the agent's real command, inside the same container lifecycle — not a separate hook mechanism, to avoid a second container-create path to maintain.
4. Detection happens **host-side**, reading the already-materialized workspace directory (`sandbox.WorkspacePath`) before any container is created — cheap, no container needed just to `ls`.

## Manifest format

`runtimes/<id>/manifest.yaml` (new top-level `runtimes/` dir, mirrors `docker/`):
```yaml
id: flutter
image: autocode/flutter:latest
detect: [pubspec.yaml]
cache:
  - host: ~/.pub-cache
    container: /home/agent/.pub-cache
  - host: ~/.gradle
    container: /home/agent/.gradle
setup: ["flutter pub get"]
healthcheck: "flutter doctor"
```
`registry.go` loads all manifests at startup (like `config.go` does for `SandboxConfig`), keyed by `id`, with a `detect` → `id` reverse index for the Detector.

## Phasing (pick one to start — this is too large for one pass)

1. **Detector + Registry + generalized Cache Manager**, wired into the *existing* single image (manifest `image:` defaults to today's fixed image if unset). No new Dockerfiles yet. Lowest risk, immediately gives config-driven cache mounts instead of the hardcoded map.
2. **Setup hooks + health check**, still on the single image (e.g. flutter manifest's `setup: ["flutter pub get"]` still runs, just inside the existing monolith image which would need flutter added to `Dockerfile.sandbox`).
3. **Per-runtime layered images** (`docker/runtimes/*/Dockerfile` FROM a shared `autocode-base`) + `Image` field wiring + build tooling. This is the expensive one (new CI/build step, image storage, `agy`/CLI-tool availability per image needs re-checking).

## Open items needing a call before coding

- Phase 1 only, or commit to all 3 up front?
- New runtimes to actually support beyond what `Dockerfile.sandbox` covers today — Flutter is the only one named in the proposal; Rust/Android aren't in the current image either. Confirm the target list (affects Phase 3 Dockerfile count).
- `runtimes/` manifests: check into git under repo root (like `docker/`), or under `server/internal/sandbox/runtimes/` (colocated with the Go code that loads them)?
