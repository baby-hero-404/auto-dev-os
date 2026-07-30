# Headless CLI Tools Overview & Comparison

This document provides a high-level comparison and recommended server setup for the three supported headless CLI tools. For detailed instructions on each tool, please refer to their respective guides:
- [Antigravity CLI Guide](file:///home/ubuntu/my_projects/auto_code_os/docs/guides/antigravity-cli-headless.md)
- [Codex CLI Guide](file:///home/ubuntu/my_projects/auto_code_os/docs/guides/codex-cli-headless.md)
- [Claude CLI Guide](file:///home/ubuntu/my_projects/auto_code_os/docs/guides/claude-cli-headless.md)

---

## Quick Comparison (Linux Headless)

| Tool | Install | Headless invocation | Model Selection | Auto-approve flag | Best use |
|------|---------|----------------------|-----------------|--------------------|----------|
| Antigravity (`agy`) | `install.sh` | `agy -p "<task>"` | `--effort high\|medium\|low` | `--dangerously-skip-permissions` | Agent workflow + skills |
| Codex CLI (`codex`) | `install.sh` | `codex exec "<task>"` | `--model gpt-4o` | `--full-auto` | Automation + coding |
| Claude Code (`claude`) | `install.sh` | `claude -p "<task>"` | `--model claude-3-7-sonnet...` | `--allowedTools "Read,Edit,Bash"` | Long context + repo agent |

**Common trait**: all three take the prompt as a **literal text argument** in their basic headless form — none of them accept a bare file path as the prompt and read its contents automatically. If you need to hand a tool a large prompt without hitting the shell's `ARG_MAX`/`MAX_ARG_STRLEN` limits, write the task to a file and give the CLI a short literal instruction telling it to open and follow that file (all three are agentic and can read files themselves) — don't pass the file path as the entire prompt.

**Model Configurations:**
- **Antigravity**: Abstracts model selection via reasoning effort (`--effort high|medium|low`), auto-mapping to Gemini tiers.
- **Codex**: Supports explicit model assignment via `--model` or `OPENAI_MODEL` environment variable.
- **Claude Code**: Supports explicit model assignment via `--model` or `ANTHROPIC_MODEL` environment variable.

**Auto-approve flags differ per tool** — there is no shared "skip confirmation" flag name:
- Antigravity: `--dangerously-skip-permissions`
- Codex: `--full-auto`
- Claude: requires explicit `--allowedTools "Read,Edit,Bash"` (Claude has no single "skip all" flag; it's an allow-list)

> ⚠️ Treat all three auto-approve/allow-list flags the same way: only use them in controlled/sandboxed environments (CI, disposable containers), never on a machine with untrusted input reachable by the agent.

---

## Recommended Server Setup

```text
Linux Server
     |
     |
+----+----+
|         |
Codex   Claude
|         |
CI/CD   Review
|
Automation

Antigravity
|
Multi-agent workflow
```

### Production AI Dev Stack

**Architecture**
```bash
agy --dangerously-skip-permissions -p "Review architecture"
```

**Implementation**
```bash
codex exec "Implement logic"
```

**Review / Debug**
```bash
claude -p
```

**CI Pipeline (e.g. GitHub Actions)**
```text
GitHub Actions
      |
      +-- codex exec
      |
      +-- claude -p
```

*Bộ 3 công cụ này đặc biệt phù hợp cho Linux server không GUI, môi trường SSH, Docker, CI/CD pipeline, và các autonomous coding agent.*
