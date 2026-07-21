package campaign

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/base/apperror"
	"github.com/linenxing/e-commerce-system/base/logger"
	"github.com/linenxing/e-commerce-system/models"
	campaignstore "github.com/linenxing/e-commerce-system/stores/campaign"
)

type service struct {
	store campaignstore.Store
	now   func() time.Time
}

func New(store campaignstore.Store) Service { return &service{store: store, now: time.Now} }

func (s *service) Create(ctx context.Context, createdBy uuid.UUID, param WriteParam) (models.Campaign, error) {
	if param.ContextType == "" {
		param.ContextType = models.EvaluationContextCampaignDiscovery
	}
	normalizeRule(param.EligibilityRule)
	if len(ValidateRule(param.EligibilityRule, param.ContextType)) > 0 {
		return models.Campaign{}, apperror.ErrInvalidInput
	}
	value, err := buildCampaign(param)
	if err != nil {
		return models.Campaign{}, err
	}
	value.Status = models.CampaignStatusDraft
	value.CreatedBy = createdBy
	value.RuleContextType = param.ContextType
	value.EligibilityRule = param.EligibilityRule
	value, err = s.store.Create(ctx, value)
	if err != nil {
		return value, err
	}
	return s.get(ctx, value.ID)
}

func (s *service) Update(ctx context.Context, id uuid.UUID, param WriteParam) (models.Campaign, error) {
	current, err := s.get(ctx, id)
	if err != nil {
		return models.Campaign{}, err
	}
	if current.Status != models.CampaignStatusDraft {
		return models.Campaign{}, apperror.ErrConflict
	}
	if param.ContextType == "" {
		param.ContextType = models.EvaluationContextCampaignDiscovery
	}
	normalizeRule(param.EligibilityRule)
	if len(ValidateRule(param.EligibilityRule, param.ContextType)) > 0 {
		return models.Campaign{}, apperror.ErrInvalidInput
	}
	value, err := buildCampaign(param)
	if err != nil {
		return models.Campaign{}, err
	}
	value.ID, value.Status, value.CreatedBy = current.ID, current.Status, current.CreatedBy
	value.CreatedAt, value.UpdatedAt, value.PublishedAt = current.CreatedAt, current.UpdatedAt, current.PublishedAt
	value, err = s.update(ctx, value)
	if err != nil {
		return value, err
	}
	if param.EligibilityRule == nil {
		return s.get(ctx, value.ID)
	}
	if _, err = s.store.CreateRuleVersion(ctx, value.ID, param.ContextType, param.EligibilityRule); err != nil {
		return models.Campaign{}, err
	}
	return s.get(ctx, value.ID)
}

func buildCampaign(param WriteParam) (models.Campaign, error) {
	param.Name = strings.TrimSpace(param.Name)
	param.PromotionTitle = strings.TrimSpace(param.PromotionTitle)
	if param.Name == "" || param.PromotionTitle == "" || !param.StartsAt.Before(param.EndsAt) || param.BenefitValue <= 0 {
		return models.Campaign{}, apperror.ErrInvalidInput
	}
	if param.BenefitType == models.BenefitTypePercentage {
		if param.BenefitValue > 100 || (param.MaximumDiscountAmount != nil && *param.MaximumDiscountAmount <= 0) {
			return models.Campaign{}, apperror.ErrInvalidInput
		}
	} else if param.BenefitType != models.BenefitTypeFixedAmount || param.MaximumDiscountAmount != nil {
		return models.Campaign{}, apperror.ErrInvalidInput
	}
	productIDs := uniqueProductIDs(param.ProductIDs)
	categories := uniqueCategories(param.Categories)
	for _, category := range categories {
		if utf8.RuneCountInString(category) > 100 {
			return models.Campaign{}, apperror.ErrInvalidInput
		}
	}
	if len(productIDs) == 0 && len(categories) == 0 {
		return models.Campaign{}, apperror.ErrInvalidInput
	}
	return models.Campaign{Name: param.Name, Description: strings.TrimSpace(param.Description), Priority: param.Priority, StartsAt: param.StartsAt, EndsAt: param.EndsAt, PromotionTitle: param.PromotionTitle, PromotionDescription: strings.TrimSpace(param.PromotionDescription), BenefitType: param.BenefitType, BenefitValue: param.BenefitValue, MaximumDiscountAmount: param.MaximumDiscountAmount, ProductIDs: productIDs, Categories: categories}, nil
}

