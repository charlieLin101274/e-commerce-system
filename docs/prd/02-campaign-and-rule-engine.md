# Campaign and Rule Engine

## Context

Campaign Service 是第一個新 domain。它管理活動生命週期與展示資料；Rule Engine 管理受眾與適用範圍。Promotion 在第一階段只保存優惠描述與 reference，不負責最終折扣計算。

## Campaign Lifecycle

```text
Draft → Scheduled → Running → Paused → Ended → Archived
```

- Draft：可編輯，不公開。
- Scheduled：已發布，尚未到開始時間。
- Running：有效且可被查詢及匹配。
- Paused：暫停曝光與新 trigger matching。
- Ended：超過結束時間。
- Archived：僅供稽核查詢。

狀態不得只靠 background job 更新；查詢時仍須以 `starts_at <= now < ends_at` 驗證。

## Campaign Data

```text
Campaign
--------
id
name
description
status
priority
starts_at
ends_at
promotion_title
promotion_description
rule_set_id
created_by
created_at
updated_at
published_at
```

商品適用範圍另存 `campaign_products`。Brand/Category scope 待 Product prerequisite 完成後加入。

## APIs

### Customer

```text
GET /campaigns
GET /campaigns/:id
POST /campaigns/:id/evaluate
```

- List/Detail 必須根據 JWT user 或 anonymous facts 執行 eligibility。
- `evaluate` 回傳 eligible 與公開 reason code，不回傳內部敏感規則內容。

### Admin

```text
POST   /admin/campaigns
GET    /admin/campaigns
GET    /admin/campaigns/:id
PUT    /admin/campaigns/:id
POST   /admin/campaigns/:id/publish
POST   /admin/campaigns/:id/pause
POST   /admin/campaigns/:id/archive
POST   /admin/campaigns/:id/rules/validate
POST   /admin/campaigns/:id/rules/evaluate
```

## Rule Engine Model

Rule 使用 declarative JSON 儲存，不允許營運人員提交 SQL 或 expression code。

```json
{
  "operator": "and",
  "conditions": [
    {"fact": "member.role", "operator": "eq", "value": "customer"},
    {"fact": "member.order_count", "operator": "gte", "value": 1},
    {
      "operator": "or",
      "conditions": [
        {"fact": "member.tag", "operator": "contains", "value": "vip"},
        {"fact": "cart.total_price", "operator": "gte", "value": 3000}
      ]
    }
  ]
}
```

## Supported Facts

### Stage 2 MVP

- `member.id`
- `member.role`
- `member.level`
- `member.tag`
- `member.registered_at`
- `member.order_count`
- `member.last_order_at`
- `product.id`
- `product.price`
- `product.status`
- `cart.total_price`
- `cart.item_count`
- `order.total_price`

## Supported Operators

- `eq`, `neq`
- `gt`, `gte`, `lt`, `lte`
- `in`, `not_in`
- `contains`
- `exists`
- `before`, `after`

Engine 必須進行 fact type validation，不允許以字串比較數字或時間。

## Evaluation Result

```json
{
  "eligible": false,
  "campaign_id": "uuid",
  "rule_version": 3,
  "reason_code": "MEMBER_ORDER_COUNT_NOT_MATCHED",
  "evaluated_at": "2026-07-17T10:00:00Z"
}
```

內部 decision log 額外保存 facts snapshot、matched condition IDs、failed condition ID 與 evaluation duration。敏感 member facts 不直接寫入一般 application logs。

## Rule Versioning

- 已發布 Campaign 的 rule version 不可原地修改。
- 修改規則時建立新 version。
- Trigger 與 notification task 保存 evaluate 時使用的 version。
- Revalidation 使用當下 active version，並記錄版本變更造成的排除。

## Campaign Ranking

同一使用者符合多個 Campaign 時依序比較：

1. Priority 較高。
2. 結束時間較早。
3. 適用商品金額較高。
4. 建立時間較早。
5. Campaign ID lexical order，作為 deterministic tie-breaker。

## Risks

- Rule facts 若逐項跨 service 查詢，容易造成 latency 與 cascading failure。
- MVP 可同步查詢；規模增加後應建立 Member/Product fact projection。
- Rule JSON 必須限制最大 depth、condition count 與 payload size，避免 evaluation abuse。
