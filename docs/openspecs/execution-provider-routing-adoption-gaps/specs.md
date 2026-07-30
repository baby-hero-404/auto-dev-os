# Specs: Execution Provider Routing — Adoption Gaps

## Modified Requirements

### REQ-001: `cliStepRunner` (spec-first flow) đi qua `ResolveExecutionProvider`
> ✅ Status: Fully Implemented

**Scenario: Project chỉ cấu hình CLI qua `ExecutionProviders`**
- WHEN 1 task chạy `cli_analyze`/`cli_spec`/`cli_implement`, project có `ExecutionProviders=[{type:cli, ref:claude_code, priority:0, enabled:true}]` và `CLIEngineConfig` rỗng (`{}`)
- THEN `cliStepRunner.resolveConfig` trả về `CLIEngineConfig` build từ `CLIProfiles["claude_code"]` (đúng `Command`/`Args`/`AuthCheckCommand`) — không phải config rỗng

**Scenario: Quota exceeded giữa flow spec-first**
- WHEN `cli_spec` step chạy xong, output khớp `CLIQuotaRules` (quota/rate-limit)
- THEN credential đã dùng cho step đó được `SetCooldown` — giống hệt hành vi `cliEngineRunner.RunLLMStep` đã có cho `code_backend`/`code_frontend`/`fix`

**Scenario: Project cũ (chưa có `ExecutionProviders`) không đổi hành vi**
- WHEN project chỉ có `ExecutionEngine="cli"` + `CLIEngineConfig` cũ, `ExecutionProviders` rỗng
- THEN `cliStepRunner.resolveConfig` trả về config y hệt hành vi hôm nay (qua `legacyResolveExecutionProvider`, không phải re-implement) — REQ-003 của OpenSpec gốc vẫn đúng

### REQ-002: `worker.go` chọn `CLISpecFirstWorkflow` nhất quán với Router
> ✅ Status: Fully Implemented

**Scenario: Project chỉ cấu hình CLI qua `ExecutionProviders`**
- WHEN task thuộc project có `ExecutionProviders` chứa 1 candidate `type:cli, enabled:true` khả dụng (credential active), `ExecutionEngine` vẫn là default `"api_native"`
- THEN `worker.go` chọn `workflow.CLISpecFirstWorkflow` — không rơi vào `DynamicDAGWorkflow`/DAG mặc định như hôm nay

**Scenario: Không còn candidate CLI khả dụng nào (tất cả rate-limited/disabled)**
- WHEN `ExecutionProviders` chỉ toàn candidate CLI nhưng không cái nào khả dụng (rate-limited hết, hoặc `enabled:false` hết)
- THEN `worker.go` **không** chọn `CLISpecFirstWorkflow` (không có gì để "spec-first" khi không CLI nào chạy được) — rơi về DAG mặc định như project chưa từng bật CLI, `resolveCLIEngineRunner` bên trong DAG đó sau này tự trả lỗi rõ ràng khi thật sự cần chạy CLI

**Scenario: Project cũ (chưa có `ExecutionProviders`) không đổi hành vi**
- WHEN `ExecutionProviders` rỗng, `ExecutionEngine="cli"`
- THEN `worker.go` vẫn chọn `CLISpecFirstWorkflow` y hệt hôm nay (byte-identical qua `ResolveEngine`, không qua Router)

### REQ-003: `TaskService.validateTaskEngineOverride` nhận diện `ExecutionProviders`
> ✅ Status: Fully Implemented

**Scenario: Project chỉ cấu hình CLI qua `ExecutionProviders`, task override `execution_engine="cli"`**
- WHEN `POST /projects/:id/tasks` hoặc `PATCH /tasks/:id` với `execution_engine="cli"`, project có `ExecutionProviders` chứa ít nhất 1 entry `type:"cli", enabled:true` (không cần check credential khả dụng ngay lúc này — đó là việc của Router lúc Task chạy)
- THEN request được chấp nhận (201/200), không trả 400 `"project has no cli_engine_config configured"`

**Scenario: Project không có CLI nào bật ở cả 2 nơi**
- WHEN `execution_engine="cli"` được set, `ExecutionProviders` rỗng **và** `CLIEngineConfig.Command` rỗng
- THEN vẫn trả 400 như hôm nay (không nới lỏng validation cho trường hợp thật sự chưa cấu hình gì)

**Scenario: Project cũ dùng `CLIEngineConfig`**
- WHEN `ExecutionProviders` rỗng, `CLIEngineConfig.Command` đã set
- THEN hành vi validate y hệt hôm nay (fallback không đổi)

## Removed Requirements
- Không có.
