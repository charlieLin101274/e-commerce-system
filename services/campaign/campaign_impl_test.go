package campaign

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/base/apperror"
	"github.com/linenxing/e-commerce-system/models"
	campaignstore "github.com/linenxing/e-commerce-system/stores/campaign"
)

type fakeStore struct {
	campaigns map[uuid.UUID]models.Campaign
	products  map[uuid.UUID]string
}

func (s *fakeStore) Create(_ context.Context, value models.Campaign) (models.Campaign, error) {
	value.ID = uuid.New()
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

func (s *fakeStore) GetProductCategories(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	result := make(map[uuid.UUID]string, len(ids))
	for _, id := range ids {
		if category, exists := s.products[id]; exists {
			result[id] = category
		}
	}
	return result, nil
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
	result, err := service.ListPublic(context.Background(), &productID)
	if err != nil {
		t.Fatalf("list public campaigns: %v", err)
	}
	if len(result) != 2 || result[0].ID != lowLexicalID || result[1].ID != highID {
		t.Fatalf("unexpected deterministic order: %+v", result)
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
