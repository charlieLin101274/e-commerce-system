package campaign

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/base/apperror"
	"github.com/linenxing/e-commerce-system/models"
	campaignstore "github.com/linenxing/e-commerce-system/stores/campaign"
)

type fakeStore struct {
	campaigns          map[uuid.UUID]models.Campaign
	products           map[uuid.UUID]string
	productFactReads   int
	memberFactReads    int
	decisionLogWrites  int
	decisionLogError   error
	lastCandidateQuery campaignstore.CandidateQuery
	candidateQueries   []campaignstore.CandidateQuery
}

func (s *fakeStore) Create(_ context.Context, value models.Campaign) (models.Campaign, error) {
	value.ID = uuid.New()
	value.RuleVersion = 1
	s.campaigns[value.ID] = value
	return value, nil
}

func (s *fakeStore) Update(_ context.Context, value models.Campaign) (models.Campaign, error) {
	if _, exists := s.campaigns[value.ID]; !exists {
		return models.Campaign{}, campaignstore.ErrNotFound
	}
	s.campaigns[value.ID] = value
	return value, nil
}

func (s *fakeStore) GetByID(_ context.Context, id uuid.UUID) (models.Campaign, error) {
	value, exists := s.campaigns[id]
	if !exists {
		return models.Campaign{}, campaignstore.ErrNotFound
	}
	return value, nil
}

func (s *fakeStore) List(context.Context) ([]models.Campaign, error) {
	result := make([]models.Campaign, 0, len(s.campaigns))
	for _, value := range s.campaigns {
		result = append(result, value)
	}
	return result, nil
}

func (s *fakeStore) ListPublicCandidates(_ context.Context, query campaignstore.CandidateQuery) ([]models.Campaign, error) {
	s.lastCandidateQuery = query
	s.candidateQueries = append(s.candidateQueries, query)
	result := make([]models.Campaign, 0, len(s.campaigns))
	for _, value := range s.campaigns {
		if effectiveStatus(value, query.Now) != models.CampaignStatusRunning {
			continue
		}
		if query.ContextType != "" && value.RuleContextType != "" && value.RuleContextType != query.ContextType {
			continue
		}
		if query.ProductID != nil && !matchesScope(value, query.ProductID, query.Category) {
			continue
		}
		result = append(result, value)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority > result[j].Priority
		}
		return result[i].ID.String() < result[j].ID.String()
	})
	start := query.Offset
	if start > len(result) {
		start = len(result)
	}
	end := start + query.Limit
	if end > len(result) {
		end = len(result)
	}
	return result[start:end], nil
}

func TestMatchCartRecallScansNextCandidateBatch(t *testing.T) {
	now := time.Now()
	productID := uuid.New()
	campaigns := make(map[uuid.UUID]models.Campaign, 21)
	ineligibleRule := &models.RuleGroup{Operator: "and", Conditions: []models.RuleCondition{{Fact: "product.price", Operator: "gt", Value: []byte(`2000`)}}}
	var expectedID uuid.UUID
	for index := 0; index < 21; index++ {
		id := uuid.New()
		value := models.Campaign{ID: id, Status: models.CampaignStatusRunning, Priority: 100 - index, StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour), BenefitType: models.BenefitTypeFixedAmount, BenefitValue: 100, ProductIDs: []uuid.UUID{productID}, RuleContextType: models.EvaluationContextCartRecall, EligibilityRule: ineligibleRule}
		if index == 20 {
			value.EligibilityRule = nil
			expectedID = id
		}
		campaigns[id] = value
	}
	store := &fakeStore{products: map[uuid.UUID]string{productID: "electronics"}, campaigns: campaigns}
	service := &service{store: store, now: func() time.Time { return now }}
	matched, _, err := service.MatchCartRecall(context.Background(), models.EvaluationFacts{
		Member:  &models.MemberFacts{ID: uuid.New()},
		Product: &models.ProductFacts{ID: productID, Category: "electronics", Price: 1000, Status: models.ProductStatusActive},
		Cart:    &models.CartFacts{TotalPrice: 1000, ItemCount: 1},
	})
	if err != nil {
		t.Fatalf("match cart recall campaign: %v", err)
	}
	if matched.ID != expectedID {
		t.Fatalf("matched campaign = %s, want candidate from second batch %s", matched.ID, expectedID)
	}
	if len(store.candidateQueries) != 2 || store.candidateQueries[1].Offset != 20 {
		t.Fatalf("candidate queries = %#v, want second batch at offset 20", store.candidateQueries)
	}
}

func (s *fakeStore) GetProductCategories(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	result := make(map[uuid.UUID]string, len(ids))
	for _, id := range ids {
		if category, exists := s.products[id]; exists {
			result[id] = category
		}
	}
	return result, nil
}