func (s *service) Publish(ctx context.Context, id uuid.UUID) (models.Campaign, error) {
	value, err := s.get(ctx, id)
	if err != nil {
		return models.Campaign{}, err
	}
	if value.Status != models.CampaignStatusDraft || !s.now().Before(value.EndsAt) {
		return models.Campaign{}, apperror.ErrConflict
	}
	if validationErrors := ValidateRule(value.EligibilityRule, value.RuleContextType); len(validationErrors) > 0 {
		return models.Campaign{}, apperror.ErrInvalidInput
	}
	products, err := s.store.GetProductCategories(ctx, value.ProductIDs)
	if err != nil {
		return models.Campaign{}, err
	}
	if len(products) != len(value.ProductIDs) {
		return models.Campaign{}, apperror.ErrInvalidInput
	}
	now := s.now()
	value.PublishedAt = &now
	value.Status = models.CampaignStatusScheduled
	if !now.Before(value.StartsAt) {
		value.Status = models.CampaignStatusRunning
	}
	return s.update(ctx, value)
}

func (s *service) Pause(ctx context.Context, id uuid.UUID) (models.Campaign, error) {
	value, err := s.get(ctx, id)
	if err != nil {
		return models.Campaign{}, err
	}
	effective := effectiveStatus(value, s.now())
	if effective != models.CampaignStatusRunning && effective != models.CampaignStatusScheduled {
		return models.Campaign{}, apperror.ErrConflict
	}
	value.Status = models.CampaignStatusPaused
	return s.update(ctx, value)
}

func (s *service) Resume(ctx context.Context, id uuid.UUID) (models.Campaign, error) {
	value, err := s.get(ctx, id)
	if err != nil {
		return models.Campaign{}, err
	}
	now := s.now()
	if value.Status != models.CampaignStatusPaused || !now.Before(value.EndsAt) {
		return models.Campaign{}, apperror.ErrConflict
	}
	value.Status = models.CampaignStatusRunning
	if now.Before(value.StartsAt) {
		value.Status = models.CampaignStatusScheduled
	}
	return s.update(ctx, value)
}

func (s *service) Archive(ctx context.Context, id uuid.UUID) (models.Campaign, error) {
	value, err := s.get(ctx, id)
	if err != nil {
		return models.Campaign{}, err
	}
	effective := effectiveStatus(value, s.now())
	if effective != models.CampaignStatusDraft && effective != models.CampaignStatusEnded && effective != models.CampaignStatusPaused {
		return models.Campaign{}, apperror.ErrConflict
	}
	value.Status = models.CampaignStatusArchived
	return s.update(ctx, value)
}

func (s *service) ListAdmin(ctx context.Context) ([]models.Campaign, error) {
	items, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	for index := range items {
		items[index].Status = effectiveStatus(items[index], s.now())
	}
	return items, nil
}

func (s *service) GetAdmin(ctx context.Context, id uuid.UUID) (models.Campaign, error) {
	value, err := s.get(ctx, id)
	value.Status = effectiveStatus(value, s.now())
	return value, err
}

func (s *service) ListPublic(ctx context.Context, productID, userID *uuid.UUID, page PageParam) ([]models.Campaign, error) {
	page = normalizePage(page)
	facts, err := s.discoveryFacts(ctx, productID, userID)
	if err != nil {
		return nil, err
	}
	category := discoveryCategory(facts)
	items, err := s.store.ListPublicCandidates(ctx, campaignstore.CandidateQuery{Now: s.now(), ProductID: productID, Category: category, ContextType: models.EvaluationContextCampaignDiscovery, Limit: page.Limit, Offset: page.Offset})
	if err != nil {
		return nil, err
	}
	result := make([]models.Campaign, 0)
	for _, value := range items {
		decision, evaluationErr := s.evaluate(ctx, value, models.EvaluationContextCampaignDiscovery, facts, false, true)
		if evaluationErr != nil {
			return nil, evaluationErr
		}
		if !decision.Eligible {
			continue
		}
		value.Status = models.CampaignStatusRunning
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority > result[j].Priority
		}
		return result[i].ID.String() < result[j].ID.String()
	})
	return result, nil
}

func normalizePage(page PageParam) PageParam {
	const defaultLimit = 20
	const maximumLimit = 20
	if page.Limit <= 0 {
		page.Limit = defaultLimit
	}
	if page.Limit > maximumLimit {
		page.Limit = maximumLimit
	}
	if page.Offset < 0 {
		page.Offset = 0
	}
	return page
}

