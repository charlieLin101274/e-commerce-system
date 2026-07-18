# Notification and Cart Recall

## Purpose

Cart Recall 的目的不是發送一般性的活動通知，而是針對已對特定商品展現明確購買意圖、但尚未完成下單的使用者，重新喚起其購買需求並引導回到購買流程。

系統以使用者將商品加入購物車作為購買意圖訊號，並在發送前重新確認購物車、商品、庫存、訂單及 Campaign eligibility。僅在購買意圖仍有效且尚未完成相關商品訂單時發送通知，以促成使用者完成下單並提升高意圖使用者的 conversion rate，而非將 Cart Recall 作為廣泛觸達或一般 Campaign promotion 的管道。

## Notification Service

Notification Service 負責「如何送達」，Journey Service 負責「為何、何時、送給誰」。Notification 不自行執行 Campaign matching。

MVP 將 Cart Recall Trigger 與 Notification Delivery 實作為可獨立執行的 worker/component，先保留在同一 repository，不要求拆成獨立 microservice。API Server 只在 Cart transaction 寫入 Outbox event，由 publisher 發送至 queue；Trigger worker 消費事件、建立 Journey、執行 Rule evaluation 與建立 Notification Task。

MVP 只 mock 外部 Push delivery provider，不 mock Trigger business flow。Mock provider 必須寫入 structured log，並將 Notification Task 更新為對應 delivery state。

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
matched_product_ids
matched_products_snapshot
notification_task_id
converted_order_id
cancel_reason
created_at
updated_at
```

`matched_product_ids` 用於 revalidation 與 conversion 判斷。Journey 在延遲評估成功時寫入 `matched_products_snapshot`，至少保存 product ID、category、unit price、quantity 與 evaluated benefit；不得依賴之後可能再次變動的 cart content 重建當時的匹配結果。

### Journey States

```text
Scheduled → Evaluating → Eligible → NotificationPending → Sent → Converted
                       └→ Skipped
Scheduled/Evaluating/Eligible/NotificationPending → Cancelled
```

### Revalidation

等待時間結束後依序確認：

1. Source event 已通過 inbox deduplication，重複投遞不建立新 Journey。
2. Cart 仍存在且至少包含一項商品。
3. Cart 自最後一次異動後已超過 configured delay。
4. 商品仍為 active 且有庫存。
5. 尚未完成包含相關商品的訂單。
6. Campaign 為 Running。
7. Rule Engine 判定 user/cart/product eligible。
8. Member consent 與 notification channel 有效。
9. Frequency cap 尚未超過。
10. 發送前再執行一次 lightweight validation。

## MVP Conversion Tracking

MVP 不建立 `CampaignAttribution` table 或 GMV attribution model。消費 `order.completed` 時，若同一 user 的訂單在通知送出後 24 小時內包含至少一個 `matched_product_ids`，則：

- Journey 更新為 `Converted` 並記錄 `converted_order_id`。
- 寫入包含 campaign、journey、notification task、order 與 matched product IDs 的 structured event log。
- 同一 Journey 只能記錄一次 conversion，並以 database constraint 或 idempotent transition 保證不重複。

Attribution window configuration、attributed amount、last-touch/multi-touch model 與 replay 均列為 post-MVP。

### Cancellation Events

- `cart.cleared`
- `order.completed`
- Campaign paused/ended
- Product inactive

取消操作必須 idempotent。已送出的 notification 不回收；後續 conversion event 仍依 MVP Conversion Tracking 的條件判斷。

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
