# Design: Follow-up Deep Dive — Mô Hình Tích Hợp LLM & Tool Loop

## Phương Pháp

Đọc trực tiếp source code trong `docs/references/agent-platform/agent-platform/{aider,hermes-agent,multica,ai-sdlc}` (hoặc đường dẫn clone tương ứng dưới `references/`), trích dẫn file:line cụ thể — không suy đoán từ README/docs của project.

## Cấu Trúc Bổ Sung Vào Mỗi Report

### `DISCOVERY-aider.md` — thêm subsection dưới "Luồng Chính (Main Flow)"
- `send_message()` → `format_messages()` (ChatChunks: system, examples, repo-map, files, history, cur-message)
- `litellm.completion(stream=True)` → xử lý streaming chunk
- `get_edits()` theo edit-format (`diff`, `whole`, `udiff`...) → `apply_edits()`
- Reflection: nếu lint/test fail sau apply, tự động gửi lại lỗi cho LLM (giới hạn số vòng lặp)
- Architect/Editor: `ArchitectCoder` gọi model mạnh để plan, sau đó tạo `Coder` mới với `editor_model` để thực thi

### `DISCOVERY-hermes-agent.md` — thêm subsection dưới "Luồng Chính (Main Flow)"
- `conversation_loop.py`: vòng lặp nhận input → build context (system + memory + skill registry + history) → gọi transport
- `agent/transports/*.py`: adapter pattern per-provider, cùng interface `base.py`
- Tool-call parsing → thực thi qua `tools/environments/*` → kết quả trả lại vào context
- `context_compressor.py`: nén context khi vượt ngưỡng token
- Background review fork: `spawn_background_review()` — daemon thread riêng, replay snapshot, quyết định lưu skill/memory, không đụng conversation chính

### `DISCOVERY-multica.md` / `DISCOVERY-ai-sdlc.md` — thêm subsection "Multi-Account CLI Spawn"
- Kiểm tra: mỗi CLI subprocess được spawn với `env` riêng (credential/API-key override) hay dùng chung `os.Environ()` của daemon?
- Kiểm tra: có cơ chế nào set `HOME`, config-dir, hay token path riêng per-spawn để tách session/account không?
- Nếu không tìm thấy bằng chứng code — ghi rõ "không hỗ trợ multi-account, mỗi daemon = 1 account" thay vì suy đoán.

## Không Cần Tạo Component Mới
Đây là công việc research/documentation thuần túy — không có code implementation, không cần data model hay API design.