func (s *service) GetPublic(ctx context.Context, id uuid.UUID, productID, userID *uuid.UUID) (models.Campaign, error) {
	value, err := s.get(ctx, id)
	if err != nil || effectiveStatus(value, s.now()) != models.CampaignStatusRunning {
		return models.Campaign{}, apperror.ErrNotFound
	}
	facts, err := s.discoveryFacts(ctx, productID, userID)
	if err != nil {
		return models.Campaign{}, err
	}
	category := discoveryCategory(facts)
	if !matchesScope(value, productID, category) {
		return models.Campaign{}, apperror.ErrNotFound
	}
	decision, err := s.evaluate(ctx, value, models.EvaluationContextCampaignDiscovery, facts, false, true)
	if err != nil {
		return models.Campaign{}, err
	}
	if !decision.Eligible {
		return models.Campaign{}, apperror.ErrNotFound
	}
	value.Status = models.CampaignStatusRunning
	return value, nil
}

func (s *service) EvaluatePublic(ctx context.Context, id uuid.UUID, productID, userID *uuid.UUID) (models.EvaluationResult, error) {
	value, err := s.get(ctx, id)
	if err != nil || effectiveStatus(value, s.now()) != models.CampaignStatusRunning {
		return models.EvaluationResult{}, apperror.ErrNotFound
	}
	facts, err := s.discoveryFacts(ctx, productID, userID)
	if err != nil {
		return models.EvaluationResult{}, err
	}
	category := discoveryCategory(facts)
	if !matchesScope(value, productID, category) {
		return publicDecision(value, false, "NOT_ELIGIBLE", s.now()), nil
	}
	result, err := s.evaluate(ctx, value, models.EvaluationContextCampaignDiscovery, facts, true, true)
	if err == nil {
		result.ConditionDecisions = nil
		result.MissingFacts = nil
		if !result.Eligible {
			result.ReasonCode = "NOT_ELIGIBLE"
		}
	}
	return result, err
}

func (s *service) ValidateRules(ctx context.Context, id uuid.UUID) ([]string, error) {
	value, err := s.get(ctx, id)
	if err != nil {
		return nil, err
	}
	return ValidateRule(value.EligibilityRule, value.RuleContextType), nil
}

func (s *service) EvaluateRules(ctx context.Context, id uuid.UUID, contextType models.EvaluationContextType, facts models.EvaluationFacts) (models.EvaluationResult, error) {
	value, err := s.get(ctx, id)
	if err != nil {
		return models.EvaluationResult{}, err
	}
	errors := ValidateRule(value.EligibilityRule, contextType)
	if len(errors) > 0 {
		return models.EvaluationResult{CampaignID: value.ID, RuleVersion: value.RuleVersion, ReasonCode: "INVALID_RULE", EvaluatedAt: s.now(), ValidationErrors: errors}, nil
	}
	return s.evaluate(ctx, value, contextType, facts, true, false)
}

// MatchCartRecall evaluates running cart-recall campaigns in deterministic rank order.
func (s *service) MatchCartRecall(ctx context.Context, facts models.EvaluationFacts) (models.Campaign, models.EvaluationResult, error) {
	if facts.Member == nil || facts.Product == nil || facts.Cart == nil {
		return models.Campaign{}, models.EvaluationResult{}, apperror.ErrInvalidInput
	}
	facts.Product.Category = normalizeCategory(facts.Product.Category)
	items, err := s.store.ListPublicCandidates(ctx, campaignstore.CandidateQuery{Now: s.now(), ProductID: &facts.Product.ID, Category: facts.Product.Category, ContextType: models.EvaluationContextCartRecall, Limit: 20})
	if err != nil {
		return models.Campaign{}, models.EvaluationResult{}, err
	}
	for _, value := range items {
		decision, evaluationErr := s.evaluate(ctx, value, models.EvaluationContextCartRecall, facts, true, true)
		if evaluationErr != nil {
			return models.Campaign{}, models.EvaluationResult{}, evaluationErr
		}
		if decision.Eligible {
			value.Status = models.CampaignStatusRunning
			return value, decision, nil
		}
	}
	return models.Campaign{}, models.EvaluationResult{}, apperror.ErrNotFound
}

func (s *service) discoveryFacts(ctx context.Context, productID, userID *uuid.UUID) (models.EvaluationFacts, error) {
	var facts models.EvaluationFacts
	if productID != nil {
		product, err := s.store.GetProductFacts(ctx, *productID)
		if errors.Is(err, campaignstore.ErrNotFound) {
			return facts, apperror.ErrNotFound
		}
		if err != nil {
			return facts, err
		}
		product.Category = normalizeCategory(product.Category)
		if product.Status != models.ProductStatusActive {
			return facts, apperror.ErrNotFound
		}
		facts.Product = &product
	}
	if userID != nil {
		member, err := s.store.GetMemberFacts(ctx, *userID)
		if errors.Is(err, campaignstore.ErrNotFound) {
			return facts, apperror.ErrNotFound
		}
		if err != nil {
			return facts, err
		}
		facts.Member = &member
	}
	return facts, nil
}

