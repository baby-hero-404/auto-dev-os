# Specs: Cross-Harness Review

## Added Requirements

### REQ-001: Policy resolution
> ❌ Status: Not Started

**Scenario:**
- WHEN policy = `different_provider` và code step dùng provider A, tồn tại provider B cấu hình sẵn
- THEN review step gọi provider B
- AND WHEN chỉ có 1 provider → dùng model khác cùng provider + log warning
- AND WHEN cũng chỉ có 1 model → chạy same + warning, pipeline không bị chặn

### REQ-001b: Underlying-model awareness cho CLI
> ❌ Status: Not Started

**Scenario:**
- WHEN task coded bởi CLI có khai báo `underlying_provider` (vd cli:claude → anthropic) trong `cli_engine_config`
- THEN resolver coi underlying provider là provider đã code (KHÔNG coi cli là "khác mọi thứ") — ưu tiên chọn review provider khác underlying (vd OpenAI/Google) nếu có key
- AND `underlying_provider` không khai báo → giữ hành vi cũ (cli ≠ mọi API provider) + log note khuyến nghị khai báo

### REQ-001c: Adversarial directive khi buộc same-provider
> ❌ Status: Not Started

**Scenario:**
- WHEN degrade chain kết thúc ở same provider/model với bên đã code (kể cả qua underlying_provider)
- THEN system prompt của review step được inject directive adversarial: review như audit code của AI khác, không giả định logic đúng, tập trung danh sách lỗi hệ thống hay gặp (off-by-one, error-swallow, unchecked nil, injection, race)
- AND directive KHÔNG được inject khi provider đã thực sự khác (tránh nhiễu)

### REQ-002: Metadata coded_by/reviewed_by
> ❌ Status: Not Started

**Scenario:**
- WHEN code step và review step hoàn thành
- THEN step state chứa `coded_by: {engine, provider, model}` và `reviewed_by: {provider, model}`
- AND PR description chứa 2 dòng metadata này

### REQ-003: CLI-mode cross_review step
> ❌ Status: Not Started

**Scenario:**
- WHEN task engine=cli và policy ≠ `same`
- THEN sau cli_implement, step `cross_review` chạy: API-native model review git diff + spec set với 2-verdict schema
- AND fail (spec hoặc quality) → re-dispatch cli_implement kèm violations, đếm vào MaxReviewFixCycles
- AND policy = `same` → cli_spec_first không có node cross_review (như Wave 1)

### REQ-004: Policy setting UI
> ❌ Status: Not Started

**Scenario:**
- WHEN user mở project settings
- THEN có select Review Harness (Same / Different model / Different provider) với mô tả ngắn từng option

## Modified Requirements

### REQ-M01: Review step model override
> ❌ Status: Not Started

**Scenario:**
- WHEN review step chạy với policy `same`
- THEN hành vi và model selection y hệt trước feature (zero regression)

## Removed Requirements
- Không có.
