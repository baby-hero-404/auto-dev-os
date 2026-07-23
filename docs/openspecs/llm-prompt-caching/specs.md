# Specs: Anthropic Prompt Caching

## Added Requirements

### REQ-001: cache_control trên system + tools
> ✅ Status: Done — `pkg/llm/anthropic.go:87-104`

**Scenario:**
- WHEN một request Anthropic được build qua `ChatWithOptions` có system prompt và tools
- THEN block cuối của system và block cuối của tools mang `cache_control: {type: "ephemeral"}`
- AND tổng số cache breakpoints ≤ 4 (giới hạn API)

### REQ-002: Prefix ổn định
> ✅ Status: Done (audit only, no automated test) — `internal/prompts/builder.go`: only stable sections (Base/Role/Step Prompt, Global/Project Rules, Tools) are `Destination: "system"`; RepoMap/diffs/memories/task requirement are `"user"`. System metadata (`assembler.go:194-211`) is per-job-stable (project_id/task_id/role/task_rules), no per-turn dynamic content. Missing: 1.5 unit test asserting byte-identical prefix across 2 calls.

**Scenario:**
- WHEN cùng một step gọi LLM 2 lần liên tiếp trong 1 tool-loop
- THEN phần system + tools bytes giống hệt nhau giữa 2 lần (không timestamp/nonce động trong prefix)
- AND RepoMap, file diffs, task instruction nằm trong messages (sau breakpoint cuối) — KHÔNG trong bất kỳ cached block nào (repomap đổi sau mỗi checkpoint sẽ vô hiệu cache)

### REQ-003: Cache metrics
> ✅ Status: Done (code), integration assertion not automated — `anthropic.go:165-179,241-242` parses/logs `cache_read_input_tokens`/`cache_creation_input_tokens`. Missing: 1.6 integration test asserting cache_read > 0 on 2nd tool-loop call.

**Scenario:**
- WHEN response chứa `cache_read_input_tokens > 0`
- THEN giá trị được ghi vào usage log cùng chỗ với input/output tokens hiện tại
- AND vòng gọi thứ 2 trở đi của một tool-loop cho thấy cache_read > 0 (integration assertion)

### REQ-004: Provider khác không bị ảnh hưởng
> ✅ Status: Done — `cache_control` field only exists in `pkg/llm/anthropic.go`, no other provider file references it, so it structurally never appears in non-Anthropic request bodies.

**Scenario:**
- WHEN request đi tới provider không phải Anthropic
- THEN request body không chứa field `cache_control`

## Modified Requirements
- Không có.

## Removed Requirements
- Không có.
