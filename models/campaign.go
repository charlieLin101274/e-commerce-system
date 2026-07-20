package models

import (
	"encoding/json"
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
	ID                    uuid.UUID             `json:"id"`
	Name                  string                `json:"name"`
	Description           string                `json:"description"`
	Status                CampaignStatus        `json:"status"`
	Priority              int                   `json:"priority"`
	StartsAt              time.Time             `json:"starts_at"`
	EndsAt                time.Time             `json:"ends_at"`
	PromotionTitle        string                `json:"promotion_title"`
	PromotionDescription  string                `json:"promotion_description"`
	BenefitType           BenefitType           `json:"benefit_type"`
	BenefitValue          int64                 `json:"benefit_value"`
	MaximumDiscountAmount *int64                `json:"maximum_discount_amount,omitempty"`
	ProductIDs            []uuid.UUID           `json:"product_ids"`
	Categories            []string              `json:"categories"`
	CreatedBy             uuid.UUID             `json:"created_by"`
	CreatedAt             time.Time             `json:"created_at"`
	UpdatedAt             time.Time             `json:"updated_at"`
	PublishedAt           *time.Time            `json:"published_at,omitempty"`
	RuleVersion           int                   `json:"-"`
	RuleContextType       EvaluationContextType `json:"-"`
	EligibilityRule       *RuleGroup            `json:"-"`
}

type EvaluationContextType string

const (
	EvaluationContextCampaignDiscovery EvaluationContextType = "campaign_discovery"
	EvaluationContextCartRecall        EvaluationContextType = "cart_recall"
)

type RuleGroup struct {
	Operator   string          `json:"operator"`
	Conditions []RuleCondition `json:"conditions,omitempty"`
	Groups     []RuleGroup     `json:"groups,omitempty"`
}

type RuleCondition struct {
	ID       string          `json:"id,omitempty"`
	Fact     string          `json:"fact"`
	Operator string          `json:"operator"`
	Value    json.RawMessage `json:"value"`
}

type EvaluationFacts struct {
	Member  *MemberFacts  `json:"member,omitempty"`
	Product *ProductFacts `json:"product,omitempty"`
	Cart    *CartFacts    `json:"cart,omitempty"`
}

type MemberFacts struct {
	ID    uuid.UUID `json:"id"`
	Level string    `json:"level"`
	Tags  []string  `json:"tags"`
}

type ProductFacts struct {
	ID       uuid.UUID     `json:"id"`
	Category string        `json:"category"`
	Price    int64         `json:"price"`
	Status   ProductStatus `json:"status"`
}

type CartFacts struct {
	TotalPrice int64 `json:"total_price"`
	ItemCount  int64 `json:"item_count"`
}

type ConditionDecision struct {
	ConditionID string `json:"condition_id"`
	Matched     bool   `json:"matched"`
	ReasonCode  string `json:"reason_code,omitempty"`
	MissingFact string `json:"missing_fact,omitempty"`
}

type EvaluationResult struct {
	Eligible           bool                `json:"eligible"`
	CampaignID         uuid.UUID           `json:"campaign_id"`
	RuleVersion        int                 `json:"rule_version"`
	ReasonCode         string              `json:"reason_code"`
	EvaluatedAt        time.Time           `json:"evaluated_at"`
	BenefitPreview     *BenefitResult      `json:"benefit_preview,omitempty"`
	ConditionDecisions []ConditionDecision `json:"condition_decisions,omitempty"`
	MissingFacts       []string            `json:"missing_facts,omitempty"`
	ValidationErrors   []string            `json:"validation_errors,omitempty"`
}

type BenefitResult struct {
	OriginalAmount int64 `json:"original_amount"`
	DiscountAmount int64 `json:"discount_amount"`
	FinalAmount    int64 `json:"final_amount"`
}
