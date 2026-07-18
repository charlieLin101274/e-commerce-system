# Campaign and Rule Engine

## Context

Campaign Service 管理活動生命週期與展示資料；Rule Engine 判斷商品、購物車與必要 Member facts 是否符合資格。MVP 另以受限的 Benefit Calculator 計算 fixed amount 或 percentage discount，避免將 eligibility 與 pricing responsibility 混入同一套任意規則。

Checkout 仍是最終價格、庫存與 promotion eligibility 的 source of truth。Campaign API 與 Notification 中的折扣結果僅能使用相同 benefit configuration 計算，且不得保證結帳前價格或庫存不變。

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
benefit_type
benefit_value
maximum_discount_amount
rule_set_id
created_by
created_at
updated_at
published_at
```

商品適用範圍以 `campaign_products` 與 Product Category 定義。Brand scope 列為 post-MVP。

## APIs

### Customer

```text
GET /campaigns
GET /campaigns/:id
POST /campaigns/:id/evaluate
```

- List/Detail 必須根據 JWT user 或 anonymous facts 執行 eligibility。
- Customer `evaluate` 使用 server-side `CampaignDiscoveryContext`，回傳 eligible、公開 reason code 與適用時的 benefit preview，不接受 client 宣告 member facts，也不回傳內部規則內容。

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

- Admin `rules/evaluate` 是 dry-run 工具，接受明確標示的測試 context，回傳 condition-level decision、missing facts 與 validation errors。
- Customer 與 Admin API 必須呼叫相同 evaluator；差異僅在 input source 與 response detail，不得實作兩套 rule semantics。

## Evaluation Context

每次 evaluation 必須指定 context type。Rule publish 時需驗證所引用 facts 是否屬於該 context 的 allowlist。

```text
CampaignDiscoveryContext
------------------------
member (optional for anonymous request)
product

CartRecallContext
-----------------
member
cart
matched_product
```

- Campaign list/detail 使用 `CampaignDiscoveryContext`，一次以單一 product 或 Campaign product scope 評估。
- Cart Recall 使用 `CartRecallContext`，每個 candidate product 獨立評估，不以整個 cart 隱含代替特定商品。
- Anonymous request 不具有 `member.*` facts。引用不存在 fact 的 condition 結果一律為 `false`，evaluation 本身不報錯。
- Internal decision log 記錄 `MISSING_FACT` 與缺少的 fact name；public response 僅回傳不揭露規則內容的 `NOT_ELIGIBLE`。

## Rule Engine Model

Rule 使用 declarative JSON 儲存，不允許營運人員提交 SQL 或 expression code。

```json
{
  "eligibility_rule": {
    "operator": "and",
    "conditions": [
      {"fact": "product.category", "operator": "eq", "value": "electronics"},
      {"fact": "product.price", "operator": "gt", "value": 500000}
    ]
  },
  "benefit": {
    "type": "percentage",
    "value": 10,
    "maximum_discount_amount": 50000
  }
}
```

`eligibility_rule` 由 Rule Engine 評估，`benefit` 由 Benefit Calculator 計算，兩者為同一 Campaign configuration 中彼此獨立的定義。

金額一律使用最小貨幣單位的 integer。以上範例代表價格高於 5,000、折扣 10%，且折扣上限為 500。Currency、percentage precision 與 rounding mode 必須由系統設定統一決定，不使用 floating point 計算。

## Supported Facts

### MVP

- `member.id`
- `member.level`
- `member.tags`
- `product.id`
- `product.category`
- `product.price`
- `product.status`
- `cart.total_price`
- `cart.item_count`

Order history facts、Brand 與時間運算列為 post-MVP。

## Supported Operators

- `eq`
- `gt`, `gte`, `lt`, `lte`
- `in`
- `contains`

Engine 必須進行 fact type validation，不允許以字串比較數字或時間。

## Benefit Calculator

MVP 支援：

- `fixed_amount`：固定折扣金額。
- `percentage`：百分比折扣，可設定 `maximum_discount_amount`。
- Applied discount 不得小於零、超過 eligible item amount，或超過設定的 maximum discount。
- Benefit calculation 使用已通過 eligibility 的特定商品作為 input，輸出 original amount、discount amount 與 final amount。

Promotion stacking、Buy X Get Y、tiered discount、coupon、跨商品組合與 dynamic pricing 均列為 post-MVP。

## Evaluation Result

```json
{
  "eligible": false,
  "campaign_id": "uuid",
  "rule_version": 3,
  "reason_code": "PRODUCT_PRICE_NOT_MATCHED",
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
2. Campaign ID lexical order，作為 deterministic tie-breaker。

結束時間、商品金額與建立時間等 ranking 維度列為 post-MVP。

## Risks

- Rule facts 若逐項跨 service 查詢，容易造成 latency 與 cascading failure。
- MVP 可同步查詢；規模增加後應建立 Member/Product fact projection。
- Rule JSON 必須限制最大 depth、condition count 與 payload size，避免 evaluation abuse。
