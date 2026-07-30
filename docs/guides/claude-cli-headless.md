# Claude Code CLI - Linux Headless Guide

This document provides a concise guide for installing and using the headless mode (Linux) of Claude Code CLI. This tool is optimized for server environments, CI/CD pipelines, and automation scripts without a graphical UI.

## Install

```bash
curl -fsSL https://claude.ai/install.sh | bash
```

**Verify:**
```bash
claude --version
```

**Start:**
```bash
claude
```

## Authentication

**Login:**
```bash
claude auth login
```

**Check:**
```bash
claude auth status
```

**Logout:**
```bash
claude auth logout
```

## Headless Mode

**Single command (Read-only)**
```bash
claude -p "Analyze this project"
```

**Single command (Auto-edit / Automation)**
```bash
claude -p "Analyze failing tests and fix them" --allowedTools "Read,Edit,Bash"
```
*(Lưu ý: Headless của Claude bắt buộc phải truyền cờ `--allowedTools` nếu bạn muốn AI được phép sửa file hoặc chạy lệnh mà không cần hỏi ý kiến con người).*

**Pipe input**
```bash
cat error.log | claude -p "Find the issue"
```

**Continue session**
```bash
claude -c
```

**Resume session**
```bash
claude -r session_id
```

## Useful Commands

| Command | Purpose |
|---------|---------|
| `claude` | Interactive mode |
| `claude -p` | Headless mode |
| `claude -c` | Continue session |
| `claude -r` | Resume session |
| `claude update` | Update |
| `claude agents` | Manage agents |

## Project Memory

Create a `CLAUDE.md` in your repository root.

**Example:**
```md
# Project Rules

- Use clean architecture
- Write tests
- Follow existing style
- Run tests before commit
```
*Claude automatically loads project instructions.*

## Automation Examples

**Code review**
```bash
claude -p "Review current git diff"
```

**Fix bugs**
```bash
claude -p "Analyze failing tests and fix them" --allowedTools "Read,Edit,Bash"
```

**Documentation**
```bash
claude -p "Generate API documentation" --allowedTools "Read,Edit"
```

## CI/CD Example
```bash
#!/bin/bash

claude -p "Analyze build failure and fix source code" --allowedTools "Read,Edit,Bash"
```
