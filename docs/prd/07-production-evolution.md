# Production Evolution

## Context

七天 MVP 使用 PostgreSQL Outbox polling 與 structured logs，已足夠驗證完整的 Cart Recall 流程。

正式流量增加後，需要讓事件處理可以獨立擴充，也需要更容易追蹤一個 request 如何經過 API、worker、Rule Engine 與 Notification provider。本文件說明後續演進方向，不屬於七天 MVP acceptance criteria。

## Replace Direct Outbox Polling with a Queue

### Current Flow

```text
Cart / Order transaction
        |
        v
PostgreSQL domain_outbox
        |
        v
Cart Recall worker polling
        |
        v
Journey and Notification Task
```

這個做法簡單，也能避免 business data 已寫入但 event 遺失。不過 event 數量增加後，worker 持續 polling business database 會增加 database load，也不容易讓不同 consumer 各自擴充。

### Target Flow

```text
Cart / Order transaction
        |
        v
PostgreSQL domain_outbox
        |
        v
Outbox publisher
        |
        v
External queue
        |
        +-------------------+
        |                   |
        v                   v
Cart Recall consumer   Analytics consumer
        |
        v
Journey and Notification Task
```

Outbox 仍然需要保留。API 不應在 business transaction 完成後直接 publish message，否則會再次產生 database 與 queue 的 dual-write 問題。

### Delivery Rules

- Publisher 只處理尚未成功發布的 Outbox records。
- Message 使用 `event_id` 作為唯一識別。
- Consumer 仍需使用 Inbox 或等效機制做 idempotency。
- 系統採 at-least-once delivery，不假設 queue 只投遞一次。
- 暫時性錯誤使用 retry 與 exponential backoff。
- 超過 retry 上限的 message 進入 Dead Letter Queue。
- 必須提供 Dead Letter inspection、人工 replay 與 replay audit log。
- Queue lag、publish failure、consumer retry 與 Dead Letter 數量必須有 metrics 和 alerts。

Queue 可以使用 Kafka、SQS、Pub/Sub 或其他 managed service。選擇時應考慮 ordering、retry、維運成本與團隊現有 infrastructure，而不是只看 throughput。

## Use Coupon Entitlements for Discounts

### Goal

MVP 的 Notification 直接顯示 Campaign benefit preview，Checkout 仍要重新確認 Campaign eligibility。這適合展示流程，但正式系統不應讓使用者已經收到的優惠完全依賴當下 Campaign 狀態。

Post-MVP 建議改成發 Coupon：

- Campaign 負責決定「誰可以獲得優惠」。
- Coupon Template 負責定義「優惠內容與使用條件」。
- User Coupon 代表「某位使用者實際取得的優惠權利」。
- Checkout 驗證並核銷 User Coupon，不需要重新呼叫 Campaign Service 判斷完整 eligibility rule。

Campaign 仍然存在，但角色從直接提供結帳優惠，改為選擇與發放 Coupon 的 orchestration source。

### Target Flow

```text
Cart Recall Journey
        |
        v
Campaign and Rule evaluation
        |
        v
Issue User Coupon
        |
        v
Create Notification Task with coupon_id
        |
        v
User returns to Cart
        |
        v
Checkout validates and redeems Coupon
```

Notification 應帶入 `coupon_id` 或 Coupon deep link，而不是只顯示一個沒有 entitlement 的折扣 preview。

### Why Coupon Is Safer

- Campaign 在通知送出後被 Pause 或修改時，已發出的 Coupon 是否仍有效可以由明確 policy 決定。
- Checkout 只依賴 Coupon contract，不必重跑可能已變更的 Campaign rule。
- 每位使用者是否已領取、是否使用、何時到期都可以稽核。
- 可以限制總發行量、每人領取次數、每張 Coupon 使用次數與活動成本。
- Retry 可以透過 idempotency key 避免發出多張相同 Coupon。
- Customer Service 可以查到使用者實際取得的優惠，而不是只看到曾經符合某個 Campaign。

### Suggested Data Model

```text
CouponTemplate
--------------
id
name
benefit_type
benefit_value
maximum_discount_amount
minimum_order_amount
product_ids
categories
starts_at
ends_at
total_issuance_limit
status
version

UserCoupon
----------
id
coupon_template_id
coupon_template_version
user_id
source_type
source_campaign_id
source_journey_id
status
issued_at
valid_from
expires_at
redeemed_at
redeemed_order_id
idempotency_key
benefit_snapshot
```

`benefit_snapshot` 保存發放當下的優惠內容。Coupon Template 之後修改時，已發出的 Coupon 不會在沒有明確 migration policy 的情況下被偷偷改變。

### Coupon Lifecycle

```text
Issued -> Active -> Redeemed
             |
             +-> Expired
             |
             +-> Revoked
```

