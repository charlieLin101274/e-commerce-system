# Simple E-Commerce System

Golang、Gin 與 PostgreSQL based e-commerce MVP。

本專案是七天完成的 Cart Recall vertical slice。重點不是建立完整的 Growth Platform，而是實作一條可以驗證的流程：Campaign 設定、Rule Engine、購物車事件、延遲評估、Notification Task、retry，以及訂單轉換記錄。

The staged PRD for Campaign, Rule Engine, Notification, Cart Recall, and
Repurchase is available in [docs/prd](docs/prd/README.md).

## MVP Scope

七天 MVP 已完成：

- Campaign lifecycle、商品 scope 與 deterministic ranking。
- Rule validation、dry-run、versioning 與 server-side eligibility evaluation。
- PostgreSQL Transactional Outbox、inbox deduplication 與 persistent Cart Recall Journey。
- Notification consent、frequency cap、idempotency、retry 與 Mock Push delivery。
- 發送前 revalidation 與 sent-to-order conversion tracking。
- Unit tests、integration tests、Docker Compose 與 Swagger。

正式上線前仍需補上：

- OpenTelemetry、Prometheus、rate limiting 與 production alerts。
- External queue、真實 Push provider 與 worker bounded concurrency。
- Control group、incremental conversion、完整 attribution 與 Growth dashboard。

完整營運流程、KPI 與 guardrails 請見 [Growth and Operations](docs/prd/06-growth-and-operations.md)。

## Member APIs

- `POST /auth/register`
- `POST /auth/login`
- `GET /users/me`
- `GET /health`

## Commerce APIs

- `GET /products`
- `GET /products/:id`
- `GET|POST /admin/products`
- `PUT|DELETE /admin/products/:id`
- `GET /cart`
- `POST /cart/items`
- `PUT|DELETE /cart/items/:id`
- `POST|GET /orders`
- `GET /orders/:id`

## Run locally

```bash
docker compose up --build
```

The API listens on `http://localhost:8080`.

Swagger UI is available at `http://localhost:8080/swagger/index.html`.

## Local admin

The database migration creates a local admin account:

- Email: `admin@example.com`
- Password: `Admin123!`

This credential is intended for local development only. Replace the seed
strategy before deploying the application to a shared or production
environment.

Regenerate the Swagger specification after changing API annotations:

```bash
make swagger
```
