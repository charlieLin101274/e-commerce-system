package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type CartRecallStatus string

const (
	CartRecallScheduled           CartRecallStatus = "scheduled"
	CartRecallEvaluating          CartRecallStatus = "evaluating"
	CartRecallNotificationPending CartRecallStatus = "notification_pending"
	CartRecallSent                CartRecallStatus = "sent"
	CartRecallConverted           CartRecallStatus = "converted"
	CartRecallSkipped             CartRecallStatus = "skipped"
	CartRecallCancelled           CartRecallStatus = "cancelled"
)

type CartRecallProductSnapshot struct {
	ProductID        uuid.UUID     `json:"product_id"`
	Category         string        `json:"category"`
	UnitPrice        int64         `json:"unit_price"`
	Quantity         int64         `json:"quantity"`
	EvaluatedBenefit BenefitResult `json:"evaluated_benefit"`
}

type CartRecallJourney struct {
	ID                      uuid.UUID        `json:"id"`
	UserID                  uuid.UUID        `json:"user_id"`
	CartID                  uuid.UUID        `json:"cart_id"`
	SourceEventID           uuid.UUID        `json:"source_event_id"`
	Status                  CartRecallStatus `json:"status"`
	EvaluateAt              time.Time        `json:"evaluate_at"`
	CampaignID              *uuid.UUID       `json:"campaign_id,omitempty"`
	RuleVersion             int              `json:"rule_version,omitempty"`
	MatchedProductIDs       []uuid.UUID      `json:"matched_product_ids"`
	MatchedProductsSnapshot json.RawMessage  `json:"matched_products_snapshot,omitempty"`
	NotificationTaskID      *uuid.UUID       `json:"notification_task_id,omitempty"`
	ConvertedOrderID        *uuid.UUID       `json:"converted_order_id,omitempty"`
	CancelReason            string           `json:"cancel_reason,omitempty"`
	CreatedAt               time.Time        `json:"created_at"`
	UpdatedAt               time.Time        `json:"updated_at"`
}

type DomainEvent struct {
	ID          uuid.UUID       `json:"event_id"`
	Type        string          `json:"event_type"`
	AggregateID uuid.UUID       `json:"aggregate_id"`
	Payload     json.RawMessage `json:"payload"`
	OccurredAt  time.Time       `json:"occurred_at"`
}