- `Issued`：已建立，但尚未到可使用時間。
- `Active`：可以在 Checkout 使用。
- `Redeemed`：已被訂單成功核銷。
- `Expired`：超過到期時間。
- `Revoked`：因風險、退款或營運決策失效。

狀態不能只依賴 background job 更新。Checkout 仍需以 `valid_from <= now < expires_at`、status 與使用條件做即時檢查。

### Issuance Rules

- 使用 `journey_type:journey_id:coupon_template_version` 作為發放 idempotency key。
- 建立 User Coupon 與 Journey transition 應在同一 transaction，或使用 Saga / Outbox 保證最終一致性。
- 發放前檢查 Campaign、Rule、consent 與 frequency cap。
- 發放成功後才建立包含 Coupon 的 Notification Task。
- Notification retry 只能重送同一張 Coupon，不能重新發一張。
- Campaign Pause 後停止發放新 Coupon；已發 Coupon 是否失效由 Campaign cancellation policy 決定。
- Coupon 到期時間應保存為明確 timestamp，不在讀取時依目前 Campaign 結束時間動態推算。

### Checkout Rules

- Client 只能提交 `coupon_id`，不能提交折扣金額。
- Checkout 必須確認 Coupon 屬於目前 user。
- Checkout 必須檢查 status、有效時間、商品範圍、minimum order amount 與使用次數。
- 折扣計算仍使用共用 Benefit Calculator，不能在 Checkout 重新實作另一套 rounding logic。
- Coupon 核銷、Order 建立與庫存扣除應在同一 database transaction；若跨 service，需使用 reservation 與 Saga。
- Database constraint 必須保證單次使用 Coupon 不會被兩個 concurrent Orders 同時核銷。
- Client 收到的失敗原因應使用穩定 error code，例如 `coupon_expired`、`coupon_not_applicable`、`coupon_already_redeemed`。

### Campaign and Coupon Relationship

```text
Campaign
  |
  +--> eligibility rule
  |
  +--> coupon_template_id
  |
  +--> issuance policy
          |
          v
      User Coupon
          |
          v
       Checkout
```

Campaign 不應成為已發 Coupon 每次讀取與核銷時的 hard dependency。User Coupon 必須保存足以獨立驗證的 template version、benefit snapshot、有效時間與適用範圍。

### Metrics

- `coupon_issued_total`
- `coupon_issue_failed_total`
- `coupon_redeemed_total`
- `coupon_expired_total`
- `coupon_revoked_total`
- Coupon issuance-to-redemption conversion rate。
- Coupon liability 與預估 promotion cost。

Coupon redemption 仍不等於 incremental growth。是否真的帶來額外訂單，仍需搭配 control group 比較。

## Distributed Tracing

### Goal

`request_id` 只能辨識單一 HTTP request。當流程跨過 Outbox、queue 與 worker 後，需要使用 distributed trace 才能把整條流程串起來。

例如：

```text
POST /cart/items
    trace_id: abc
        |
        v
write domain_outbox
    trace_id: abc
        |
        v
publish cart.item_added
    trace_id: abc
        |
        v
cart_recall.evaluate
    trace_id: abc
        |
        v
notification.create
    trace_id: abc
        |
        v
notification.deliver
    trace_id: abc
```

同一條流程共用 `trace_id`，每個操作則有自己的 `span_id`。這樣可以查到 request 經過哪些元件、每個步驟花多久，以及錯誤發生在哪裡。

### Trace Propagation

- HTTP 使用 W3C Trace Context 的 `traceparent` 與 `tracestate` headers。
- Outbox event 保存建立事件時的 trace context。
- Publisher 將 trace context 放入 queue message metadata，不混入 business payload。
- Consumer 從 message metadata 建立 consumer span，再把新的 context 傳給 service 與 store。
- 呼叫 external provider 時繼續傳遞 trace context；若 provider 不支援，至少在本地 span 記錄結果與 latency。
- Trace ID 只用於追蹤系統流程，不作為 user identity、idempotency key 或 business key。

建議 spans：

- `http.request`
- `outbox.write`
- `outbox.publish`
- `event.consume`
- `campaign.candidates`
- `rule.evaluate`
- `cart_recall.evaluate`
- `notification.create`
- `notification.deliver`

Span attributes 不得放入 token、password、完整 member tags 或 notification content 等敏感資料。

## Context-based Logging

### Goal

Logger 應由 `context.Context` 取得。只要上游已放入 trace context，後續所有使用 logger 的地方就能自動帶入共同欄位。

預期使用方式：

```go
log := logger.FromContext(ctx)
log.Error().Err(err).Str("campaign_id", campaignID.String()).Msg("evaluate campaign")
```

Log output 自動包含：

```json
{
  "trace_id": "abc",
  "span_id": "def",
  "request_id": "req-123",
  "campaign_id": "campaign-id",
  "message": "evaluate campaign"
}
```

### Logging Rules

