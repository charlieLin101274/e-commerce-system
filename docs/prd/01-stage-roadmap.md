# Stage Roadmap

## Proposal

每個 stage 都必須能獨立部署與驗收。後續 stage 只能依賴已完成 stage 的公開 contract，不得直接耦合內部 table。

## Stage 0 — Commerce Prerequisite

### Status

大部分已完成。

### Remaining Work

- Member marketing consent 與 notification channel。
- Product brand/category metadata。
- Cart/Order Transactional Outbox。

### Exit Criteria

- 既有購買流程 integration test 通過。
- Cart/Order event 可被 idempotently 消費。

## Stage 1 — Campaign Service

### Goal

營運人員可以管理活動，使用者可以查詢「此刻對自己有效」的活動。

### Scope

- Campaign CRUD、狀態與有效期間。
- Campaign priority。
- 活動商品範圍。
- 簡化版 Promotion metadata，只描述優惠，不負責 checkout 計價。
- Public Campaign List/Detail API。
- Admin Campaign API。

### Exit Criteria

- Admin 可建立 Draft Campaign 並發布。
- 未開始、已結束、Paused、Archived 活動不出現在 public API。
- 使用者查詢只會看到通過 Rule Engine 的活動。
- 同一時間查詢結果依 priority 與結束時間排序。

## Stage 2 — Audience Rule Engine

### Goal

Campaign 可設定哪些使用者與商品符合資格，並提供一致、可稽核的 evaluate result。

### Scope

- Member、Product、Cart、Order rule facts。
- AND/OR rule group。
- 基礎 operators。
- Rule validation、versioning 與 dry-run evaluate API。
- Eligibility decision log。

### Exit Criteria

- Campaign publish 前 rule 必須通過 validation。
- 相同 facts 與 rule version 產生相同結果。
- Public Campaign API 套用相同 Rule Engine。
- Decision log 包含 matched rules 與第一個失敗原因。

## Stage 3 — Notification Service

### Goal

建立可靠、可重試且不重複的通知管線，先支援 In-app 與 Mock Push。

### Scope

- Notification template。
- Notification Task。
- Outbox consumer。
- Idempotency、retry、dead-letter state。
- Delivery status 與 open event。
- Member consent 與 frequency cap。

### Exit Criteria

- 相同 idempotency key 只建立一個 task。
- 暫時性失敗可 retry，永久失敗不無限重試。
- 未授權行銷通知的會員不會收到 campaign notification。

## Stage 4 — Cart Recall Journey

### Goal

購物車加入商品後未完成購買時，匹配活動並建立召回通知。

### Scope

- Cart event consumer。
- Delayed Trigger。
- 發送前 revalidation。
- Campaign matching 與 deterministic ranking。
- Order completed cancellation。
- Recall attribution。

### Exit Criteria

- 商品已移除、下架、售罄或已購買時不發送。
- 每個 trigger 最多匹配一個 Campaign。
- Trigger retry 不造成重複通知。
- 每個 skipped trigger 都具有 reason code。

## Stage 5 — Repurchase Service

### Goal

根據歷史訂單辨識可回購商品，在適當時間提供回購提醒與入口。

### Scope

- Repurchase policy。
- Order completed event consumer。
- Next eligible time scheduling。
- 商品可購買性與 Campaign eligibility revalidation。
- Repurchase notification 與 deep link。
- Repurchase conversion attribution。

### Exit Criteria

- 同一 order item 不會重複建立 active repurchase journey。
- 使用者已再次購買相同商品時取消舊 journey 並重新計算週期。
- 下架或無庫存商品不發送回購提醒。

## Stage 6 — Analytics and Operations

### Goal

讓營運人員理解 funnel、排除原因與 Campaign 成效。

### Scope

- Campaign exposure、notification、click、order funnel。
- Cart Recall 與 Repurchase 分開報表。
- Attribution Window。
- Campaign GMV 與 conversion count。
- Operational dashboard 與 failed task inspection。

### Exit Criteria

- Funnel 指標可按 Campaign 與 Journey type 查詢。
- 可查詢未觸達原因與 notification failures。
- Attribution 計算可重跑且不重複累加。
