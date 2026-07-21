# Activity-driven Commerce Platform PRD

## Context

本文件集描述建立於既有電商 MVP 之上的活動平台。本專案的實作時間為七天，重點是完成 Campaign、Rule Engine、Notification 與 Cart Recall 的完整流程。Repurchase、完整 Attribution、experiment 與 Analytics 為 post-MVP。

基礎會員、商品、購物車與訂單流程視為 prerequisite，不重複建立另一套 commerce domain。新功能必須透過既有 service、store 與 PostgreSQL schema 演進。

## Product Goal

平台需逐步提供以下能力：

1. 營運人員建立活動，使用者查詢目前可參與的活動。
2. 使用 Rule Engine 決定哪些使用者與商品符合 Campaign。
3. 根據購物車與訂單事件建立 Notification Task 並安全送達。
4. 對已展現特定商品購買意圖、但尚未完成下單的使用者進行 Cart Recall，重新喚起購買需求並促成轉換。
5. 以 event log 追蹤 Cart Recall 的判斷、通知與轉換結果。
6. Post-MVP 再提供 Repurchase、完整 attribution 與 Analytics。

## Documents

| 文件 | 說明 |
| --- | --- |
| [00-prerequisites.md](00-prerequisites.md) | 現有能力、缺口與系統邊界 |
| [01-stage-roadmap.md](01-stage-roadmap.md) | 分階段交付順序與驗收標準 |
| [02-campaign-and-rule-engine.md](02-campaign-and-rule-engine.md) | Campaign 與受眾 Rule Engine |
| [03-notification-and-cart-recall.md](03-notification-and-cart-recall.md) | Notification、事件與 Cart Recall |
| [04-repurchase-and-attribution.md](04-repurchase-and-attribution.md) | Repurchase 與成效歸因 |
| [05-non-functional-requirements.md](05-non-functional-requirements.md) | 一致性、安全性與 observability |
| [06-growth-and-operations.md](06-growth-and-operations.md) | 營運流程、KPI、guardrails 與後續 experiment 設計 |
| [07-production-evolution.md](07-production-evolution.md) | Queue、distributed tracing、context logging 與 error handling 的後續設計 |

## Guiding Principles

- Campaign 是商業活動，Rule 是活動資格，Trigger 是啟動評估的事件，Notification 是觸達結果。
- Cart Recall 是針對具明確商品購買意圖使用者的 conversion journey，不是一般活動通知或廣泛行銷觸達。
- Rule Engine 只判斷 eligibility；Benefit Calculator 根據已驗證的 benefit configuration 計算折扣，兩者不得混合成任意 expression engine。
- Cart Abandonment 與 Repurchase 是不同 Journey，不應共用同一套狀態機。
- 使用者看到的活動必須通過即時資格判斷，不能只依賴離線 audience snapshot。
- Notification retry 必須具備 idempotency，不得重複觸達。
- 系統不因棄單或回購機會自行建立額外折扣。
- Checkout 仍是價格、庫存與 Promotion 最終資格的 source of truth。
- 所有 eligible、ineligible、matched、skipped 決策都必須可追蹤。

## Out of Scope

- 金流、物流、退款與發票。
- 完整 Promotion 計價與優惠疊加引擎。
- ML-based recommendation、最佳發送時間預測與 conversion scoring。
- Email、SMS、LINE 等外部通道。
- 完整活動審核流程與企業級 RBAC。
