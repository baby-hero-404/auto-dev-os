# Specs: Smart LLM Router

## Added Requirements

### REQ-001: Usage persistence
> ✅ Status: Done

**Scenario:**
- WHEN một LLM call hoàn thành trong tool-loop
- THEN 1 row `token_usage` được ghi với đủ token fields + cost estimate
- AND ghi fail không làm fail step (log error, best-effort)

### REQ-002: Step routing matrix
> ✅ Status: Done

**Scenario:**
- WHEN step `analyze` chạy cho project DefaultModelLevel=powerful
- THEN model được resolve ở level `fast`
- AND step `code_backend` vẫn resolve `powerful`

### REQ-003: Complexity downgrade
> ✅ Status: Done

**Scenario:**
- WHEN task Complexity=easy
- THEN mỗi step hạ 1 bậc so với matrix (powerful→balanced, balanced→fast, fast giữ nguyên)

### REQ-004: Escape hatch
> ✅ Status: Done

**Scenario:**
- WHEN step fail và được retry bởi patch_retry_loop
- THEN retry attempt ≥2 nâng lại level về matrix gốc (không để model rẻ loop mãi)

### REQ-005: Usage API + UI
> ✅ Status: Done

**Scenario:**
- WHEN GET `/projects/{id}/usage?days=30`
- THEN trả aggregate `{total_cost, by_model[], by_step[], cache_savings}`
- AND project page hiển thị card tương ứng

## Modified Requirements

### REQ-M01: Routing tắt được
> ✅ Status: Done

**Scenario:**
- WHEN project setting `smart_routing=false` (default **true** sau khi ship)
- THEN mọi step dùng DefaultModelLevel như trước feature

## Removed Requirements
- Không có.
