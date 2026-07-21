# Specs: Anthropic Prompt Caching

## Added Requirements

### REQ-001: cache_control trên system + tools
> ❌ Status: Not Started

**Scenario:**
- WHEN một request Anthropic được build qua `ChatWithOptions` có system prompt và tools
- THEN block cuối của system và block cuối của tools mang `cache_control: {type: "ephemeral"}`
- AND tổng số cache breakpoints ≤ 4 (giới hạn API)

### REQ-002: Prefix ổn định
> ❌ Status: Not Started

**Scenario:**
- WHEN cùng một step gọi LLM 2 lần liên tiếp trong 1 tool-loop
- THEN phần system + tools bytes giống hệt nhau giữa 2 lần (không timestamp/nonce động trong prefix)
- AND RepoMap, file diffs, task instruction nằm trong messages (sau breakpoint cuối) — KHÔNG trong bất kỳ cached block nào (repomap đổi sau mỗi checkpoint sẽ vô hiệu cache)

### REQ-003: Cache metrics
> ❌ Status: Not Started

**Scenario:**
- WHEN response chứa `cache_read_input_tokens > 0`
- THEN giá trị được ghi vào usage log cùng chỗ với input/output tokens hiện tại
- AND vòng gọi thứ 2 trở đi của một tool-loop cho thấy cache_read > 0 (integration assertion)

### REQ-004: Provider khác không bị ảnh hưởng
> ❌ Status: Not Started

**Scenario:**
- WHEN request đi tới provider không phải Anthropic
- THEN request body không chứa field `cache_control`

## Modified Requirements
- Không có.

## Removed Requirements
- Không có.
