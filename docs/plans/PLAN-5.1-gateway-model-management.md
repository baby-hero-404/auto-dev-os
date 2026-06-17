# PLAN 5.1: Gateway Model Management & Routing

**Status:** Proposed  
**Feature Owner:** `docs/features/5.1-unified-ai-gateway.md`  

## 1. Ngữ Cảnh & Mục Tiêu (Context & Objective)
Hiện tại, AI Gateway đang sử dụng cấu hình tĩnh (biến môi trường như `LLM_FAST_MODEL`, `LLM_POWERFUL_MODEL` trong `config.Config`) hoặc bảng `model_routes` cũ để quyết định model nào được sử dụng khi Agent gọi `fast`, `balanced`, hay `powerful`.
Theo bản cập nhật kiến trúc 5.1 mới nhất:
- Hệ thống cần cho phép tạo tự động một danh sách các model chia theo các Model Level Group (Fast/Balanced/Powerful) khi Admin thêm mới một Provider.
- Admin có quyền (thông qua UI/API) thêm, sửa, xóa, vô hiệu hóa, và xếp thứ tự ưu tiên các model trong từng Level Group của từng Provider.
- Gateway tại thời điểm runtime sẽ tự động tra cứu danh sách model được cấu hình trong CSDL (`provider_models`) thay vì fallback vào code cứng hoặc bảng `model_routes` cũ.
- **Bãi bỏ hoàn toàn hệ thống `model_routes` cũ** bao gồm code service, handler, repository và migration table để tránh trùng lặp dữ liệu và logic điều phối.


## 2. Thiết kế Cơ Sở Dữ Liệu (Database Schema)

Tạo file migration `000014_provider_models.up.sql`:

```sql
CREATE TABLE IF NOT EXISTS provider_models (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL, -- e.g., 'openai', 'anthropic', 'gemini'
    level_group VARCHAR(50) NOT NULL, -- 'fast', 'balanced', 'powerful'
    model_name VARCHAR(100) NOT NULL, -- e.g., 'gpt-4o-mini', 'claude-sonnet-4'
    priority INT NOT NULL DEFAULT 0, -- Thứ tự ưu tiên fallback (càng nhỏ ưu tiên càng cao)
    is_active BOOLEAN NOT NULL DEFAULT TRUE, -- Cho phép Admin tạm tắt model bị lỗi
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, provider, level_group, model_name)
);

CREATE INDEX IF NOT EXISTS idx_provider_models_org_level 
    ON provider_models(org_id, level_group, is_active, priority ASC);
```


Tạo file migration `000014_provider_models.down.sql`:

```sql
DROP INDEX IF EXISTS idx_provider_models_org_level;
DROP TABLE IF EXISTS provider_models;
```


## 3. GORM Models (`server/pkg/models/provider_model.go`)

```go
package models

import "time"

const (
	ModelLevelFast     = "fast"
	ModelLevelBalanced = "balanced"
	ModelLevelPowerful = "powerful"
)

type ProviderModel struct {
	ID         string    `json:"id" gorm:"type:uuid;default:uuid_generate_v4();primaryKey"`
	OrgID      string    `json:"org_id" gorm:"type:uuid;not null"`
	Provider   string    `json:"provider" gorm:"not null"`
	LevelGroup string    `json:"level_group" gorm:"not null"`
	ModelName  string    `json:"model_name" gorm:"not null"`
	Priority   int       `json:"priority" gorm:"default:0;not null"`
	IsActive   bool      `json:"is_active" gorm:"default:true;not null"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CreateProviderModelInput struct {
	Provider   string `json:"provider"`
	LevelGroup string `json:"level_group"`
	ModelName  string `json:"model_name"`
	Priority   int    `json:"priority"`
	IsActive   *bool  `json:"is_active"` // Dùng pointer để tránh Go zero value override DB default TRUE. Nếu nil, service mặc định set thành true.
}


type UpdateProviderModelInput struct {
	Provider   *string `json:"provider,omitempty"`
	LevelGroup *string `json:"level_group,omitempty"`
	ModelName  *string `json:"model_name,omitempty"`
	Priority   *int    `json:"priority,omitempty"`
	IsActive   *bool   `json:"is_active,omitempty"`
}

