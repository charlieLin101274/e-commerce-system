# Growth and Operations

## Context

使用者把商品放進購物車，代表他對商品有明確興趣。如果使用者一段時間沒有完成購買，Cart Recall 可以提醒他回來，也可以顯示目前符合資格的活動。

本專案只有七天實作時間。MVP 的目標是完成一條可以實際操作與驗證的流程，不是建立完整的 Growth Platform。

## Goal

MVP 要做到：

- 營運人員可以建立、檢查、發布及暫停 Campaign。
- 購物車長時間沒有異動時，系統可以建立 Cart Recall Journey。
- 發送前再次確認購物車、商品、庫存、Campaign、consent 與 frequency cap。
- 重複事件或 worker retry 不會產生重複通知。
- 營運人員可以從 Journey、Notification Task 與 reason code 找到未發送原因。
- 使用者收到通知後完成訂單時，可以記錄 sent-to-order conversion。

MVP 不包含：

- 完整 A/B testing platform。
- Incremental conversion 或 incremental GMV 計算。
- 多觸點 attribution。
- Campaign 成本與預算管理。
- ML audience 或 send-time optimization。
- 完整營運 dashboard。

## Users

### Marketing Operator

負責建立 Campaign、設定活動時間、商品範圍、優惠與 eligibility rule。

### Customer

把商品加入購物車，並在符合條件時收到 Cart Recall notification。

### Customer Service or Operator

查詢 Journey 和 Notification Task，了解通知為何未發送、失敗或取消。

### Engineer or On-call

透過 structured logs、task status 與 reason code 排查 worker 或 delivery 問題。

## Operational Workflow

```text
建立 Draft Campaign
        |
        v
設定活動時間、商品範圍、優惠與 rule
        |
        v
執行 rule validation 與 dry-run
        |
        v
發布 Campaign
        |
        v
查看 Cart Recall Journey 與 Notification Task
        |
        +---- 發現設定有誤 ----> Pause Campaign
        |
        v
活動結束後查看 sent-to-order conversion
```

### Before Publishing

營運人員應確認：

- 活動開始與結束時間正確。
- 商品 ID 或 category scope 正確。
- 優惠金額與折扣上限正確。
- Rule validation 通過。
- 使用 dry-run 測試常見的 member、product 與 cart facts。

### During the Campaign

營運人員可以：

- 查看 Journey status。
- 查看 Notification Task status。
- 依照 reason code 找出未發送原因。
- 發現活動設定錯誤時立即 Pause Campaign。
- 對永久失敗且問題已排除的 task 執行人工 retry。

### After the Campaign

MVP 可以查看通知送出後是否完成相關商品訂單。這只能表示兩件事有時間上的關聯，不能直接證明訂單是通知帶來的。

## MVP KPI

MVP 先觀察系統是否可靠運作：

| KPI | 說明 |
| --- | --- |
| Journey evaluation success rate | 成功完成評估的 Journey 比例 |
| Notification delivery success rate | 成功送達的 Notification Task 比例 |
| Retry and failure rate | 需要 retry 或最後失敗的 task 比例 |
| Duplicate effective delivery count | 相同通知被有效送達超過一次的次數，目標為 0 |
| Invalid notification count | 商品、Campaign 或 consent 已失效仍送出的次數，目標為 0 |
| Sent-to-order conversion rate | 通知送出後 24 小時內完成相關商品訂單的比例 |

`Sent-to-order conversion rate` 是 observational metric，不等於 incremental conversion lift。

## Guardrails

MVP 必須遵守：

- 同一 Campaign 對同一 user，rolling 24 hours 最多一則通知。
- 同一 user，rolling 24 hours 最多兩則 marketing notifications。
- 使用者沒有 marketing consent 時不得建立或發送 task。
- 使用者沒有啟用對應 channel 時不得發送。
- Cart 已清空、商品下架、庫存不足或相關訂單已完成時不得發送。
- Campaign 已暫停或結束時不得發送。
- 相同 idempotency key 不得產生重複有效 delivery。

正式上線後還需要追蹤：

- Push opt-out rate。
- Complaint rate。
- App uninstall rate。
- Provider delivery cost。
- Campaign budget usage。

## Control Group and Experiment

Control group 不屬於七天 MVP，但會是判斷 Growth 效果的重要後續功能。

建議流程：

1. Journey 通過 eligibility check。
2. 使用 stable hash 將 Journey 固定分到 treatment 或 control。
3. Treatment 建立 Notification Task。
4. Control 不發送通知，但保留 assignment 與後續訂單結果。
5. 比較兩組在相同時間窗內的 conversion rate。

Stable assignment 必須確保 retry 後仍屬於同一組。可以保存：

```text
experiment_id
experiment_variant
assignment_key
assigned_at
```

初期 experiment 可使用 completed-order conversion rate 作為主要指標。正式判讀時還要一起檢查 opt-out、complaint 與通知成本等 guardrails。

## Future Growth KPI

加入 control group 後，才適合計算：

- Incremental conversion lift。
- Incremental orders。
- Incremental GMV。
- Revenue per notification。
- Cost per incremental order。

## MVP Acceptance Criteria

- Admin 可以建立、驗證、發布及暫停 Campaign。
- Cart mutation 可以建立或重新排程 Journey。
- 重複事件不會建立重複 active Journey。
- 不符合 guardrails 的 Journey 不會建立有效通知。
- Notification retry 不會造成重複有效 delivery。
- Order completed 可以取消 pending Journey，或記錄 sent-to-order conversion。
- Operator 可以查詢 Journey、Notification Task、status 與 reason code。
