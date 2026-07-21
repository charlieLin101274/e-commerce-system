# Cart Recall E-Commerce MVP

這是一個以 Go、Gin 與 PostgreSQL 實作的電商行銷活動系統，也是七天完成的 Cart Recall vertical slice。

專案重點是展示一條可以實際執行與驗證的流程：Campaign 設定、Rule Engine、購物車事件、延遲評估、Notification Task、retry，以及訂單轉換記錄。

完整 PRD 請見 [docs/prd](docs/prd/README.md)。營運流程、KPI 與 guardrails 請見 [Growth and Operations](docs/prd/06-growth-and-operations.md)。

## MVP Scope

七天 MVP 已完成：

- Member 註冊、登入與 JWT Authentication。
- Product、Cart 與 Order 基礎購買流程。
- Campaign lifecycle、商品 scope 與 deterministic ranking。
- Rule validation、dry-run、versioning 與 server-side eligibility evaluation。
- PostgreSQL Transactional Outbox、Inbox deduplication 與 persistent Cart Recall Journey。
- Notification consent、frequency cap、idempotency、retry 與 Mock Push delivery。
- 發送前 revalidation 與 sent-to-order conversion tracking。
- Unit tests、integration tests、Docker Compose 與 Swagger。

以下項目列為 Post-MVP：

- OpenTelemetry、Prometheus、rate limiting 與 production alerts。
- External queue、真實 Push provider 與 worker bounded concurrency。
- Control group、incremental conversion、完整 attribution 與 Growth dashboard。
- 統一的 client-safe error code、HTTP status mapping 與 internal error handling。

詳細演進設計請見 [Production Evolution](docs/prd/07-production-evolution.md)。

## Architecture

```text
Client
  |
  v
Gin API
  |
  +--> Member / Product / Cart / Order
  |
  +--> Campaign / Rule Engine
  |
  v
PostgreSQL
  |
  +--> domain_outbox
  |        |
  |        v
  |   Cart Recall Worker
  |        |
  |        v
  |   Cart Recall Journey
  |        |
  |        v
  +--> Notification Task
           |
           v
      Notification Worker
           |
           v
      In-app / Mock Push
```

MVP 由 Cart Recall Worker polling PostgreSQL Outbox。External publisher 與 queue 屬於 Post-MVP，不是目前已實作的架構。

## API

以下列出目前所有 application APIs。Request、response schema 與錯誤格式以 Swagger 為準。

### Health and Authentication

| Method | Path | Access | Purpose |
| --- | --- | --- | --- |
| `GET` | `/health` | Public | 服務健康檢查 |
| `POST` | `/auth/register` | Public | 註冊 Customer 並取得 JWT |
| `POST` | `/auth/login` | Public | 登入並取得 JWT |
| `GET` | `/users/me` | Customer/Admin | 查詢目前使用者 |

### Products

| Method | Path | Access | Purpose |
| --- | --- | --- | --- |
| `GET` | `/products` | Public | 查詢 active Products |
| `GET` | `/products/:id` | Public | 查詢 active Product |
| `GET` | `/admin/products` | Admin | 查詢所有 Products |
| `POST` | `/admin/products` | Admin | 建立 Product |
| `PUT` | `/admin/products/:id` | Admin | 修改 Product |
| `DELETE` | `/admin/products/:id` | Admin | 停用 Product |

### Cart and Orders

| Method | Path | Access | Purpose |
| --- | --- | --- | --- |
| `GET` | `/cart` | Customer/Admin | 查詢目前購物車 |
| `POST` | `/cart/items` | Customer/Admin | 加入購物車商品 |
| `PUT` | `/cart/items/:id` | Customer/Admin | 修改購物車商品數量 |
| `DELETE` | `/cart/items/:id` | Customer/Admin | 移除購物車商品 |
| `POST` | `/orders` | Customer/Admin | 將購物車轉換為訂單 |
| `GET` | `/orders` | Customer/Admin | 查詢自己的訂單 |
| `GET` | `/orders/:id` | Customer/Admin | 查詢自己的單筆訂單 |

### Public Campaigns

| Method | Path | Access | Purpose |
| --- | --- | --- | --- |
| `GET` | `/campaigns` | Optional JWT | 查詢目前可見且 eligible 的 Campaigns |
| `GET` | `/campaigns/:id` | Optional JWT | 查詢單一可見且 eligible 的 Campaign |
| `POST` | `/campaigns/:id/evaluate` | Optional JWT | 使用 server-side facts 判斷 eligibility |

`GET /campaigns` 支援 `product_id`、`limit` 與 `offset`。`limit` 預設及上限為 20。

