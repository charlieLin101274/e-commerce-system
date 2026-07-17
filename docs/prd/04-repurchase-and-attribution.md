# Repurchase and Attribution

## Repurchase Service

Repurchase Service 根據已完成訂單推算下一次可能需要相同商品的時間。它與 Cart Recall 的差異如下：

| 項目 | Cart Recall | Repurchase |
| --- | --- | --- |
| Trigger | Cart 行為 | Order completed |
| 意圖 | 尚未完成本次購買 | 完成購買後再次購買 |
| 等待時間 | 分鐘或小時 | 天或週 |
| 取消條件 | Cart 清空或完成訂單 | 再次購買、商品失效 |
| Attribution | Recall notification 至訂單 | Repurchase notification 至相同商品訂單 |

## Repurchase Policy

MVP 由 Admin 對 Product 設定固定週期：

```text
RepurchasePolicy
----------------
id
product_id
interval_days
remind_before_days
campaign_id
status
created_at
updated_at
```

Stage 5 不以 ML 預測消耗週期，也不根據個人購買頻率自動調整。

## Journey Data

```text
RepurchaseJourney
-----------------
id
user_id
source_order_id
source_order_item_id
product_id
policy_id
status
eligible_at
campaign_id
rule_version
notification_task_id
converted_order_id
cancel_reason
created_at
updated_at
```

## Flow

```text
Order Completed
      │
      ▼
Load active Repurchase Policy
      │
      ▼
Schedule eligible_at
      │
      ▼
Revalidate product, member and Campaign
      │
      ▼
Create Notification Task
      │
      ▼
User opens deep link and places order
      │
      ▼
Attribute conversion and schedule next cycle
```

## Business Rules

1. 只有 completed order item 可建立 journey。
2. 同一 source order item 最多一個 journey。
3. 同 user/product 同時最多一個 Scheduled 或 NotificationPending journey。
4. 使用者再次購買相同 Product 後，舊 journey 轉為 Converted 或 Cancelled，並以新 order item 計算下一週期。
5. 商品下架、無庫存、Campaign 結束或 Member 不再 eligible 時不發送。
6. 沒有 Campaign 時，MVP 不自動降級為一般回購提醒；此能力列入後續 stage。
7. Repurchase notification 受平台 marketing frequency cap 約束。

## APIs

```text
POST   /admin/repurchase-policies
GET    /admin/repurchase-policies
PUT    /admin/repurchase-policies/:id
DELETE /admin/repurchase-policies/:id

GET    /admin/repurchase-journeys
GET    /admin/repurchase-journeys/:id
POST   /admin/repurchase-journeys/:id/cancel
```

## Attribution

### Attribution Window

- Campaign 可設定 attribution window。
- Cart Recall MVP 預設 24 小時。
- Repurchase MVP 預設 7 天。
- Window 從 notification sent 或 opened 開始，MVP 採 sent time，後續可設定。

### Conversion Conditions

- Order 屬於同一 user。
- Order completed 時間位於 attribution window。
- Cart Recall：訂單至少包含一個 matched cart product。
- Repurchase：訂單包含 journey product。
- 同一 order/campaign/journey type 只計算一次。

### Attribution Data

```text
CampaignAttribution
-------------------
id
campaign_id
user_id
journey_type
journey_id
notification_task_id
order_id
attributed_amount
attributed_at
created_at
```

MVP 採 last eligible notification attribution；Control Group、incremental GMV 與 multi-touch model 不在本階段。

## Metrics

- Journey scheduled/eligible/skipped/cancelled。
- Notification sent/delivered/opened/failed。
- Conversion count。
- Attributed order amount。
- Time to conversion。
- Skip reason distribution。

Cart Recall 與 Repurchase 指標必須分開呈現，避免將兩種購買意圖混為同一 funnel。
