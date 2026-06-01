# 9router Agent Connection Report

## Mục tiêu tham khảo

Tài liệu này tóm tắt cách 9router kết nối một AI agent/client vào nhiều provider LLM mà vẫn giữ runtime ổn định. Điểm quan trọng không phải là agent gọi thẳng OpenAI/Anthropic/Gemini, mà là agent gọi vào một local gateway có contract cố định. Gateway chịu trách nhiệm resolve model, dịch format, chọn account, fallback và theo dõi lỗi.

## Nguyên lý chính

9router đặt một proxy/gateway ở giữa agent và provider:

```text
Agent / IDE / CLI
  -> Local gateway endpoint
  -> API key validation
  -> Model route resolution
  -> Format translation
  -> Account/provider selection
  -> Provider execution
  -> Fallback on quota/error
  -> Normalized response
```

Agent chỉ cần biết:

- `baseUrl`: URL local gateway, ví dụ `http://localhost:20128/v1`.
- `apiKey`: key do gateway cấp.
- `model`: route hoặc combo name, không nhất thiết là raw vendor model.

## Các kiểu connect một agent

### 1. OpenAI-Compatible Connection

Phù hợp với Cursor, Cline, Continue, RooCode, custom agent hoặc bất kỳ client nào hỗ trợ OpenAI-compatible endpoint.

```text
Base URL: http://localhost:20128/v1
API Key: <9router-api-key>
Model: <provider-prefix/model> hoặc <combo-name>
Endpoint: POST /v1/chat/completions
```

Đây là kiểu kết nối nên dùng làm default cho Auto Code OS vì phổ biến, đơn giản và dễ map sang gateway abstraction.

### 2. Claude-Compatible Connection

Phù hợp với Claude Code hoặc client dùng Anthropic Messages API.

```text
Base URL: http://localhost:20128/v1
API Key: <9router-api-key>
Endpoint: POST /v1/messages
```

Gateway phát hiện endpoint `/v1/messages` là Claude format, sau đó dịch sang provider target format nếu upstream không phải Claude-native.

### 3. OpenAI Responses / Codex Connection

Codex CLI có thể được cấu hình để dùng provider `9router` với wire API dạng Responses.

Ý tưởng config:

```toml
model_provider = "9router"
model = "<model-route>"

[model_providers.9router]
name = "9Router"
base_url = "http://localhost:20128/v1"
wire_api = "responses"
```

API key được lưu riêng theo cơ chế auth của Codex. Pattern này đáng tham khảo nếu Auto Code OS cần hỗ trợ agent runtime kiểu Codex.

### 4. Tool Auto-Config

9router không chỉ document thủ công; nó có API route để ghi config trực tiếp cho từng tool:

- Claude settings.
- Codex settings.
- Cline settings.
- OpenClaw settings.
- Droid/OpenCode/Hermes/Kilo/JCode settings.

Pattern đáng học: thay vì bắt user copy/paste config, UI có thể có nút `Apply` để ghi đúng file config của tool/agent runtime.

### 5. Model Prefix Connection

9router dùng model string dạng:

```text
<provider-prefix>/<model-id>
```

Ví dụ:

```text
cc/claude-opus-4-7
cx/gpt-5.2-codex
cu/claude-4.5-sonnet-thinking
kr/claude-sonnet-4.5
glm/glm-5.1
minimax/MiniMax-M2.7
```

Gateway parse prefix để biết provider thật. Agent không cần biết credential hoặc API shape của provider đó.

### 6. Combo Connection

Agent có thể chọn một combo name thay vì một model cụ thể:

```text
model = "premium-coding"
```

Gateway load combo thành danh sách model/provider theo thứ tự. Nếu model đầu lỗi quota, rate limit, auth hoặc transient error, gateway thử model kế tiếp.

Đây là pattern tốt nhất nếu muốn agent “không chọn model cụ thể” nhưng vẫn chạy ổn định.

## Luồng xử lý runtime

### Bước 1: Nhận request

Client gọi endpoint local như `/v1/chat/completions`, `/v1/messages`, hoặc `/v1/responses`.

Gateway đọc:

- headers auth.
- endpoint path.
- request body.
- `body.model`.

### Bước 2: Validate API key

Nếu bật `requireApiKey`, gateway bắt buộc request phải có API key hợp lệ qua `Authorization: Bearer ...` hoặc `x-api-key`.

Với local loopback có thể được relax hơn, nhưng khi expose qua network thì API key là bắt buộc.

### Bước 3: Resolve model route

`body.model` có thể là:

- `provider/model`.
- alias.
- combo name.
- custom provider node prefix.

Nếu là combo, gateway không resolve thành một model duy nhất ngay mà chạy fallback chain.

### Bước 4: Chọn account/credential

Gateway chọn credential còn active cho provider:

