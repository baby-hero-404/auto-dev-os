# Proposal: Follow-up Deep Dive — Mô Hình Tích Hợp LLM & Tool Loop

## Why

OpenSpec `reference-analysis` đã hoàn thành 25/25 task, sinh ra 20 report `DISCOVERY-*.md` + master `README.md`. Trong quá trình review, phát sinh 3 khoảng trống cần đào sâu thêm mà scope gốc chưa cover chi tiết:

1. `README.md` tổng hợp có bảng so sánh architecture/testing/DX nhưng **chưa có bảng so sánh riêng về mô hình tích hợp LLM** (ai giữ tool-loop, ai trả token API) — đây là quyết định kiến trúc quan trọng nhất khi Auto Code OS cân nhắc có nên cho phép "outsource" trí tuệ cho CLI agent người dùng tự cài hay không.
2. `DISCOVERY-aider.md` và `DISCOVERY-hermes-agent.md` đã có overview kiến trúc nhưng phần **tool loop (build prompt → gọi LLM → parse tool call → apply edit → reflection)** còn ở mức khái quát, chưa đủ chi tiết implementation để so sánh trực tiếp với `server/internal/orchestrator/llmrunner/toolloop.go` của Auto Code OS.
3. `DISCOVERY-multica.md` và `DISCOVERY-ai-sdlc.md` đã mô tả cơ chế spawn CLI subprocess, nhưng **chưa làm rõ cơ chế multi-account** — 1 process cha có quản lý được nhiều phiên CLI con với credential/session khác nhau (vd nhiều tài khoản Claude) hay không, và bằng cách nào (env var override, config file riêng, working-dir isolation...).

## What Changes

### Issue 1: Bảng so sánh 4 mô hình kiến trúc tích hợp LLM
- Thêm section "🔌 Mô Hình Tích Hợp LLM (4 Kiến Trúc)" vào `docs/references/README.md`: Subprocess-CLI / SDK-embedded / API-native / Hybrid (pluggable spawner)
- Map từng project reference vào đúng mô hình, kèm trade-off

### Issue 2: Đào sâu tool loop của Aider
- Bổ sung `DISCOVERY-aider.md`: chi tiết `base_coder.py` — build prompt (`ChatChunks`), gọi LLM qua `litellm`, nhận stream, parse edit-format, `apply_edits()`, reflection loop khi lỗi
- Architect/Editor two-model pattern: cách 2 `Coder` instance phối hợp

### Issue 3: Đào sâu tool loop của Hermes Agent
- Bổ sung `DISCOVERY-hermes-agent.md`: chi tiết `agent/conversation_loop.py` — vòng lặp turn, gọi LLM qua `agent/transports/*.py` adapter, tool-call parsing, `agent/context_compressor.py`, background review fork
- Learning loop: cách skill/memory được tạo và ghi lại từ kinh nghiệm hội thoại

### Issue 4: Cơ chế spawn CLI đa tài khoản (multi-account)
- Bổ sung `DISCOVERY-multica.md` và/hoặc `DISCOVERY-ai-sdlc.md`: xác nhận (hoặc bác bỏ bằng code) khả năng chạy nhiều CLI agent con với credential khác nhau từ 1 process cha — cơ chế env var override, config path isolation, hay không hỗ trợ

## Capabilities

### New Capabilities
- Bảng so sánh 4 mô hình tích hợp LLM trong master README
- Nội dung deep-dive tool-loop cho Aider và Hermes Agent (đủ chi tiết để mapping trực tiếp vào `toolloop.go`)
- Kết luận xác nhận/bác bỏ về khả năng multi-account CLI spawn

### Modified Capabilities
- `DISCOVERY-aider.md`, `DISCOVERY-hermes-agent.md`, `DISCOVERY-multica.md`/`DISCOVERY-ai-sdlc.md` — mở rộng section hiện có, không viết lại từ đầu

### Removed Capabilities
- Không có

## Impact

| Area | Files Affected |
|------|----------------|
| Master Index | `docs/references/README.md` |
| Agent Platform Reports | `docs/references/agent-platform/DISCOVERY-aider.md`, `DISCOVERY-hermes-agent.md`, `DISCOVERY-multica.md`, `DISCOVERY-ai-sdlc.md` |

## Open Questions
- Không có — scope xác nhận qua yêu cầu trực tiếp của user.
