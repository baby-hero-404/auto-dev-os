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
| `-p` / `--print` | Chạy 1 prompt non-interactive và in kết quả (với v1.1.8+, có hỗ trợ output stream) |
| `--output-format <fmt>` | Format cho kết quả in (vd: `text`, `json`, `stream-json`) |
| `--json-schema <path>` | Ép output trả về dạng JSON theo schema (hữu ích cho CI/CD) |
| `--mode plan` | Chỉ lập kế hoạch (`PLAN.md`) |
| `--mode accept-edits` | Tự động sửa file không hỏi line-by-line review |
| `--sandbox` | Giới hạn quyền terminal (chặn các lệnh nguy hiểm) |
| `--dangerously-skip-permissions` | Bỏ confirm (BẮT BUỘC dùng cho automation/CI) |
| `--effort high\|medium\|low` | Điều chỉnh reasoning effort của agent |
| `-c` / `--continue` | Tiếp tục conversation gần nhất (Dùng kèm `-p` để chạy multi-turn headless) |
| `--conversation <id>` | Resume 1 session cụ thể (thay thế cờ `--resume` cũ) |
| `--add-dir <path>` | Kéo thư mục cụ thể vào context của session |
| `--agent <agent>` | Dùng profile agent cụ thể (e.g. `frontend`, `reviewer`) |
| `--print-timeout <dur>` | Thay đổi timeout cho `-p` (mặc định 5m0s). Tăng lên `30m` cho task dài. |
| `--project <id>` | Bắt ép chạy trên context project ID cụ thể |

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

**Automation (JSON Output)**
```bash
agy \
--dangerously-skip-permissions \
--output-format json \
--json-schema ./schema.json \
-p "Analyze project health and return JSON report"
```

## Advanced Automation Patterns

### 1. Multi-turn Continuity (Chaining headless turns)
Bạn có thể kết hợp `-c` và `-p` để agent làm việc liên tiếp nhiều turn mà không cần TUI:
```bash
# Turn 1: Khởi tạo
agy -p "Initialize project" --dangerously-skip-permissions

# Turn 2: Tiếp tục với context từ Turn 1 (Chỉ in ra kết quả mới, không in lại lịch sử)
agy -c -p "Now add index.html" --dangerously-skip-permissions
```

### 2. Explicit Content Injection
Vì Agent duy trì context session độc lập, việc `cd` vào folder không làm thay đổi focus. Để đảm bảo agent đọc đúng file trong CI/CD:
```bash
agy -p "$(cat README.md)\n\nSummarize this file." --dangerously-skip-permissions
```

### 3. Workspace Scoping
Nếu agent bị kẹt ở context project cũ, hãy dùng `--add-dir` để ép nạp thư mục hiện tại:
```bash
agy -p "Review code" --add-dir ./src --dangerously-skip-permissions
```

## Custom Agents & Plugins

Từ bản v1.1.6, Subagents/Skills được cấu trúc dưới dạng file Markdown (`agent.md`) chứa YAML Frontmatter thay vì `agent.json` như trước. Agent được hệ thống tự động nhận diện nếu đặt ở `~/.gemini/config/`.

**Quản lý Plugin (Thay thế Gemini CLI Extensions):**
- Import CLI cũ: `agy plugin import gemini`
- Cài plugin: `agy plugin install <target>`
- Bật/Tắt: `agy plugin enable/disable <name>`

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

# Dùng G1 Credits nếu hết AI credits, và kéo dài timeout cho build task.
agy \
--dangerously-skip-permissions \
--effort high \
--print-timeout 15m \
--output-format stream-json \
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
