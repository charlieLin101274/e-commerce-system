package cartrecall

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/base/apperror"
	"github.com/linenxing/e-commerce-system/base/logger"
	"github.com/linenxing/e-commerce-system/models"
	notificationservice "github.com/linenxing/e-commerce-system/services/notification"
	store "github.com/linenxing/e-commerce-system/stores/cartrecall"
)

const (
	defaultDelay             = 30 * time.Minute
	defaultBatchSize         = 20
	defaultProcessingTimeout = 5 * time.Minute
	inAppTemplateID          = "10000000-0000-0000-0000-000000000001"
	pushTemplateID           = "10000000-0000-0000-0000-000000000002"
)

type service struct{ store store.Store }

func New(value store.Store) Service { return &service{store: value} }
func (s *service) List(ctx context.Context) ([]models.CartRecallJourney, error) {
	return s.store.List(ctx)
}
func (s *service) Get(ctx context.Context, id uuid.UUID) (models.CartRecallJourney, error) {
	v, err := s.store.Get(ctx, id)
	return v, mapStoreError(err)
}
func (s *service) Cancel(ctx context.Context, id uuid.UUID) error {
	return mapStoreError(s.store.Cancel(ctx, id, "ADMIN_CANCELLED"))
}

type Worker struct {
	store             store.Store
	campaigns         CampaignMatcher
	notifications     NotificationService
	now               func() time.Time
	delay             time.Duration
	batchSize         int
	processingTimeout time.Duration
}

type CampaignMatcher interface {
	MatchCartRecall(context.Context, models.EvaluationFacts) (models.Campaign, models.EvaluationResult, error)
}

type NotificationService interface {
	GetPreferences(context.Context, uuid.UUID) (models.NotificationPreferences, error)
	CreateTask(context.Context, notificationservice.CreateTaskParam) (models.NotificationTask, bool, error)
}

func NewWorker(value store.Store, campaigns CampaignMatcher, notifications NotificationService, delay time.Duration) *Worker {
	if delay <= 0 {
		delay = defaultDelay
	}
	return &Worker{store: value, campaigns: campaigns, notifications: notifications, now: time.Now, delay: delay, batchSize: defaultBatchSize, processingTimeout: defaultProcessingTimeout}
}

func (w *Worker) ProcessBatch(ctx context.Context) (int, error) {
	now := w.now()
	events, err := w.store.ConsumeEvents(ctx, now, w.delay, w.batchSize)
	if err != nil {
		return events, err
	}
	journeys, err := w.store.ClaimDue(ctx, now, w.processingTimeout, w.batchSize)
	if err != nil {
		return events, err
	}
	for _, journey := range journeys {
		if err = w.evaluate(ctx, journey); err != nil {
			return events + len(journeys), err
		}
	}
	if _, err = w.store.CancelInvalidPending(ctx, now); err != nil {
		return events + len(journeys), err
	}
	_, err = w.store.SyncDelivered(ctx, now)
	return events + len(journeys), err
}

