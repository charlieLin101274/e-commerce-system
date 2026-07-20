package campaign

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

type WriteParam struct {
	Name                  string
	Description           string
	Priority              int
	StartsAt              time.Time
	EndsAt                time.Time
	PromotionTitle        string
	PromotionDescription  string
	BenefitType           models.BenefitType
	BenefitValue          int64
	MaximumDiscountAmount *int64
	ProductIDs            []uuid.UUID
	Categories            []string
	ContextType           models.EvaluationContextType
	EligibilityRule       *models.RuleGroup
}

type Service interface {
	Create(context.Context, uuid.UUID, WriteParam) (models.Campaign, error)
	Update(context.Context, uuid.UUID, WriteParam) (models.Campaign, error)
	Publish(context.Context, uuid.UUID) (models.Campaign, error)
	Pause(context.Context, uuid.UUID) (models.Campaign, error)
	Resume(context.Context, uuid.UUID) (models.Campaign, error)
	Archive(context.Context, uuid.UUID) (models.Campaign, error)
	ListAdmin(context.Context) ([]models.Campaign, error)
	GetAdmin(context.Context, uuid.UUID) (models.Campaign, error)
	ListPublic(context.Context, *uuid.UUID, *uuid.UUID) ([]models.Campaign, error)
	GetPublic(context.Context, uuid.UUID, *uuid.UUID, *uuid.UUID) (models.Campaign, error)
	EvaluatePublic(context.Context, uuid.UUID, *uuid.UUID, *uuid.UUID) (models.EvaluationResult, error)
	ValidateRules(context.Context, uuid.UUID) ([]string, error)
	EvaluateRules(context.Context, uuid.UUID, models.EvaluationContextType, models.EvaluationFacts) (models.EvaluationResult, error)
	MatchCartRecall(context.Context, models.EvaluationFacts) (models.Campaign, models.EvaluationResult, error)
}
