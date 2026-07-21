# Non-functional Requirements

## Context

本專案的實作時間為七天。MVP 的重點是完成一條可以實際驗證的 Cart Recall 流程，不是一次完成所有 production infrastructure。

本文件把需求分成兩類：

- `MVP`：本次交付必須完成，也有測試驗證。
- `Production Readiness Backlog`：正式上線前需要補上，但不列入七天 MVP 的驗收範圍。

## MVP Requirements

### Performance

- Public Campaign API 必須先在 PostgreSQL 篩選活動期間、商品範圍與 Rule context。
- 每次 Public request 最多取得 20 個 Campaign candidates，再交給 Rule Engine 判斷。
- Public Campaign List 支援 `limit` 與 `offset`。`limit` 預設及上限都是 20。
- Rule Engine 篩選後的結果可能少於 `limit`，MVP 不會為了補滿一頁而重複查詢。
- Campaign 排序固定使用 priority 由高到低，再使用 Campaign ID 排序。

### Consistency and Reliability

- Cart 與 Order 必須在原本的 transaction 內寫入 Outbox event。
- Cart Recall worker 以 `event_id` 做 inbox deduplication。
- Notification Task 使用 unique idempotency key，避免建立重複 task。
- Worker 使用 `FOR UPDATE SKIP LOCKED`，避免多個 worker 同時取得同一筆工作。
- Worker crash 後，超過 processing timeout 的工作可以被重新取得。
- Notification retry 使用 exponential backoff 與 jitter。
- 暫時性錯誤可以 retry；永久錯誤會進入 Failed 狀態。
- Campaign 或 Rule Engine 無法完成檢查時，不得直接發送通知。
- Notification provider 發生錯誤時，不得影響 Cart 或 Order transaction。

### Security and Privacy

- Admin API 使用 JWT role authorization。
- Public Campaign response 不回傳完整 eligibility rule、member tag 或其他內部資料。
- Member、Product 與 Cart facts 必須由 server 取得，不信任 client 自行提供的資料。
- Notification template variables 必須 escape。
- Deep Link 只能使用允許的 scheme 與 host。
- Token、credential 與 notification preference 不得寫入一般 application log。

### Observability

MVP 使用 structured JSON logs。依情境記錄以下欄位：

- `request_id`
- `event_id`
- `campaign_id`
- `journey_type`
- `journey_id`
- `notification_task_id`
- `decision`
- `reason_code`

不是每一筆 log 都會同時有全部欄位。例如一般 HTTP request 不一定有 Journey ID。

### Verification

- Service business rules 使用 unit tests 驗證。
- API、PostgreSQL migration 與 worker flow 使用 integration tests 驗證。
- `go test ./...`、`go vet ./...` 與 `make integration-test` 必須通過。

## Production Readiness Backlog

以下項目很重要，但不列入七天 MVP acceptance criteria：

### Platform Protection

- Public 與 Admin API rate limiting。
- Public 與 Admin 使用不同 credential scope。
- Request body size limit 與更完整的 abuse protection。

### Observability

- OpenTelemetry trace propagation 與 exporter。
- 核心 API、Rule Engine、event consumer、Journey 與 Notification spans。
- Prometheus registry 與 metrics endpoint。
- Worker throughput、consumer lag、retry、failure 與 Journey transition metrics。
- Grafana dashboard 與 production alerts。

### Worker and Event Platform

- 每個 DB 與 provider operation 的 context timeout。
- Worker bounded concurrency 與 graceful drain。
- 將 PostgreSQL Outbox polling 拆成 publisher、external queue 與 consumer。
- Dead-letter inspection 與 event replay 工具。
- 真實 Push provider integration。

### Performance and Operations

- 固定資料量的 load test 與 multi-instance soak test。
- 使用 `EXPLAIN ANALYZE` 驗證 Campaign candidate query 與 indexes。
- Eligibility decision logs、Notification Tasks 與 raw events 的清理工作。
- Campaign 與 rule version 長期保存政策。

## Current Event Flow

MVP 沒有使用外部 message broker。目前流程如下：

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
event_inbox deduplication
        |
        v
Cart Recall Journey
```

Outbox polling 可以讓七天 MVP 保留 transaction consistency，也能實際驗證完整流程。正式流量增加後，再拆成 publisher 與 external queue。

## Risks

- PostgreSQL polling 適合 MVP，但事件量增加後可能增加 database load。
- 同步讀取 Member、Product 與 Cart facts 會增加 latency 與 service coupling。
- 只看 sent-to-order conversion 不能證明通知帶來 incremental growth。
- Campaign eligibility、庫存與 consent 都可能在不同時間改變，因此發送前仍要再次檢查。
- Notification 顯示的優惠只是 preview，Checkout 才是最後價格與庫存的 source of truth。