func (s *fakeStore) GetProductFacts(_ context.Context, id uuid.UUID) (models.ProductFacts, error) {
	s.productFactReads++
	category, exists := s.products[id]
	if !exists {
		return models.ProductFacts{}, campaignstore.ErrNotFound
	}
	return models.ProductFacts{ID: id, Category: category, Price: 1000, Status: models.ProductStatusActive}, nil
}

func (s *fakeStore) GetMemberFacts(_ context.Context, id uuid.UUID) (models.MemberFacts, error) {
	s.memberFactReads++
	return models.MemberFacts{ID: id}, nil
}

func (s *fakeStore) CreateRuleVersion(_ context.Context, campaignID uuid.UUID, contextType models.EvaluationContextType, rule *models.RuleGroup) (int, error) {
	value := s.campaigns[campaignID]
	value.RuleVersion++
	value.RuleContextType = contextType
	value.EligibilityRule = rule
	s.campaigns[campaignID] = value
	return value.RuleVersion, nil
}

func (s *fakeStore) SaveDecisionLog(context.Context, campaignstore.DecisionLog) error {
	s.decisionLogWrites++
	return s.decisionLogError
}

func TestCalculateBenefit(t *testing.T) {
	maximum := int64(150)
	result, err := CalculateBenefit(models.BenefitTypePercentage, 20, &maximum, 1000)
	if err != nil {
		t.Fatalf("calculate percentage benefit: %v", err)
	}
	if result.DiscountAmount != 150 || result.FinalAmount != 850 {
		t.Fatalf("unexpected result: %+v", result)
	}

	result, err = CalculateBenefit(models.BenefitTypeFixedAmount, 1200, nil, 1000)
	if err != nil {
		t.Fatalf("calculate fixed benefit: %v", err)
	}
	if result.DiscountAmount != 1000 || result.FinalAmount != 0 {
		t.Fatalf("discount must be capped by amount: %+v", result)
	}
}

func TestPublicListFiltersAndRanksCampaigns(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	productID := uuid.New()
	highID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	lowLexicalID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	store := &fakeStore{
		products: map[uuid.UUID]string{productID: "electronics"},
		campaigns: map[uuid.UUID]models.Campaign{
			highID:       {ID: highID, Status: models.CampaignStatusScheduled, Priority: 10, StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour), Categories: []string{"electronics"}},
			lowLexicalID: {ID: lowLexicalID, Status: models.CampaignStatusRunning, Priority: 10, StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour), ProductIDs: []uuid.UUID{productID}},
			uuid.New():   {ID: uuid.New(), Status: models.CampaignStatusPaused, Priority: 100, StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour), Categories: []string{"electronics"}},
		},
	}
	service := &service{store: store, now: func() time.Time { return now }}
	result, err := service.ListPublic(context.Background(), &productID, nil, PageParam{})
	if err != nil {
		t.Fatalf("list public campaigns: %v", err)
	}
	if len(result) != 2 || result[0].ID != lowLexicalID || result[1].ID != highID {
		t.Fatalf("unexpected deterministic order: %+v", result)
	}
	if store.productFactReads != 1 {
		t.Fatalf("expected product facts to be loaded once, got %d", store.productFactReads)
	}
}

func TestPublicListDoesNotFailWhenDecisionLogWriteFails(t *testing.T) {
	now := time.Now()
	id, productID := uuid.New(), uuid.New()
	store := &fakeStore{decisionLogError: errors.New("log unavailable"), products: map[uuid.UUID]string{productID: "electronics"}, campaigns: map[uuid.UUID]models.Campaign{id: {ID: id, Status: models.CampaignStatusRunning, StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour), ProductIDs: []uuid.UUID{productID}}}}
	service := &service{store: store, now: func() time.Time { return now }}
	result, err := service.ListPublic(context.Background(), &productID, nil, PageParam{})
	if err != nil || len(result) != 1 {
		t.Fatalf("decision log failure must not fail public list: result=%v err=%v", result, err)
	}
}

func TestPublicListPassesBoundedPageToStore(t *testing.T) {
	now := time.Now()
	store := &fakeStore{products: map[uuid.UUID]string{}, campaigns: map[uuid.UUID]models.Campaign{}}
	service := &service{store: store, now: func() time.Time { return now }}
	if _, err := service.ListPublic(context.Background(), nil, nil, PageParam{Limit: 100, Offset: 7}); err != nil {
		t.Fatalf("list public campaigns: %v", err)
	}
	if store.lastCandidateQuery.Limit != 20 || store.lastCandidateQuery.Offset != 7 {
		t.Fatalf("candidate page = limit %d offset %d, want limit 20 offset 7", store.lastCandidateQuery.Limit, store.lastCandidateQuery.Offset)
	}
	if store.lastCandidateQuery.ContextType != models.EvaluationContextCampaignDiscovery {
		t.Fatalf("candidate context = %q", store.lastCandidateQuery.ContextType)
	}
}

