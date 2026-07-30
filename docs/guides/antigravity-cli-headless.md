# Antigravity CLI (agy) - Linux Headless Guide

This document provides a concise guide for installing and using the headless mode (Linux) of Antigravity CLI. This tool is optimized for server environments, CI/CD pipelines, and automation scripts without a graphical UI.

## Install (Linux)

```bash
curl -fsSL https://antigravity.google/cli/install.sh | bash
```

**Verify:**
```bash
agy --version
```

## Authentication

**Login:**
```bash
agy login
```
*(The CLI will open a browser or provide a URL to authenticate via Google Accounts.)*

## Run Headless Mode

Dùng cho:
- Server
- CI/CD
- Automation
- Remote SSH
- Không cần UI/TUI

**Basic non-interactive mode**
```bash
agy --dangerously-skip-permissions -p "Analyze this project and create a plan"
```
*(Note: In headless mode, tools like `read_file` cannot prompt for approval and will be auto-denied unless `--dangerously-skip-permissions` is used or explicit allow-rules are set).*

**Execute single task**
```bash
agy --dangerously-skip-permissions -p "Analyze this project and create a plan"
```

**Auto approve changes**
```bash
agy \
--dangerously-skip-permissions \
-p "Implement feature X"
```
> ⚠️ **Warning:** Chỉ dùng `--dangerously-skip-permissions` trong môi trường kiểm soát.

## Project Usage

```bash
cd /path/to/project

agy --dangerously-skip-permissions -p "Review this repository and explain architecture"
```

## Useful CLI Flags

| Flag | Usage |
|------|-------|
| `-p` / `--print` | Chạy 1 prompt non-interactive và in kết quả (flag `-p` phải đặt cuối cùng trước prompt) |
| `--mode plan` | Chỉ lập kế hoạch |
| `--mode accept-edits` | Tự động sửa file |
| `--sandbox` | Giới hạn quyền |
| `--dangerously-skip-permissions` | Bỏ confirm |
| `--effort high\|medium\|low` | Điều chỉnh reasoning effort |
| `-c` / `--continue` | Tiếp tục conversation gần nhất |
| `--conversation <id>` | Resume 1 conversation theo ID |
| `--add-dir <path>` | Thêm thư mục vào workspace (repeatable) |

## Recommended Headless Modes

**Planning**
```bash
agy \
--mode plan \
-p "Design architecture for this project"
```

**Coding**
```bash
agy \
--mode accept-edits \
-p "Implement API authentication"
```

**Automation**
```bash
agy \
--dangerously-skip-permissions \
-p "Fix tests and run CI checks"
```

## Skills Structure

Project structure:
```text
.agy/
└── skills/
    ├── architect/
    │   └── SKILL.md
    ├── developer/
    │   └── SKILL.md
    ├── reviewer/
    │   └── SKILL.md
    ├── tester/
    │   └── SKILL.md
    └── devops/
        └── SKILL.md
```

## Common Headless Workflows

**Code Review**
```bash
agy --dangerously-skip-permissions -p "Load reviewer skill. Review current code changes."
```

**Generate Tests**
```bash
agy --dangerously-skip-permissions -p "Load tester skill. Generate missing tests."
```

**Refactor**
```bash
agy --dangerously-skip-permissions -p "Load developer skill. Refactor this module safely."
```

**DevOps**
```bash
agy --dangerously-skip-permissions -p "Load devops skill. Create Docker and CI configuration."
```

## CI/CD Example
```bash
#!/bin/bash

agy \
--dangerously-skip-permissions \
--effort high \
-p "Analyze failures, fix code, run tests"
```

## Recommended Production Flow

Request -> Architect Skill -> Developer Skill -> Tester Skill -> Reviewer Skill -> DevOps Skill -> Goal

Antigravity CLI chạy Linux headless trở thành:
- AI Developer
- AI Code Reviewer
- AI Tester
- AI DevOps Agent
- CI/CD Automation Agent

*Lưu ý: các flag ở trên đã được xác nhận trực tiếp từ `agy --help` (không phải suy đoán). Vẫn nên chạy `agy --help` khi update lên version mới để phát hiện sớm nếu flag đổi tên.*
