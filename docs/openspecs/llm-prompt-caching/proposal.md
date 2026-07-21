# Proposal: Anthropic Prompt Caching (P0.1)

## Why

`grep -rn "cache_control" server/pkg/llm` trả về rỗng (verified 2026-07-20) — Auto Code OS gọi Anthropic API mà không dùng prompt caching, trong khi system prompt + tool definitions lặp lại y hệt qua mọi vòng tool-loop. Đây là tiền đề cho các tối ưu token khác (headroom cache-drift, claw-compactor prefix stabilization đều giả định caching đã có). Chi phí thấp nhất, lợi ngay: cached input rẻ hơn ~10x.

## What Changes

### Issue 1: cache_control trên prefix ổn định

- Thêm `cache_control: {type: "ephemeral"}` vào block cuối của system prompt và block cuối của tools array trong `server/pkg/llm/anthropic.go` (`ChatWithOptions`, ~line 51).
- Đảm bảo thứ tự assembly prompt ổn định (system → tools → messages) — không chèn nội dung động (timestamp, task-id) vào phần được cache.

### Issue 2: Cache metrics

- Đọc `cache_creation_input_tokens` / `cache_read_input_tokens` từ response usage, log vào token usage tracking hiện có (chuẩn bị số liệu cho Smart Router P3.3).

## Capabilities

### New Capabilities
- Prompt caching cho mọi Anthropic call qua `pkg/llm`.
- Cache hit/miss metrics trong logs.

### Modified Capabilities
- `ChatWithOptions` request construction.

### Removed Capabilities
- Không có.

## Impact

| Area | Files Affected |
|------|----------------|
| LLM client | `server/pkg/llm/anthropic.go` |
| Usage logging | nơi đang ghi usage tokens (cùng file hoặc caller) |