func BenchmarkEvaluateRule(b *testing.B) {
	rule := &models.RuleGroup{Operator: "and", Conditions: []models.RuleCondition{
		{ID: "category", Fact: "product.category", Operator: "eq", Value: []byte(`"electronics"`)},
		{ID: "price", Fact: "product.price", Operator: "gte", Value: []byte(`500`)},
	}}
	facts := models.EvaluationFacts{Product: &models.ProductFacts{Category: "electronics", Price: 1000}}
	b.ResetTimer()
	for range b.N {
		evaluateRule(rule, facts)
	}
}

func TestAdminDryRunDoesNotWriteDecisionLog(t *testing.T) {
	id := uuid.New()
	store := &fakeStore{products: map[uuid.UUID]string{}, campaigns: map[uuid.UUID]models.Campaign{id: {ID: id, BenefitType: models.BenefitTypeFixedAmount, BenefitValue: 100, RuleContextType: models.EvaluationContextCampaignDiscovery}}}
	service := &service{store: store, now: time.Now}
	_, err := service.EvaluateRules(context.Background(), id, models.EvaluationContextCampaignDiscovery, models.EvaluationFacts{})
	if err != nil {
		t.Fatalf("dry-run evaluation: %v", err)
	}
	if store.decisionLogWrites != 0 {
		t.Fatalf("dry-run must not write decision logs, got %d", store.decisionLogWrites)
	}
}

func TestPublishRejectsMissingProduct(t *testing.T) {
	now := time.Now()
	id := uuid.New()
	store := &fakeStore{products: map[uuid.UUID]string{}, campaigns: map[uuid.UUID]models.Campaign{id: {ID: id, Status: models.CampaignStatusDraft, StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour), ProductIDs: []uuid.UUID{uuid.New()}}}}
	service := &service{store: store, now: func() time.Time { return now }}
	_, err := service.Publish(context.Background(), id)
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestCreateRequiresProductScope(t *testing.T) {
	store := &fakeStore{products: map[uuid.UUID]string{}, campaigns: map[uuid.UUID]models.Campaign{}}
	service := &service{store: store, now: time.Now}
	_, err := service.Create(context.Background(), uuid.New(), WriteParam{Name: "Campaign", PromotionTitle: "Promotion", StartsAt: time.Now(), EndsAt: time.Now().Add(time.Hour), BenefitType: models.BenefitTypeFixedAmount, BenefitValue: 100})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestPausedCampaignEndsAtBoundary(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	value := models.Campaign{Status: models.CampaignStatusPaused, StartsAt: now.Add(-time.Hour), EndsAt: now}
	if got := effectiveStatus(value, now); got != models.CampaignStatusEnded {
		t.Fatalf("expected ended status, got %s", got)
	}
}

func TestResumeUsesCurrentCampaignWindow(t *testing.T) {
	now := time.Date(2026, 7, 18, 12, 0, 0, 0, time.UTC)
	id := uuid.New()
	store := &fakeStore{products: map[uuid.UUID]string{}, campaigns: map[uuid.UUID]models.Campaign{id: {ID: id, Status: models.CampaignStatusPaused, StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour)}}}
	service := &service{store: store, now: func() time.Time { return now }}
	result, err := service.Resume(context.Background(), id)
	if err != nil {
		t.Fatalf("resume campaign: %v", err)
	}
	if result.Status != models.CampaignStatusScheduled {
		t.Fatalf("expected scheduled status, got %s", result.Status)
	}
}

func TestArchiveAllowsDraft(t *testing.T) {
	id := uuid.New()
	store := &fakeStore{products: map[uuid.UUID]string{}, campaigns: map[uuid.UUID]models.Campaign{id: {ID: id, Status: models.CampaignStatusDraft}}}
	service := &service{store: store, now: time.Now}
	result, err := service.Archive(context.Background(), id)
	if err != nil {
		t.Fatalf("archive draft campaign: %v", err)
	}
	if result.Status != models.CampaignStatusArchived {
		t.Fatalf("expected archived status, got %s", result.Status)
	}
}

func TestCategoryNormalization(t *testing.T) {
	result := uniqueCategories([]string{" Electronics ", "electronics", "HOME"})
	if len(result) != 2 || result[0] != "electronics" || result[1] != "home" {
		t.Fatalf("unexpected normalized categories: %#v", result)
	}
}
