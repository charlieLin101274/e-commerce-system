# Prerequisites

## Context

目前專案已具備可完成一次購買的 commerce MVP。這些能力是活動平台的 prerequisite，後續 stage 應在既有架構上增量開發。

## Completed Capabilities

### Member

- Customer 註冊與登入。
- Admin seed account。
- JWT Authentication 與 role-based Admin authorization。
- 查詢目前使用者。

### Product

- Admin 建立、修改、下架與查看商品。
- Customer 查看上架商品列表與詳細資訊。
- 商品價格、庫存與狀態。

### Cart

- 每位使用者一個購物車。
- 加入、修改、移除與查看購物車商品。
- 商品狀態與庫存檢查。

### Order

- 購物車轉換為訂單。
- PostgreSQL transaction 內鎖定庫存、扣除庫存、建立訂單與清空購物車。
- 使用者只能查看自己的訂單。

## Required Extensions

活動平台開始前或對應 stage 中，既有 domain 需要補上以下欄位與事件。

### Member Extensions

- `marketing_consent`
- `notification_channels`
- `member_level`
- `member_tags`
- `last_login_at`
- Device Token 或 mock endpoint

### Product Extensions

- Category 與活動標籤；Brand 可於 post-MVP 加入。
- SKU 可延後至多規格商品 stage。

### Cart Extensions

- Cart status 與 `last_activity_at`。
- Transactional Outbox 發布 `cart.item_added`、`cart.item_updated`、`cart.item_removed`、`cart.cleared`。

### Order Extensions

- Transactional Outbox 發布 `order.created` 與 `order.completed`。
- MVP 現有訂單建立即視為 completed；未來加入 payment 後改以 payment completed 判斷轉換。

## Event Contract

所有 domain event 至少包含：

```json
{
  "event_id": "uuid",
  "event_type": "cart.item_added",
  "aggregate_id": "uuid",
  "user_id": "uuid",
  "occurred_at": "2026-07-17T10:00:00Z",
  "schema_version": 1,
  "payload": {}
}
```

事件必須在 domain transaction 內寫入 Outbox，避免 DB commit 成功但 event publish 失敗。

## System Boundary

```text
Member / Product / Cart / Order
                │
                ├── synchronous query
                │
                └── domain events via Outbox
                            │
                            ▼
Campaign / Rule Engine / Journey / Notification
```

活動平台可以讀取必要 commerce data，但不得直接修改購物車、庫存或訂單資料。

MVP 中 Cart/Order API 不直接 publish message。Domain transaction 同時寫入 business data 與 Outbox event，再由 publisher 非同步送至 queue，避免 database commit 與 message publish 的 dual-write inconsistency。