type ProviderModelFilter struct {
	Provider   *string `json:"provider"`
	LevelGroup *string `json:"level_group"`
}
```

## 3.5. Định nghĩa interface `ProviderModelService` và Kiến Trúc Tách Lớp (Layering & Interfaces)

Để tránh hiện tượng vòng lặp phụ thuộc (Circular Dependency) do package `handler` không được import bởi `service` hay `gateway`, ta áp dụng phương pháp khai báo interface tại nơi tiêu thụ (Consumer-defined Interfaces) rất phổ biến trong Go:

1. **Tại `server/internal/handler/services.go`**: Khai báo interface đầy đủ cho Handler layer:
```go
type ProviderModelService interface {
	Create(ctx context.Context, orgID string, input models.CreateProviderModelInput) (*models.ProviderModel, error)
	ListByOrg(ctx context.Context, orgID string, filter models.ProviderModelFilter) ([]models.ProviderModel, error)
	Update(ctx context.Context, id string, input models.UpdateProviderModelInput) (*models.ProviderModel, error)
	Delete(ctx context.Context, id string) error
}
```


2. **Tại package `service` (ví dụ `server/internal/service/credential_pool.go`)**: Định nghĩa interface hẹp (narrow interface) cục bộ phục vụ cho luồng Auto-seeding. Giao diện này chỉ chứa các hàm cần thiết để lọc cấu hình hiện tại và tạo dữ liệu seed:
```go
type providerModelSeeder interface {
	ListByOrg(ctx context.Context, orgID string, filter models.ProviderModelFilter) ([]models.ProviderModel, error)
	Create(ctx context.Context, orgID string, input models.CreateProviderModelInput) (*models.ProviderModel, error)
}
```

3. **Tại package `gateway` (ví dụ `server/internal/gateway/gateway.go`)**: Định nghĩa interface hẹp cục bộ phục vụ cho việc định tuyến model ở runtime:
```go
type ProviderModelResolver interface {
	ResolveModels(ctx context.Context, orgID string, levelGroup string) ([]models.ProviderModel, error)
}

