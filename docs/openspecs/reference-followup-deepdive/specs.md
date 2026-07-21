# Specs: Follow-up Deep Dive — Mô Hình Tích Hợp LLM & Tool Loop

## Added Requirements

### REQ-001: Bảng so sánh 4 mô hình tích hợp LLM trong README
> ✅ Status: Done

**Scenario:**
- WHEN mở `docs/references/README.md`
- THEN thấy section liệt kê 4 mô hình: Subprocess-CLI, SDK-embedded, API-native, Hybrid
- AND mỗi mô hình map với project cụ thể + trade-off
- AND có kết luận Auto Code OS đang ở mô hình nào

### REQ-002: Deep dive tool loop của Aider
> ⚠️ Status: In Progress

**Scenario:**
- WHEN đọc `DISCOVERY-aider.md`
- THEN thấy mô tả chi tiết chuỗi build-prompt → gọi LLM → parse edit → apply → reflection
- AND có reference tới file/dòng code cụ thể trong `base_coder.py`

### REQ-003: Deep dive tool loop của Hermes Agent
> ⚠️ Status: In Progress

**Scenario:**
- WHEN đọc `DISCOVERY-hermes-agent.md`
- THEN thấy mô tả chi tiết vòng lặp turn trong `conversation_loop.py`, transport adapter, tool-call parsing
- AND có mô tả learning loop (tạo skill/memory từ kinh nghiệm)

### REQ-004: Xác nhận cơ chế multi-account CLI spawn
> ❌ Status: Not Started

**Scenario:**
- WHEN đọc report Multica hoặc ai-sdlc phần multi-account
- THEN thấy kết luận rõ ràng: có hỗ trợ hay không, và cơ chế cụ thể (env var, config path, working-dir isolation)
- AND kết luận trích dẫn code thật (file:line), không suy đoán

## Modified Requirements
- Không có

## Removed Requirements
- Không có