- HTTP middleware 建立或接收 trace context，並把 logger 放入 request context。
- Queue consumer 建立 consumer span，再把 logger 放入 consumer context。
- Service、store 與 provider 都沿用收到的 context，不自行建立 `context.Background()`。
- `logger.FromContext(ctx)` 自動加入 `trace_id`、`span_id` 與 `request_id`。
- Domain-specific fields，例如 `campaign_id`、`journey_id` 與 `reason_code`，仍由最了解該資料的呼叫點加入。
- 同一個 error 原則上只在 API、worker 或 scheduled job boundary 記錄一次。
- 不在每一層重複記錄相同 error，避免 log noise。

## Error Handling

### Goal

Client 需要穩定且安全的 error code；工程人員則需要原始 internal error 進行排查。兩者不能直接使用同一段錯誤訊息。

Error 可以分為：

```text
Internal Error
    原始 DB、provider 或程式錯誤
    只保留在 server side

Application Error Code
    穩定、可供 client 判斷的 code
    例如 campaign_not_found、campaign_conflict

HTTP Status
    Transport layer 對 error code 的 HTTP mapping
    例如 404、409、500
```

### Recommended Model

Domain 與 service layer 不應直接依賴 HTTP。`AppError` 保存公開 error code、安全訊息與 internal cause；HTTP status 由 API layer 的 registry 統一 mapping。

```go
type AppError struct {
    Code        string
    SafeMessage string
    Cause       error
}

var HTTPStatusByCode = map[string]int{
    "invalid_request":    400,
    "campaign_not_found": 404,
    "campaign_conflict":  409,
    "internal_error":     500,
}
```

`Cause` 支援 `errors.Is` 與 `errors.As`，但不得被序列化到 client response。

Client response：

```json
{
  "error": {
    "code": "campaign_conflict",
    "message": "campaign was modified, please retry"
  }
}
```

Server log：

```json
{
  "level": "error",
  "trace_id": "abc",
  "request_id": "req-123",
  "error_code": "campaign_conflict",
  "error": "update campaign: serialization failure",
  "message": "API request failed"
}
```

### Mapping Rules

- Validation error 回傳 `400` 與明確但安全的 error code。
- Authentication error 回傳 `401`。
- Authorization error 回傳 `403`。
- Resource not found 回傳 `404`。
- Version conflict、idempotency conflict 或非法 state transition 回傳 `409`。
- Frequency limit 可依 API contract 使用 `429`；若只是 Journey skip，則保存 reason code，不回傳 HTTP error。
- 未預期的 DB、provider 或程式錯誤一律回傳一般化的 `internal_error` 與 `500`。
- Client 不得收到 SQL、table name、stack trace、provider response body 或 internal hostname。

### Logging Boundary

- Service 與 store 回傳 wrapped error，但通常不記錄 log。
- API middleware、worker loop 或 scheduled job 是主要 logging boundary。
- Boundary log 記錄 internal cause、公開 error code、trace context 與必要 domain identifiers。
- Client response 只使用 safe message。
- 已知的 `4xx` business error 可使用 info 或 warn；未預期的 `5xx` 使用 error。
- Panic 由 recovery middleware 記錄 stack trace，client 仍只收到一般化的 `internal_error`。

## Implementation Plan

1. 導入 OpenTelemetry SDK，先完成 HTTP trace extraction 與 response propagation。
2. 讓 logger 從 context 自動取得 trace、span 與 request IDs。
3. 重構 `apperror`，分離 public code、safe message 與 internal cause。
4. 在 API boundary 建立 error code 到 HTTP status 的統一 mapping。
5. Outbox schema 加入 trace context metadata。
6. 建立 Outbox publisher 與 external queue。
7. Queue consumer extraction trace context，並保留 Inbox deduplication。
8. 建立 Coupon Template、User Coupon、發放 idempotency 與 Checkout redemption contract。
9. Cart Recall 改為先發 Coupon，再建立包含 `coupon_id` 的 Notification Task。
10. 補 trace propagation、error sanitization、Coupon concurrent redemption、duplicate delivery 與 replay integration tests。

## Risks

- Trace context 若直接放入 business payload，容易污染 event schema，因此應使用 metadata 或獨立欄位。
- Sampling 太低可能找不到低頻錯誤；sampling 太高則增加 storage cost。
- Error code 數量若沒有管理，client contract 會快速失控。
- 同一錯誤在每層都記錄會造成 log noise，並提高 Loki 儲存成本。
- Queue 不會自動提供 exactly-once business processing；consumer idempotency 仍然必要。
- Campaign Pause、Coupon revoke 與已送出 Notification 之間必須有明確 policy，否則使用者可能看到無法使用的優惠。
- Coupon 發放與核銷若缺少 idempotency 及 database constraint，可能造成重複折扣或超發。
- 已發 Coupon 會形成 promotion liability，需要 issuance limit、成本監控與到期政策。