```

*Lưu ý về Validation & Defaulting:* 
- `ProviderModelService` (struct cụ thể trong package `service`) sẽ chịu trách nhiệm validate thủ công các tham số đầu vào (như kiểm tra provider trong whitelist, level_group thuộc fast/balanced/powerful, các trường bắt buộc không được để trống) tương tự cách validate thủ công của AgentService hiện tại.
- **Xử lý IsActive mặc định:** Nếu `input.IsActive` là con trỏ `nil` (không được truyền lên từ frontend), service sẽ chủ động gán `true` cho thuộc tính `IsActive` của model struct trước khi gọi repository để lưu vào DB, tránh việc mapping sai lệch làm vô hiệu hóa cấu hình model một cách vô ý.

## 4. Logic Tự động nạp (Auto-Seeding Logic)

Tại `server/internal/service/credential_pool.go` (trong phương thức hoặc hàm tạo credential/thêm vào pool): Khi Admin gọi API `CreateProviderCredential` thành công (sau khi `s.repo.Create(...)` thực thi thành công không lỗi), hệ thống sẽ kiểm tra xem đã tồn tại cấu hình model (`provider_models`) nào cho `provider` này trong `org_id` chưa (thông qua hàm `ListByOrg` với filter provider tương ứng). Việc thực hiện kiểm tra sau khi lưu DB thành công giúp đảm bảo không phát sinh seeding thừa khi xảy ra lỗi validation key hoặc trùng lặp credential key.

*Dependency Wiring:* 
- Thêm trường `seeder providerModelSeeder` vào struct `CredentialPoolService` tại `server/internal/service/credential_pool.go`.
- Bổ sung hàm thiết lập builder:
  ```go
  func (s *CredentialPoolService) WithProviderModelSeeder(seeder providerModelSeeder) *CredentialPoolService {
      s.seeder = seeder
      return s
  }
  ```
- Tại `server/cmd/api/main.go`, thực hiện tiêm `providerModelSvc` (đóng vai trò là concrete implementation thỏa mãn interface `providerModelSeeder`) vào `credentialPoolSvc` khi khởi tạo.




**Mapping mặc định (Seed Data):**
- **OpenAI**: 
  - Fast: `gpt-4o-mini` (priority 0)
  - Balanced: `gpt-4o` (priority 0)
  - Powerful: `gpt-4o` (priority 0)
  - Powerful (fallback): `o1-preview` (priority 1)
- **Anthropic**:
  - Balanced: `claude-3-5-sonnet-20241022` (priority 0)
  - Powerful: `claude-3-5-sonnet-20241022` (priority 0), `claude-3-opus-20240229` (priority 1)
- **Gemini**:
  - Fast: `gemini-1.5-flash` (priority 0)
  - Balanced: `gemini-1.5-pro` (priority 0)
  - Powerful: `gemini-1.5-pro` (priority 0)

*Hành vi & Concurrency:* Insert theo batch vào bảng `provider_models` nếu `org_id` đó chưa tồn tại cấu hình model nào cho `provider` này. Để tránh xung đột dữ liệu khi hai luồng tạo credential chạy đồng thời (race condition), câu lệnh SQL chèn dữ liệu cần sử dụng cú pháp `ON CONFLICT (org_id, provider, level_group, model_name) DO NOTHING`, hoặc service layer phải bắt lỗi unique constraint violation và bỏ qua một cách an toàn (gracefully ignore) để không làm gián đoạn luồng đăng ký key của Admin.

## 5. Gateway Runtime Execution (`server/internal/gateway/gateway.go`)

Gateway hiện đang gọi `defaultEntries(cfg, opts.Complexity)`. Cần refactor lại:

1. **Thay đổi dependency**: Thêm interface hẹp cục bộ `ProviderModelResolver` vào struct `AIGateway` (thay thế cho `ModelRouteService` / `routeService` cũ) để giải phóng Gateway khỏi phụ thuộc trực tiếp vào các layer khác.
2. **Cập nhật `routeEntries`**:
   - Nhận vào `LevelGroup` (từ `opts.RouteName` hoặc tính toán từ `opts.Complexity`).
   - Gọi `ProviderModelResolver.ResolveModels(ctx, opts.OrgID, LevelGroup)` để lấy danh sách active models được sắp xếp theo `priority` tăng dần.
   - Map các model trả về thành mảng `models.ComboEntry{Provider, Model, Priority, Tier}`.
   - Lưu ý: KHÔNG thực hiện tiền lọc (pre-filter) các model bằng cách gọi `SelectCredential` để kiểm tra API key hợp lệ ở bước này, vì hàm đó sẽ sinh ra các sự kiện audit log (credential used) giả mạo. Cứ trả về toàn bộ ComboEntry lấy được và để luồng credential selection ở runtime tự nhiên xử lý lỗi fail-over sang model tiếp theo nếu không có key hợp lệ.


3. **Cơ chế Fallback an toàn**:
   - Nếu truy vấn DB trả về empty list (ví dụ Org chưa thiết lập hoặc lỗi DB), gateway TỰ ĐỘNG fallback gọi hàm `defaultEntries()` cũ (dựa trên ENV variables) để đảm bảo không đứt gãy luồng hoạt động của app.


## 6. REST API Endpoints (Admin UI Integration)

Tạo file `server/internal/handler/provider_model.go` và định nghĩa handler `ProviderModelHandler` sử dụng interface `ProviderModelService`.

**Cấu hình router tại `server/internal/handler/router.go`:**
Thay thế nhóm route `/model-routes` cũ bằng:

```go
r.Route("/provider-models", func(r chi.Router) {
    r.Get("/", providerModelH.List) // Cho phép Member/Admin xem danh sách cấu hình
    r.Group(func(r chi.Router) {
        // Chỉ Admin mới có quyền thay đổi cấu hình Model Level
        r.Use(mw.RequireRole(models.UserRoleAdmin))
        r.Post("/", providerModelH.Create)
        r.Put("/{providerModelID}", providerModelH.Update)
        r.Delete("/{providerModelID}", providerModelH.Delete)
    })
})
```

Các endpoint tương ứng:
- **GET** `/api/v1/organizations/{orgID}/provider-models`: Trả về danh sách model của Org, hỗ trợ filter theo `provider` và `level_group`.
  - *Xử lý Query:* Phương thức `ProviderModelHandler.List` sẽ đọc tham số thông qua `r.URL.Query().Get("provider")` và `r.URL.Query().Get("level_group")`.
  - *Chuẩn hóa & Validate:* Chuẩn hóa `provider` về dạng chữ thường (lowercase) bằng `strings.ToLower`. Nếu `level_group` không rỗng, kiểm tra xem có thuộc danh sách `fast`, `balanced`, `powerful` không; nếu không hợp lệ, trả về lỗi HTTP 400 Bad Request.
  - *Gửi Service:* Điền thông tin vào struct `models.ProviderModelFilter` (dưới dạng con trỏ `*string` để phân biệt trường rỗng/không lọc) rồi truyền vào `ProviderModelService.ListByOrg`.
- **POST** `/api/v1/organizations/{orgID}/provider-models`: Tạo cấu hình model mới (nhận `CreateProviderModelInput`).
- **PUT** `/api/v1/organizations/{orgID}/provider-models/{providerModelID}`: Cập nhật cấu hình model (nhận `UpdateProviderModelInput`).
- **DELETE** `/api/v1/organizations/{orgID}/provider-models/{providerModelID}`: Xoá model khỏi danh sách.




## 7. Bãi bỏ & Dọn dẹp Hệ thống `model_routes` cũ

Để tránh dư thừa và chồng chéo logic, toàn bộ các thành phần liên quan đến `model_routes` cần được bãi bỏ:

1. **Cơ sở dữ liệu (Database Migration)**:
   - Tạo file migration `000015_drop_model_routes.up.sql` để thực hiện:
     ```sql
     DROP TABLE IF EXISTS model_routes;
     ```
    - Tạo file migration `000015_drop_model_routes.down.sql` để khôi phục cấu trúc bảng chính xác như trước:
     ```sql
     CREATE TABLE IF NOT EXISTS model_routes (
         id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
         org_id          UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
         name            VARCHAR(50) NOT NULL,
         route_type      VARCHAR(20) NOT NULL,
         config          JSONB NOT NULL,
         is_default      BOOLEAN NOT NULL DEFAULT FALSE,
         created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
         updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
         UNIQUE(org_id, name)
     );
     ```
2. **Xóa/Bãi bỏ Code Backend**:
   - Xóa file Handler: `server/internal/handler/model_route.go`
   - Xóa file Service: `server/internal/service/model_route.go`
   - Xóa file Repository: `server/internal/repository/model_route.go`
   - Xóa file Model: `server/pkg/models/model_route.go`.
   - **Di chuyển struct `ComboEntry`**: Trước khi xóa, hãy di chuyển định nghĩa struct `ComboEntry` sang file model mới là `server/pkg/models/provider_model.go` để không làm đứt gãy import trong `server/internal/gateway/gateway.go:205` và các file khác.
3. **Dọn dẹp Router & Handlers**:
   - Gỡ bỏ đăng ký router cho các endpoint `/api/v1/model-routes` trong `server/internal/handler/router.go`.
   - Gỡ bỏ `ModelRouteService` khỏi container và thay bằng `ProviderModelService` trong `server/internal/handler/services.go`.
4. **Cập nhật Entrypoint (`server/cmd/api/main.go`)**:
   - Thay thế khởi tạo `modelRouteRepo := repository.NewModelRouteRepo(db)` và `modelRouteSvc := service.NewModelRouteService(modelRouteRepo)` bằng `providerModelRepo` và `providerModelSvc` tương ứng.
   - Refactor lại hàm `buildLLMProvider(...)` để nhận cấu trúc service cụ thể (concrete provider model service) hoặc tốt nhất là tự định nghĩa một interface hẹp nội bộ ngay trong file `main.go` thay cho `ModelRouteService`. Tuyệt đối tránh import trực tiếp interface từ `gateway` package.

   - **Xử lý Virtual Key:** Giữ nguyên toàn bộ legacy virtual key code hiện tại (ví dụ: `virtualKeySvc`, `virtualKeyRepo`, `virtualKeyHandler`) và các tham chiếu của chúng trong `buildLLMProvider` và options, không tiến hành thay đổi cơ chế này do tính năng Virtual Key đã được trì hoãn.
   - Cập nhật struct `aigateway.Options` tại `server/internal/gateway/gateway.go:28`, cụ thể: thay thế trường `RouteService` cũ bằng trường mới có tên rõ ràng là `ProviderModelResolver ProviderModelResolver` (đảm bảo cả field name và interface name đều được Exported - viết hoa chữ cái đầu) để chứa tham chiếu đến struct service.



5. **Cập nhật và Viết mới tệp Test**:
   - Xóa bỏ hoặc cập nhật bất kỳ tệp test nào của model route cũ (nếu có) để tránh lỗi build hệ thống.
   - Bổ sung các unit test tập trung cho `ProviderModelService`: kiểm thử tính đúng đắn của logic validation, cấu hình gán default (`IsActive`), và độ ổn định của query filtering (`ProviderModelHandler.List`).
   - Bổ sung test case cho `CredentialPoolService` để đảm bảo tính Idempotency (chống trùng lặp) của luồng Auto-seeding.