### Admin Campaigns and Rules

| Method | Path | Access | Purpose |
| --- | --- | --- | --- |
| `POST` | `/admin/campaigns` | Admin | 建立 Draft Campaign |
| `GET` | `/admin/campaigns` | Admin | 查詢所有 Campaigns |
| `GET` | `/admin/campaigns/:id` | Admin | 查詢單一 Campaign 與 Rule |
| `PUT` | `/admin/campaigns/:id` | Admin | 修改 Draft Campaign |
| `POST` | `/admin/campaigns/:id/publish` | Admin | 發布 Campaign |
| `POST` | `/admin/campaigns/:id/pause` | Admin | 暫停 Campaign |
| `POST` | `/admin/campaigns/:id/resume` | Admin | 恢復 Campaign |
| `POST` | `/admin/campaigns/:id/archive` | Admin | 封存 Campaign |
| `POST` | `/admin/campaigns/:id/rules/validate` | Admin | 驗證目前 Rule |
| `POST` | `/admin/campaigns/:id/rules/evaluate` | Admin | 使用測試 facts 執行 Rule dry-run |

### Notification Preferences and Inbox

| Method | Path | Access | Purpose |
| --- | --- | --- | --- |
| `GET` | `/me/notification-preferences` | Customer/Admin | 查詢 marketing consent 與 channels |
| `PUT` | `/me/notification-preferences` | Customer/Admin | 修改 marketing consent 與 channels |
| `GET` | `/notifications` | Customer/Admin | 查詢已送達的 In-app notifications |
| `POST` | `/notifications/:id/open` | Customer/Admin | 將 In-app notification 標記為 opened |

### Admin Notification Operations

| Method | Path | Access | Purpose |
| --- | --- | --- | --- |
| `GET` | `/admin/notification-tasks` | Admin | 查詢所有 Notification Tasks |
| `GET` | `/admin/notification-tasks/:id` | Admin | 查詢單一 Notification Task |
| `POST` | `/admin/notification-tasks/:id/retry` | Admin | 人工 retry Failed Task |

### Admin Cart Recall Operations

| Method | Path | Access | Purpose |
| --- | --- | --- | --- |
| `GET` | `/admin/cart-recall-journeys` | Admin | 查詢所有 Cart Recall Journeys |
| `GET` | `/admin/cart-recall-journeys/:id` | Admin | 查詢單一 Cart Recall Journey |
| `POST` | `/admin/cart-recall-journeys/:id/cancel` | Admin | 取消尚未完成的 Journey |

## Run Locally

### Prerequisites

- Docker Engine 或 Docker Desktop。
- Docker Compose。

啟動 PostgreSQL、migration、API、Cart Recall Worker 與 Notification Worker：

```bash
docker compose up --build
```

或使用：

```bash
make up
```

啟動後：

- API：`http://localhost:8080`
- Swagger UI：`http://localhost:8080/swagger/index.html`
- PostgreSQL：`localhost:5432`

停止服務：

```bash
make down
```

PostgreSQL data 保存在 Docker volume。`make down` 不會刪除既有資料。

## Local Admin

Migration 會建立本機 Admin account：

- Email：`admin@example.com`
- Password：`Admin123!`

這組帳號只供 local development 與筆試驗證使用。部署到 shared 或 production environment 前，必須更換 seed 與 credential 管理方式。

## Verification

執行 unit tests：

```bash
make test
```

執行完整 Docker integration tests：

```bash
make integration-test
```

Integration test 會建立獨立的 containers、network 與 database volume，測試完成後自動清理。

執行 static analysis：

```bash
go vet ./...
```

重新產生 Swagger：

```bash
make swagger
```

## Repository Structure

```text
cmd/                  API and worker entrypoints
services/             Business rules and use cases
stores/               PostgreSQL persistence
models/               Domain and API models
middlewares/          Authentication, recovery and request logging
infra/postgres/        Database migrations
integration-tests/    End-to-end integration test suites
docs/prd/             Product and architecture documents
docs/swagger/         Generated OpenAPI files
```

## Known Limitations

- Public Campaign pagination 作用在 DB candidates；Rule Engine 篩選後，回傳數量可能少於 `limit`。
- Cart Recall 每批評估 20 個 Campaign candidates，最多評估 100 個。
- PostgreSQL Outbox polling 適合 MVP；大量事件需要 external queue。
- Push delivery 使用 Mock Provider。
- Sent-to-order conversion 是 observational metric，不等於 incremental conversion lift。
- OpenTelemetry、Prometheus 與 distributed rate limiting 尚未實作。
