# Non-functional Requirements

## Performance

- Public Campaign List/Detail API：p95 小於 300 ms。
- Rule evaluation：單一 Campaign p95 小於 50 ms；一次 request 最多評估 100 個 Campaign。
- Notification task creation：p95 小於 200 ms。
- Worker throughput 與 queue lag 必須可觀測。

## Consistency

- Cart、Order 與 domain event 使用 Transactional Outbox。
- Consumer 以 `event_id` 實作 inbox deduplication。
- Notification task 以 database unique idempotency key 防止重複建立。
- Campaign publish 與 rule version activation 必須在同一 transaction。
- Attribution 使用 unique constraint 防止重複計算。

## Security and Privacy

- Admin APIs 使用 JWT role authorization，未來可替換 RBAC。
- Public Campaign response 不揭露完整 audience rules、內部 tag 或 budget。
- Rule facts 必須來自 server-side data，不信任 client 宣告的 member level、tag、order count。
- Template variables 必須 escape，Deep Link 必須使用 allowlist scheme/domain。
- Device Token 與聯絡偏好不得寫入一般 logs。
- API 必須加入 rate limiting，Admin 與 Public key/credential scope 分離。

## Observability

### Structured Logs

必要欄位：

- `request_id`
- `trace_id`
- `event_id`
- `campaign_id`
- `journey_type`
- `journey_id`
- `notification_task_id`
- `decision`
- `reason_code`

`user_id` 僅在必要情境記錄，且不得使用 `event_origin` 作為使用者識別。

### OpenTelemetry

必要 spans：

- `campaign.list`
- `rule.evaluate`
- `event.consume`
- `cart_recall.evaluate`
- `repurchase.evaluate`
- `notification.create`
- `notification.deliver`
- `attribution.create`

### Prometheus Metrics

- `campaign_rule_evaluations_total`
- `campaign_rule_evaluation_duration_seconds`
- `journey_transitions_total`
- `notification_tasks_total`
- `notification_delivery_duration_seconds`
- `notification_retries_total`
- `event_consumer_lag_seconds`
- `attributed_orders_total`
- `attributed_order_amount_total`

## Reliability

- Worker 使用 bounded concurrency 與 context timeout。
- Retry 使用 exponential backoff 與 jitter。
- 超過最大 retry 次數進入 Failed/Dead Letter 狀態，支援人工 retry。
- Campaign/Rule Engine 暫時不可用時不得發送未驗證的優惠通知。
- Notification provider failure 不應阻塞 Cart/Order transaction。

## Data Retention

- Campaign 與 rule versions：至少保留兩年。
- Eligibility decision log：MVP 保留 90 天。
- Notification task 與 attribution：至少保留一年。
- Raw event payload：MVP 保留 30 天，再轉為摘要或刪除。

## Risks

- 同步跨 service 取得 facts 會增加 latency 與 availability coupling。
- Delayed trigger 若只依靠 process memory，部署或 crash 後會遺失；必須持久化。
- Rule 過度彈性會提高營運誤設風險；MVP 僅支援 allowlist facts/operators。
- Marketing consent、frequency cap 與 Campaign eligibility 若在不同時間判斷，可能產生 race；發送前必須 final revalidation。
- Campaign promised benefit 與 checkout 結果不一致會造成信任問題；Notification 內容不得保證最終價格或庫存。
