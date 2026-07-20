# Campaign and Rule Engine

## Context

Campaign Service 管理活動生命週期與展示資料；Rule Engine 判斷商品、購物車與必要 Member facts 是否符合資格。MVP 另以受限的 Benefit Calculator 計算 fixed amount 或 percentage discount，避免將 eligibility 與 pricing responsibility 混入同一套任意規則。

Checkout 仍是最終價格、庫存與 promotion eligibility 的 source of truth。Campaign API 與 Notification 中的折扣結果僅能使用相同 benefit configuration 計算，且不得保證結帳前價格或庫存不變。

## Campaign Lifecycle

```text
Draft ────────────────→ Archived
  │
  ▼
Scheduled ⇄ Paused
  │           ▲
  ▼           │
Running ──────┘
  │
  ▼
Ended ────────────────→ Archived
```

- Draft：可編輯，不公開。
- Scheduled：已發布，尚未到開始時間。
- Running：有效且可被查詢及匹配。
- Paused：暫停曝光與新 trigger matching；可 Resume，並依當下時間恢復為 Scheduled 或 Running。
- Ended：超過結束時間。
- Archived：僅供稽核查詢。

所有已發布狀態（包含 Paused）都受 `ends_at` 約束；當 `now >= ends_at` 時 effective status 為 Ended。狀態不得只靠 background job 更新；查詢時仍須以 `starts_at <= now < ends_at` 驗證。

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

Product Category 使用 canonical lowercase string：寫入 Product 與 Campaign scope 前必須先 TrimSpace 並轉為 lowercase，比對採 exact match。

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
POST   /admin/campaigns/:id/resume
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

金額一律使用最小貨幣單位的 integer。以上範例代表價格高於 5,000、折扣 10%，且折扣上限為 500。MVP percentage precision 為整數百分比，rounding mode 為向下取整（floor），不使用 floating point 計算。Campaign preview、Notification 與 Checkout 必須直接共用同一個 Benefit Calculator，不得各自重作 rounding logic。

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

## Stage 2 Implementation Decisions

- MVP 不另設 rule write API；Admin 透過 Campaign Create/Update request 寫入 `context_type` 與 `eligibility_rule`。`rules/validate` 與 `rules/evaluate` 分別維持 validation 與 dry-run responsibility。
- Campaign 只有 Draft 可由既有 Update API 編輯。資料層已保存 immutable rule versions 與 active version，但「已發布 Campaign 切換至新 rule version」尚無公開 Admin endpoint；此能力在營運需要修改已發布規則前另行補充，不屬 Stage 2 MVP 驗收阻擋項目。
- Draft 每次明確帶入 `eligibility_rule` 的 Update 都建立新 version，即使 JSON 與前一版相同。MVP 優先保留完整修改軌跡，不進行 semantic deduplication。
- `member_level` 與 `member_tags` 僅作為必要 Member facts 保存及讀取，不在 Stage 2 建立 audience engine 或會員標籤維護 API。
- Cart Recall 的 member、cart 與 matched product facts 由 Stage 4 journey 組裝；Stage 2 僅提供相同 evaluator、context validation 與 Admin dry-run contract。
- Browse mode 未提供 `product_id` 時不合成 product facts。依 missing fact 規則，引用 `product.*` 的 condition 為 `false`；此類 Campaign 不出現在 browse 結果。若產品需求改為 browse 僅套用 Member rule，需另行修改 contract，不在 evaluator 中隱含跳過 Product condition。
- Product Category condition value 與 Product fact 都 canonicalize 為 TrimSpace 後的 lowercase；`eq` 與 `in` 仍採 exact match。
- `member.tags` 在 MVP 僅支援 `contains`。不接受可通過 validation 但 evaluator 無明確語意的 array `eq` 或 `in`。
- Customer List/Detail/Evaluate 的 decision log 採 synchronous best-effort write：保存失敗寫 structured error log，但不使 customer request 失敗。Admin dry-run 不寫入正式 decision log，避免測試 facts 污染稽核資料。後續流量增加時再改為 batch/async pipeline。
- Nested OR/AND 的 first failure 依 condition deterministic traversal order 記錄。MVP 不計算最小失敗證明或重建 group-level causal explanation。
- Public API 收到 inactive Product ID 時維持 resource not found semantics，不回傳該 Product 的 Campaign eligibility。
- Prometheus registry、distributed rate limiting 與 rule payload byte-size limit 尚無共用 infrastructure。MVP 已限制 rule depth 與 condition count；上述能力列為 external production exposure 前的 platform follow-up。

## Campaign Ranking

同一使用者符合多個 Campaign 時依序比較：

1. Priority 較高。
2. Campaign ID lexical order，作為 deterministic tie-breaker。

結束時間、商品金額與建立時間等 ranking 維度列為 post-MVP。

## Risks

- Rule facts 若逐項跨 service 查詢，容易造成 latency 與 cascading failure。
- MVP 可同步查詢；規模增加後應建立 Member/Product fact projection。
- Rule JSON 必須限制最大 depth、condition count 與 payload size，避免 evaluation abuse。
