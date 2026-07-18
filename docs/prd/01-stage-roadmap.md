# Stage Roadmap

## Proposal

每個 stage 都必須能獨立部署與驗收。後續 stage 只能依賴已完成 stage 的公開 contract，不得直接耦合內部 table。

MVP 範圍為 Stage 0 至 Stage 4。Stage 5 與 Stage 6 明確列為 post-MVP，不影響 Cart Recall MVP 驗收。

## Stage 0 — Commerce Prerequisite

### Status

大部分已完成。

### Remaining Work

- Member marketing consent 與 notification channel。
- Product category metadata 與活動標籤。
- Cart/Order Transactional Outbox。

### Exit Criteria

- 既有購買流程 integration test 通過。
- Cart/Order event 可被 idempotently 消費。

## Stage 1 — Campaign Service (MVP Simplified)

### Goal

營運人員可以管理活動，使用者可以查詢「此刻對自己有效」的活動。

### Scope

- Campaign CRUD、狀態與有效期間。
- Campaign priority。
- 以 Product ID 或 Category 定義活動商品範圍。
- Fixed amount 或 percentage benefit；percentage 可設定 maximum discount。
- Benefit Calculator 提供 deterministic discount result，checkout 仍為最終價格與資格的 source of truth。
- Public Campaign List/Detail API。
- Admin Campaign API。

### Exit Criteria

- Admin 可建立 Draft Campaign 並發布。
- 未開始、已結束、Paused、Archived 活動不出現在 public API。
- 使用者查詢只會看到通過 Rule Engine 的活動。
- 同一時間查詢結果依 priority 與 Campaign ID deterministic 排序。

## Stage 2 — Rule Engine (MVP Simplified)

### Goal

Campaign 可設定哪些商品或購物車符合資格，並提供一致、可稽核的 evaluate result。Member segmentation 僅保留必要欄位，不建立完整 audience engine。

### Scope

- 明確區分 Campaign Discovery 與 Cart Recall evaluation context。
- Product category/price、Cart total/item count 與必要 Member facts。
- AND/OR rule group。
- 精簡 allowlist operators。
- Rule validation、versioning 與 dry-run evaluate API。
- Eligibility decision log。
- Missing fact 統一視為 condition false，並記錄 `MISSING_FACT`。

### Exit Criteria

- Campaign publish 前 rule 必須通過 validation。
- 相同 facts 與 rule version 產生相同結果。
- Public Campaign API 套用相同 Rule Engine。
- Decision log 包含 matched rules 與第一個失敗原因。

## Stage 3 — Notification Pipeline (MVP Simplified)

### Goal

建立可靠、可重試且不重複的通知管線。Notification delivery 以獨立 worker/component 執行；MVP 實作真實的 Notification Task lifecycle，只 mock 外部 Push delivery provider。

### Scope

- Notification template。
- Notification Task。
- Notification Task consumer 與 delivery worker。
- Idempotency、retry、dead-letter state。
- In-app delivery 與 Mock Push provider；Mock provider 寫入 structured log 並更新 delivery status。
- Member consent 與 frequency cap。

### Exit Criteria

- 相同 idempotency key 只建立一個 task。
- 相同 Notification Task 可安全 retry，但只產生一次有效 delivery。
- 暫時性失敗可 retry，永久失敗不無限重試。
- 未授權行銷通知的會員不會收到 campaign notification。

## Stage 4 — Cart Recall Journey

### Goal

購物車加入商品後未完成購買時，匹配活動並建立召回通知。

### Scope

- Cart event consumer。
- Outbox publisher、queue 與 inbox-deduplicated consumer。
- 可獨立執行的 Cart Recall Trigger worker。
- Delayed Trigger。
- 發送前 revalidation。
- Campaign matching 與簡化 deterministic ranking。
- Order completed cancellation。
- Matched product snapshot。
- Sent-to-order conversion event log；不建立完整 attribution model。

### Exit Criteria

- 商品已移除、下架、售罄或已購買時不發送。
- 每個 trigger 最多匹配一個 Campaign。
- Trigger retry 不造成重複通知。
- Queue event 可重複投遞，但只產生一個 Journey 與 Notification Task。
- 每個 skipped trigger 都具有 reason code。
- 可由 Journey、Notification Task 與 structured log 查明未發送、已發送及是否轉換。

## Stage 5 — Repurchase Service (Post-MVP)

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

## Stage 6 — Analytics and Operations (Post-MVP)

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
