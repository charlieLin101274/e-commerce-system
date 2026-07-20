package campaign

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

var ErrNotFound = errors.New("campaign not found")
var ErrConflict = errors.New("campaign was concurrently modified")

type Store interface {
	Create(context.Context, models.Campaign) (models.Campaign, error)
	Update(context.Context, models.Campaign) (models.Campaign, error)
	GetByID(context.Context, uuid.UUID) (models.Campaign, error)
	List(context.Context) ([]models.Campaign, error)
	GetProductCategories(context.Context, []uuid.UUID) (map[uuid.UUID]string, error)
	GetProductFacts(context.Context, uuid.UUID) (models.ProductFacts, error)
	GetMemberFacts(context.Context, uuid.UUID) (models.MemberFacts, error)
	CreateRuleVersion(context.Context, uuid.UUID, models.EvaluationContextType, *models.RuleGroup) (int, error)
	SaveDecisionLog(context.Context, DecisionLog) error
}

type DecisionLog struct {
	CampaignID           uuid.UUID
	RuleVersion          int
	ContextType          models.EvaluationContextType
	Eligible             bool
	ReasonCode           string
	Facts                models.EvaluationFacts
	MatchedConditionIDs  []string
	FailedConditionID    string
	MissingFacts         []string
	DurationMicroseconds int64
	EvaluatedAt          time.Time
}