- lọc account bị exclude do retry trước đó.
- lọc account/model đang bị lock do rate limit/quota.
- chọn theo strategy như fill-first hoặc round-robin.
- hỗ trợ preferred connection nếu có.

Nếu provider là free/no-auth provider, gateway có thể inject virtual credential `Public`.

### Bước 5: Refresh token nếu cần

Với OAuth providers như Claude Code, Codex, Kiro, Cursor, Gemini CLI, gateway refresh token trước hoặc sau lỗi `401/403` tùy provider.

Pattern quan trọng: agent không tự refresh token; gateway làm việc đó.

### Bước 6: Detect và translate format

Gateway detect source format từ endpoint và body:

- OpenAI Chat.
- OpenAI Responses.
- Claude Messages.
- Gemini.
- Cursor.
- Kiro.
- Antigravity.

Sau đó dịch sang target format của provider. Nếu client và provider cùng ecosystem, gateway có thể passthrough để tránh mất fidelity.

### Bước 7: Token saver trước dispatch

RTK Token Saver chạy trước khi gửi upstream:

- phát hiện `tool_result`.
- nhận diện output dài như `git diff`, `grep`, `find`, `ls`, `tree`, build logs.
- nén nội dung có cấu trúc.
- không nén error traces.
- không trả output rỗng hoặc output dài hơn input.

Pattern này phù hợp với agent coding vì tool output thường chiếm nhiều input token.

### Bước 8: Execute provider

Gateway lấy executor theo provider và gọi upstream. Executor chịu trách nhiệm:

- build URL.
- build headers auth đúng provider.
- xử lý streaming/non-streaming.
- normalize response.

### Bước 9: Fallback

Nếu upstream trả lỗi:

- Gateway parse error.
- Lock account/model theo cooldown.
- Exclude account hiện tại.
- Thử account khác cùng provider.
- Nếu hết account và request là combo, thử model tiếp theo.
- Nếu hết tất cả, trả lỗi normalized.

## Bài học áp dụng cho Auto Code OS Agents

### Không nên cho nhập raw model tự do

Raw text dễ lỗi:

- typo model name.
- provider không khớp model.
- model hết quota nhưng agent không biết fallback.
- UI lưu model cũ khi đổi provider.

Nên dùng dropdown hoặc route abstraction.

### Agent nên trỏ gateway/tier thay vì vendor model

Config nên ưu tiên:

```json
{
  "provider": "gateway",
  "model_route": "auto"
}
```

Hoặc:

```json
{
  "provider": "gateway",
  "model_route": "balanced"
}
```

Backend/gateway resolve:

```text
easy   -> fast
medium -> balanced
hard   -> powerful
```

### Khi cần fallback mạnh, dùng combo

Ví dụ:

```json
{
  "provider": "gateway",
  "model_route": "coding-default"
}
```

`coding-default` có thể map sang:

```text
1. subscription model
2. cheap API model
3. free model
```

Agent chỉ biết một route ổn định, gateway chịu trách nhiệm fallback.

### Backend phải validate

Không chỉ validate ở UI. API tạo agent nên reject:

- provider không nằm trong allowlist.
- role không hợp lệ.
- level không hợp lệ.
- model route không hợp provider.
- custom model nếu không được bật explicit advanced mode.

### Nên tách `model_route` khỏi `model`

Hiện tại nhiều hệ thống lưu `model` như raw string. Để tránh nhầm, nên tách:

```json
{
  "provider": "gateway",
  "model_route": "balanced",
  "resolved_model": null
}
```

`resolved_model` chỉ dùng telemetry sau khi request chạy xong.

## Đề xuất UI Add Agent

### Provider

Default:

```text
gateway
```

Direct provider chỉ nên là advanced mode:

```text
openai
anthropic
gemini
```

### Model Mode

```text
Auto by level
Tier
Combo
Specific model
```

### Model Route

Nếu `Auto by level`:

```text
easy   -> fast
medium -> balanced
hard   -> powerful
```

Nếu `Tier`:

```text
fast
balanced
powerful
```

Nếu `Combo`:

```text
coding-default
premium-coding
cheap-fallback
free-emergency
```

Nếu `Specific model`, chọn từ dropdown đã load từ provider/model registry.

## Đề xuất backend flow cho Auto Code OS

```text
Task execution
  -> select agent
  -> build route options from task complexity
  -> if agent.provider == gateway:
       resolve model_route/tier/combo
     else:
       validate direct provider + model
  -> call LLM gateway abstraction
  -> record actual provider/model in usage telemetry
```

## Kết luận

9router connect agent ổn bằng cách giữ agent config đơn giản và ổn định:

- agent gọi local gateway.
- agent chọn route/tier/combo, không cần raw provider model.
- gateway xử lý credential, format translation, RTK token saving, account fallback và model fallback.

Với Auto Code OS, hướng an toàn là để Agents Page tạo agent mặc định theo `gateway + auto/tier`, còn direct provider/model chỉ là advanced mode có validation chặt.
