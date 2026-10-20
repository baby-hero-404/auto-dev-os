# Sandbox Image Configuration & CLI Engine Setup

This directory contains the Dockerfile definitions for the agent execution sandbox environment used by **Auto Code OS**.

## Sandbox Image (`docker/Dockerfile.sandbox`)

The sandbox image provides an isolated runtime container (`agent` user, `/workspace` directory) where agent tasks and CLI execution engines run safely.

### Installing AI Coding CLI Tools (e.g., Claude Code, Cursor CLI)

To use the **CLI Execution Engine** (`execution_engine: "cli"`), the target CLI binary must be installed inside the sandbox container image.

#### 1. Installing Claude Code (`claude`)

Add Node.js global package installation to `Dockerfile.sandbox`:

```dockerfile
# Install Claude Code globally
RUN npm install -g @anthropic-ai/claude-code
```

Or for a custom image build:

```bash
docker exec -u 0 -it <sandbox_container> npm install -g @anthropic-ai/claude-code
```

#### 2. Authentication & Environment Configuration

When running CLI tools in sandbox mode:
- **Subscription & Session Auth**: CLI tools authenticate via persistent subscription sessions (e.g. `claude auth status`, `codex login status`, `agy models`), managed dynamically through the **AI Providers -> CLI Profiles** dashboard.
- **Auth Check Command**: Configure an optional `auth_check_command` (e.g., `claude auth status`) in project settings to verify authentication before task execution.
- **Headless Execution**: Auto Code OS automatically sets `CI=1` and closes `stdin` during CLI execution to prevent interactive browser authentication hangs.

#### 3. Recommended Project CLI Settings for Claude Code

In Project Settings -> **Execution Engine**:
- **Engine**: `CLI`
- **Command**: `claude`
- **Args**:
  ```text
  -p
  --dangerously-skip-permissions
  {prompt_file}
  ```
- **Timeout**: `30` (minutes)
- **Auth Check Command**: `claude auth status`
