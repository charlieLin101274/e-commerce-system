package models

import (
	"time"

	"github.com/google/uuid"
)

type CampaignStatus string

const (
	CampaignStatusDraft     CampaignStatus = "draft"
	CampaignStatusScheduled CampaignStatus = "scheduled"
	CampaignStatusRunning   CampaignStatus = "running"
	CampaignStatusPaused    CampaignStatus = "paused"
	CampaignStatusEnded     CampaignStatus = "ended"
	CampaignStatusArchived  CampaignStatus = "archived"
)

type BenefitType string

const (
	BenefitTypeFixedAmount BenefitType = "fixed_amount"
	BenefitTypePercentage  BenefitType = "percentage"
)

type Campaign struct {
	ID                    uuid.UUID      `json:"id"`
	Name                  string         `json:"name"`
	Description           string         `json:"description"`
	Status                CampaignStatus `json:"status"`
	Priority              int            `json:"priority"`
	StartsAt              time.Time      `json:"starts_at"`
	EndsAt                time.Time      `json:"ends_at"`
	PromotionTitle        string         `json:"promotion_title"`
	PromotionDescription  string         `json:"promotion_description"`
	BenefitType           BenefitType    `json:"benefit_type"`
	BenefitValue          int64          `json:"benefit_value"`
	MaximumDiscountAmount *int64         `json:"maximum_discount_amount,omitempty"`
	ProductIDs            []uuid.UUID    `json:"product_ids"`
	Categories            []string       `json:"categories"`
	CreatedBy             uuid.UUID      `json:"created_by"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	PublishedAt           *time.Time     `json:"published_at,omitempty"`
}

type BenefitResult struct {
	OriginalAmount int64 `json:"original_amount"`
	DiscountAmount int64 `json:"discount_amount"`
	FinalAmount    int64 `json:"final_amount"`
}