func discoveryCategory(facts models.EvaluationFacts) string {
	if facts.Product == nil {
		return ""
	}
	return facts.Product.Category
}

func (s *service) evaluate(ctx context.Context, value models.Campaign, contextType models.EvaluationContextType, facts models.EvaluationFacts, includeDetail, persistDecision bool) (models.EvaluationResult, error) {
	startedAt := time.Now()
	eligible, decisions := evaluateRule(value.EligibilityRule, facts)
	failedID, internalReason := firstFailure(decisions)
	reason := "ELIGIBLE"
	if !eligible {
		reason = internalReason
	} else {
		failedID = ""
	}
	result := models.EvaluationResult{Eligible: eligible, CampaignID: value.ID, RuleVersion: value.RuleVersion, ReasonCode: reason, EvaluatedAt: s.now()}
	if eligible && facts.Product != nil && includeDetail {
		preview, err := CalculateBenefit(value.BenefitType, value.BenefitValue, value.MaximumDiscountAmount, facts.Product.Price)
		if err != nil {
			return models.EvaluationResult{}, err
		}
		result.BenefitPreview = &preview
	}
	missing := missingFacts(decisions)
	matched := make([]string, 0)
	for _, decision := range decisions {
		if decision.Matched {
			matched = append(matched, decision.ConditionID)
		}
	}
	if persistDecision {
		logValue := campaignstore.DecisionLog{CampaignID: value.ID, RuleVersion: value.RuleVersion, ContextType: contextType, Eligible: eligible, ReasonCode: reason, Facts: facts, MatchedConditionIDs: matched, FailedConditionID: failedID, MissingFacts: missing, DurationMicroseconds: time.Since(startedAt).Microseconds(), EvaluatedAt: result.EvaluatedAt}
		if err := s.store.SaveDecisionLog(ctx, logValue); err != nil {
			logger.FromContext(ctx).Error().Err(err).Str("campaign_id", value.ID.String()).Int("rule_version", value.RuleVersion).Msg("failed to save campaign decision log")
		}
	}
	if includeDetail {
		result.ConditionDecisions, result.MissingFacts = decisions, missing
	}
	return result, nil
}

func publicDecision(value models.Campaign, eligible bool, reason string, now time.Time) models.EvaluationResult {
	return models.EvaluationResult{Eligible: eligible, CampaignID: value.ID, RuleVersion: value.RuleVersion, ReasonCode: reason, EvaluatedAt: now}
}

func (s *service) get(ctx context.Context, id uuid.UUID) (models.Campaign, error) {
	value, err := s.store.GetByID(ctx, id)
	if errors.Is(err, campaignstore.ErrNotFound) {
		return models.Campaign{}, apperror.ErrNotFound
	}
	return value, err
}

func (s *service) update(ctx context.Context, value models.Campaign) (models.Campaign, error) {
	updated, err := s.store.Update(ctx, value)
	if errors.Is(err, campaignstore.ErrConflict) {
		return models.Campaign{}, apperror.ErrConflict
	}
	if errors.Is(err, campaignstore.ErrNotFound) {
		return models.Campaign{}, apperror.ErrNotFound
	}
	return updated, err
}

func effectiveStatus(value models.Campaign, now time.Time) models.CampaignStatus {
	if value.Status == models.CampaignStatusScheduled || value.Status == models.CampaignStatusRunning || value.Status == models.CampaignStatusPaused {
		if !now.Before(value.EndsAt) {
			return models.CampaignStatusEnded
		}
		if value.Status == models.CampaignStatusPaused {
			return models.CampaignStatusPaused
		}
		if now.Before(value.StartsAt) {
			return models.CampaignStatusScheduled
		}
		return models.CampaignStatusRunning
	}
	return value.Status
}

func matchesScope(value models.Campaign, productID *uuid.UUID, category string) bool {
	if productID == nil {
		return true
	}
	for _, id := range value.ProductIDs {
		if id == *productID {
			return true
		}
	}
	for _, item := range value.Categories {
		if item == category {
			return true
		}
	}
	return false
}

func uniqueProductIDs(values []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(values))
	result := make([]uuid.UUID, 0, len(values))
	for _, value := range values {
		if value == uuid.Nil {
			continue
		}
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func uniqueCategories(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = normalizeCategory(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; !exists {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}

func normalizeCategory(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