func (w *Worker) evaluate(ctx context.Context, journey models.CartRecallJourney) error {
	state, err := w.store.GetCartState(ctx, journey.CartID)
	if errors.Is(err, store.ErrNotFound) {
		return w.skip(ctx, journey, "CART_EMPTY")
	}
	if err != nil {
		return err
	}
	if len(state.Items) == 0 {
		return w.skip(ctx, journey, "CART_EMPTY")
	}
	if state.LastChange.Add(w.delay).After(w.now()) {
		return w.skip(ctx, journey, "CART_CHANGED_RECENTLY")
	}
	member, err := w.store.GetMemberFacts(ctx, journey.UserID)
	if err != nil {
		return err
	}
	preferences, err := w.notifications.GetPreferences(ctx, journey.UserID)
	if err != nil {
		return w.skip(ctx, journey, "PREFERENCE_LOOKUP_FAILED")
	}
	if !preferences.MarketingConsent {
		return w.skip(ctx, journey, "MARKETING_CONSENT_DISABLED")
	}
	channel, ok := preferredChannel(preferences.Channels)
	if !ok {
		return w.skip(ctx, journey, "NO_NOTIFICATION_CHANNEL")
	}
	total := int64(0)
	for _, item := range state.Items {
		total += item.Product.Price * item.Quantity
	}
	factsCart := &models.CartFacts{TotalPrice: total, ItemCount: int64(len(state.Items))}
	var matchedCampaign models.Campaign
	var matchedDecision models.EvaluationResult
	var matchedItem *store.CartItemState
	for index := range state.Items {
		item := &state.Items[index]
		if item.Product.Status != models.ProductStatusActive {
			continue
		}
		if item.Product.Stock < item.Quantity {
			continue
		}
		facts := models.EvaluationFacts{Member: &member, Cart: factsCart, Product: &models.ProductFacts{ID: item.Product.ID, Category: item.Product.Category, Price: item.Product.Price, Status: item.Product.Status}}
		candidate, decision, matchErr := w.campaigns.MatchCartRecall(ctx, facts)
		if matchErr != nil {
			if errors.Is(matchErr, apperror.ErrNotFound) {
				continue
			}
			return matchErr
		}
		if matchedItem == nil || candidate.Priority > matchedCampaign.Priority || (candidate.Priority == matchedCampaign.Priority && candidate.ID.String() < matchedCampaign.ID.String()) {
			matchedCampaign, matchedDecision, matchedItem = candidate, decision, item
		}
	}
	if matchedItem == nil {
		reason := "NO_ELIGIBLE_CAMPAIGN"
		active, stock := false, false
		for _, item := range state.Items {
			if item.Product.Status == models.ProductStatusActive {
				active = true
				if item.Product.Stock >= item.Quantity {
					stock = true
				}
			}
		}
		if !active {
			reason = "PRODUCT_INACTIVE"
		} else if !stock {
			reason = "OUT_OF_STOCK"
		}
		return w.skip(ctx, journey, reason)
	}
	// Final lightweight validation closes the largest gap between evaluation and task creation.
	latest, err := w.store.GetCartState(ctx, journey.CartID)
	if err != nil || latest.LastChange.After(state.LastChange) {
		return w.cancel(ctx, journey, "CART_CHANGED_RECENTLY")
	}
	templateID := uuid.MustParse(inAppTemplateID)
	if channel == models.NotificationChannelPush {
		templateID = uuid.MustParse(pushTemplateID)
	}
	task, _, err := w.notifications.CreateTask(ctx, notificationservice.CreateTaskParam{UserID: journey.UserID, CampaignID: &matchedCampaign.ID, JourneyType: "cart_recall", JourneyID: journey.ID, TemplateID: templateID, TemplateVersion: 1, Channel: channel, ScheduledAt: w.now(), Variables: map[string]string{"title": matchedCampaign.PromotionTitle, "body": promotionBody(matchedCampaign), "product_id": matchedItem.Product.ID.String()}})
	if err != nil {
		switch {
		case errors.Is(err, notificationservice.ErrFrequencyLimited):
			return w.skip(ctx, journey, "FREQUENCY_LIMIT_REACHED")
		case errors.Is(err, notificationservice.ErrConsentDisabled):
			return w.skip(ctx, journey, "MARKETING_CONSENT_DISABLED")
		case errors.Is(err, notificationservice.ErrChannelDisabled):
			return w.skip(ctx, journey, "NO_NOTIFICATION_CHANNEL")
		case errors.Is(err, apperror.ErrInvalidInput):
			return w.skip(ctx, journey, "INVALID_PAYLOAD")
		default:
			return err
		}
	}
	snapshot := models.CartRecallProductSnapshot{ProductID: matchedItem.Product.ID, Category: strings.ToLower(strings.TrimSpace(matchedItem.Product.Category)), UnitPrice: matchedItem.Product.Price, Quantity: matchedItem.Quantity}
	if matchedDecision.BenefitPreview != nil {
		snapshot.EvaluatedBenefit = *matchedDecision.BenefitPreview
	}
	logger.FromContext(ctx).Info().Str("event_id", journey.SourceEventID.String()).Str("campaign_id", matchedCampaign.ID.String()).Str("journey_type", "cart_recall").Str("journey_id", journey.ID.String()).Str("notification_task_id", task.ID.String()).Str("decision", "notification_pending").Msg("cart recall evaluated")
	return w.store.MarkNotificationPending(ctx, journey.ID, matchedCampaign, []models.CartRecallProductSnapshot{snapshot}, task.ID)
}

func (w *Worker) skip(ctx context.Context, journey models.CartRecallJourney, reason string) error {
	if err := w.store.MarkSkipped(ctx, journey.ID, reason); err != nil {
		return err
	}
	w.logDecision(ctx, journey, "skipped", reason)
	return nil
}

func (w *Worker) cancel(ctx context.Context, journey models.CartRecallJourney, reason string) error {
	return w.store.MarkCancelled(ctx, journey.ID, reason)
}

func (w *Worker) logDecision(ctx context.Context, journey models.CartRecallJourney, decision, reason string) {
	logger.FromContext(ctx).Info().
		Str("event_id", journey.SourceEventID.String()).
		Str("campaign_id", optionalUUID(journey.CampaignID)).
		Str("journey_type", "cart_recall").
		Str("journey_id", journey.ID.String()).
		Str("notification_task_id", optionalUUID(journey.NotificationTaskID)).
		Str("decision", decision).
		Str("reason_code", reason).
		Msg("cart recall decision")
}

func preferredChannel(values []models.NotificationChannel) (models.NotificationChannel, bool) {
	for _, v := range values {
		if v == models.NotificationChannelPush {
			return v, true
		}
	}
	for _, v := range values {
		if v == models.NotificationChannelInApp {
			return v, true
		}
	}
	return "", false
}
func promotionBody(value models.Campaign) string {
	if strings.TrimSpace(value.PromotionDescription) != "" {
		return value.PromotionDescription
	}
	return fmt.Sprintf("Return to your cart to use %s.", value.PromotionTitle)
}
func mapStoreError(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return apperror.ErrNotFound
	}
	if errors.Is(err, store.ErrConflict) {
		return apperror.ErrConflict
	}
	return err
}

func optionalUUID(value *uuid.UUID) string {
	if value == nil {
		return ""
	}
	return value.String()
}
