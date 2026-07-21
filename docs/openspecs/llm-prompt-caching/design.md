# Design: Anthropic Prompt Caching

## Request shape

```jsonc
{
  "system": [
    {"type": "text", "text": "<static system prompt>", "cache_control": {"type": "ephemeral"}}
  ],
  "tools": [ /* ...all tools..., last one gets: */
    {"name": "...", "input_schema": {...}, "cache_control": {"type": "ephemeral"}}
  ],
  "messages": [ /* dynamic, uncached */ ]
}
```

- `cache_control` trên block **cuối** của mỗi section → cache toàn bộ prefix tới điểm đó. 2 breakpoints (system, tools) là đủ; để dành 2 slot còn lại cho tương lai (vd frozen context block).
- Nếu `anthropic.go` hiện gửi system dạng string đơn, chuyển sang array-of-blocks (API tương thích).

## Cache partition (phần khó nhất — quyết định hit rate)

Bật cache dễ; giữ hit rate > 0 mới khó. Phân loại tường minh theo tần suất thay đổi:

| Khối | Đổi khi nào | Vị trí | Cache? |
|------|-------------|--------|--------|
| System persona + rules | Theo release | system block 1 | ✅ breakpoint 1 |
| Tool definitions | Theo release | tools (block cuối) | ✅ breakpoint 2 |
| Frozen context / architecture summary | 1 lần mỗi job | system block 2 (sau rules) | ✅ breakpoint 3 — **chỉ khi** xác nhận bất biến trong job |
| **RepoMap** | Sau mỗi commit/checkpoint | **messages** (user turn đầu) | ❌ TUYỆT ĐỐI KHÔNG — repomap rebuild sau mỗi checkpoint sẽ invalidate toàn prefix, hit rate = 0 |
| File contents / diffs | Mỗi tool call | messages | ❌ |
| Task description / instruction | Mỗi task | messages | ❌ |

Quy tắc: **thứ gì đổi trong vòng đời 1 job thì không được đứng trước breakpoint cuối**. Audit hiện trạng: grep nơi assemble system prompt (`internal/prompts/`) — nếu repomap/diff/timestamp đang nằm trong system, di chuyển xuống messages trước khi bật cache. `steps/frozen_context.go` được thiết kế bất biến trong job → đủ điều kiện block 3, nhưng phải có test khẳng định (2 lần assemble cùng job → identical bytes).

## Metrics

`Usage` struct (nơi parse response) thêm `CacheCreationInputTokens`, `CacheReadInputTokens`; propagate lên log line usage hiện có. Không cần bảng DB mới — Smart Router (P3.3) sẽ thêm bảng `token_usage` sau, spec này chỉ cần log.

## Trade-offs

- Ephemeral cache TTL 5 phút (mặc định API) — tool-loop iterations cách nhau giây/phút nên hit rate cao; job kéo dài qua đêm sẽ miss lại, chấp nhận.
- Không dùng extended TTL (1h) vì tăng chi phí ghi cache; đánh giá lại sau khi có metrics.
