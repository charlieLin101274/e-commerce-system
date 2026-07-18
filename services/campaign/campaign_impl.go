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
	"github.com/linenxing/e-commerce-system/models"
	campaignstore "github.com/linenxing/e-commerce-system/stores/campaign"
)

type service struct {
	store campaignstore.Store
	now   func() time.Time
}

func New(store campaignstore.Store) Service { return &service{store: store, now: time.Now} }

func (s *service) Create(ctx context.Context, createdBy uuid.UUID, param WriteParam) (models.Campaign, error) {
	value, err := buildCampaign(param)
	if err != nil {
		return models.Campaign{}, err
	}
	value.Status = models.CampaignStatusDraft
	value.CreatedBy = createdBy
	return s.store.Create(ctx, value)
}

func (s *service) Update(ctx context.Context, id uuid.UUID, param WriteParam) (models.Campaign, error) {
	current, err := s.get(ctx, id)
	if err != nil {
		return models.Campaign{}, err
	}
	if current.Status != models.CampaignStatusDraft {
		return models.Campaign{}, apperror.ErrConflict
	}
	value, err := buildCampaign(param)
	if err != nil {
		return models.Campaign{}, err
	}
	value.ID, value.Status, value.CreatedBy = current.ID, current.Status, current.CreatedBy
	value.CreatedAt, value.PublishedAt = current.CreatedAt, current.PublishedAt
	return s.update(ctx, value)
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

func (s *service) ListPublic(ctx context.Context, productID *uuid.UUID) ([]models.Campaign, error) {
	items, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	category, err := s.productCategory(ctx, productID)
	if err != nil {
		return nil, err
	}
	result := make([]models.Campaign, 0)
	for _, value := range items {
		if effectiveStatus(value, s.now()) == models.CampaignStatusRunning && matchesScope(value, productID, category) {
			value.Status = models.CampaignStatusRunning
			result = append(result, value)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority > result[j].Priority
		}
		return result[i].ID.String() < result[j].ID.String()
	})
	return result, nil
}

func (s *service) GetPublic(ctx context.Context, id uuid.UUID, productID *uuid.UUID) (models.Campaign, error) {
	value, err := s.get(ctx, id)
	if err != nil || effectiveStatus(value, s.now()) != models.CampaignStatusRunning {
		return models.Campaign{}, apperror.ErrNotFound
	}
	category, err := s.productCategory(ctx, productID)
	if err != nil {
		return models.Campaign{}, err
	}
	if !matchesScope(value, productID, category) {
		return models.Campaign{}, apperror.ErrNotFound
	}
	value.Status = models.CampaignStatusRunning
	return value, nil
}

func (s *service) productCategory(ctx context.Context, productID *uuid.UUID) (string, error) {
	if productID == nil {
		return "", nil
	}
	products, err := s.store.GetProductCategories(ctx, []uuid.UUID{*productID})
	if err != nil {
		return "", err
	}
	category, exists := products[*productID]
	if !exists {
		return "", apperror.ErrNotFound
	}
	return normalizeCategory(category), nil
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
