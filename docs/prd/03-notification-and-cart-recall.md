# Notification and Cart Recall

## Notification Service

Notification Service 負責「如何送達」，Journey Service 負責「為何、何時、送給誰」。Notification 不自行執行 Campaign matching。

## Notification Data

```text
NotificationTemplate
--------------------
id
channel
title_template
body_template
deep_link_template
version
status

NotificationTask
----------------
id
user_id
campaign_id
journey_type
journey_id
channel
status
idempotency_key
scheduled_at
attempt_count
next_attempt_at
sent_at
opened_at
failure_code
payload
```

## Channels

MVP 支援：

- In-app message。
- Mock Push provider。

APP Push provider 整合可在 contract 穩定後替換 Mock implementation。

## Delivery States

```text
Pending → Processing → Sent → Delivered → Opened
                    └→ RetryScheduled → Processing
                    └→ Failed
                    └→ Cancelled
```

## Retry and Idempotency

- Idempotency key：`journey_type:journey_id:channel:template_version`。
- PostgreSQL unique index 保證同一 key 只建立一筆 task。
- Exponential backoff 加 jitter。
- Provider timeout、429、5xx 可 retry。
- Invalid token、missing consent、invalid payload 不 retry。
- Worker crash 後可回收逾時 Processing task。

## Frequency Control

MVP 預設：

- 同一 Campaign 對同一 user，24 小時最多一次。
- 同一 user 每日最多兩則 marketing notification。
- 同一 Journey 只允許一個 active task。
- Transactional notification 不受 marketing cap，但不在本 PRD scope。

## Cart Recall Trigger

### Trigger Event

`cart.item_added` 建立或更新該 user/cart 的 recall journey。

```text
CartRecallJourney
-----------------
id
user_id
cart_id
source_event_id
status
evaluate_at
campaign_id
rule_version
notification_task_id
cancel_reason
created_at
updated_at
```

### Journey States

```text
Scheduled → Evaluating → Eligible → NotificationPending → Sent → Converted
                       └→ Skipped
Scheduled/Evaluating/Eligible/NotificationPending → Cancelled
```

### Revalidation

等待時間結束後依序確認：

1. Source event 尚未處理過。
2. Cart 仍存在且至少包含一項商品。
3. Cart 自最後一次異動後已超過 configured delay。
4. 商品仍為 active 且有庫存。
5. 尚未完成包含相關商品的訂單。
6. Campaign 為 Running。
7. Rule Engine 判定 user/cart/product eligible。
8. Member consent 與 notification channel 有效。
9. Frequency cap 尚未超過。
10. 發送前再執行一次 lightweight validation。

### Cancellation Events

- `cart.cleared`
- `order.completed`
- Campaign paused/ended
- Product inactive

取消操作必須 idempotent。已送出的 notification 不回收，但 attribution 必須正確停止或更新。

## APIs

```text
GET  /notifications
POST /notifications/:id/open

GET  /admin/notification-tasks
GET  /admin/notification-tasks/:id
POST /admin/notification-tasks/:id/retry

GET  /admin/cart-recall-journeys
GET  /admin/cart-recall-journeys/:id
POST /admin/cart-recall-journeys/:id/cancel
```

## Skip and Failure Codes

- `CART_EMPTY`
- `CART_CHANGED_RECENTLY`
- `PRODUCT_INACTIVE`
- `OUT_OF_STOCK`
- `ORDER_ALREADY_COMPLETED`
- `NO_ELIGIBLE_CAMPAIGN`
- `MEMBER_NOT_ELIGIBLE`
- `MARKETING_CONSENT_DISABLED`
- `NO_NOTIFICATION_CHANNEL`
- `FREQUENCY_LIMIT_REACHED`
- `CAMPAIGN_ENDED`
- `PROVIDER_TEMPORARY_FAILURE`
- `PROVIDER_PERMANENT_FAILURE`
