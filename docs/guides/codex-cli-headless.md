# OpenAI Codex CLI - Linux Headless Guide

This document provides a concise guide for installing and using the headless mode (Linux) of OpenAI Codex CLI. This tool is optimized for server environments, CI/CD pipelines, and automation scripts without a graphical UI.

## Install

```bash
curl -fsSL https://chatgpt.com/codex/install.sh | sh
```

**Verify:**
```bash
codex --version
```

**Start:**
```bash
codex
```

## Authentication

**Device Code Login (Recommended for Headless):**
```bash
codex login --device-auth
```
*Lưu ý: Bạn cần bật tính năng xác thực mã thiết bị (device code authentication) cho Codex trong Cài đặt bảo mật (Security Settings).*

**Alternative - Set API key:**
```bash
export OPENAI_API_KEY="your_api_key"
```

**Permanent API key:**
```bash
echo 'export OPENAI_API_KEY="your_api_key"' >> ~/.bashrc
source ~/.bashrc
```

## Headless Mode

**Execute one task**
```bash
codex exec "Analyze this repository"
```

**Specifying a Model**
```bash
codex exec --model gpt-4o "Analyze this repository"
```
*(You can also set the `OPENAI_MODEL` environment variable to define the default model for headless runs).*

**Code modification**
```bash
codex exec "Implement JWT authentication"
```

**Automation mode**
```bash
codex exec --full-auto "Fix tests and commit changes"
```

## Useful Commands

| Command | Purpose |
|---------|---------|
| `codex` | Interactive mode |
| `codex exec` | Headless execution |
| `codex app` | Desktop app |
| `codex --help` | Help |
| `codex update` | Update |

## Common Workflow

**Review code**
```bash
codex exec "Review current changes and report bugs"
```

**Generate tests**
```bash
codex exec "Generate missing unit tests"
```

**Refactor**
```bash
codex exec "Refactor this module safely"
```

## CI/CD Example
```bash
#!/bin/bash

codex exec \
--full-auto \
"Analyze CI failure, fix code, run tests"
```
