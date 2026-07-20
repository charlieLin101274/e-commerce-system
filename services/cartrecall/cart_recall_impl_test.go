package cartrecall

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/base/apperror"
	"github.com/linenxing/e-commerce-system/models"
	notificationservice "github.com/linenxing/e-commerce-system/services/notification"
	store "github.com/linenxing/e-commerce-system/stores/cartrecall"
)

type fakeStore struct {
	state       store.CartState
	member      models.MemberFacts
	skipped     string
	pending     bool
	snapshots   []models.CartRecallProductSnapshot
	pendingTask uuid.UUID
}

func (f *fakeStore) ConsumeEvents(context.Context, time.Time, time.Duration, int) (int, error) {
	return 0, nil
}
func (f *fakeStore) ClaimDue(context.Context, time.Time, time.Duration, int) ([]models.CartRecallJourney, error) {
	return nil, nil
}
func (f *fakeStore) GetCartState(context.Context, uuid.UUID) (store.CartState, error) {
	return f.state, nil
}
func (f *fakeStore) GetMemberFacts(context.Context, uuid.UUID) (models.MemberFacts, error) {
	return f.member, nil
}
func (f *fakeStore) MarkSkipped(_ context.Context, _ uuid.UUID, reason string) error {
	f.skipped = reason
	return nil
}
func (f *fakeStore) MarkCancelled(_ context.Context, _ uuid.UUID, reason string) error {
	f.skipped = reason
	return nil
}
func (f *fakeStore) MarkNotificationPending(_ context.Context, _ uuid.UUID, _ models.Campaign, snapshots []models.CartRecallProductSnapshot, task uuid.UUID) error {
	f.pending = true
	f.snapshots = snapshots
	f.pendingTask = task
	return nil
}
func (f *fakeStore) CancelInvalidPending(context.Context, time.Time) (int, error) { return 0, nil }
func (f *fakeStore) SyncDelivered(context.Context, time.Time) (int, error)        { return 0, nil }
func (f *fakeStore) List(context.Context) ([]models.CartRecallJourney, error)     { return nil, nil }
func (f *fakeStore) Get(context.Context, uuid.UUID) (models.CartRecallJourney, error) {
	return models.CartRecallJourney{}, nil
}
func (f *fakeStore) Cancel(context.Context, uuid.UUID, string) error { return nil }

type fakeCampaignMatcher struct {
	campaign models.Campaign
	decision models.EvaluationResult
}

func (f fakeCampaignMatcher) MatchCartRecall(context.Context, models.EvaluationFacts) (models.Campaign, models.EvaluationResult, error) {
	return f.campaign, f.decision, nil
}

type fakeNotifications struct {
	preferences models.NotificationPreferences
	task        models.NotificationTask
	created     notificationservice.CreateTaskParam
	createErr   error
}

func (f *fakeNotifications) GetPreferences(context.Context, uuid.UUID) (models.NotificationPreferences, error) {
	return f.preferences, nil
}
func (f *fakeNotifications) CreateTask(_ context.Context, p notificationservice.CreateTaskParam) (models.NotificationTask, bool, error) {
	f.created = p
	return f.task, true, f.createErr
}

func TestWorkerClassifiesCreateTaskErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		reason string
	}{
		{name: "frequency", err: notificationservice.ErrFrequencyLimited, reason: "FREQUENCY_LIMIT_REACHED"},
		{name: "consent", err: notificationservice.ErrConsentDisabled, reason: "MARKETING_CONSENT_DISABLED"},
		{name: "channel", err: notificationservice.ErrChannelDisabled, reason: "NO_NOTIFICATION_CHANNEL"},
		{name: "payload", err: apperror.ErrInvalidInput, reason: "INVALID_PAYLOAD"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
			journey := models.CartRecallJourney{ID: uuid.New(), UserID: uuid.New(), CartID: uuid.New(), SourceEventID: uuid.New()}
			valueStore := &fakeStore{state: eligibleCartState(now), member: models.MemberFacts{ID: journey.UserID}}
			notifications := &fakeNotifications{preferences: models.NotificationPreferences{MarketingConsent: true, Channels: []models.NotificationChannel{models.NotificationChannelPush}}, createErr: test.err}
			matcher := fakeCampaignMatcher{campaign: models.Campaign{ID: uuid.New(), PromotionTitle: "Save"}, decision: models.EvaluationResult{Eligible: true}}
			worker := NewWorker(valueStore, matcher, notifications, time.Minute)
			worker.now = func() time.Time { return now }
			if err := worker.evaluate(context.Background(), journey); err != nil {
				t.Fatal(err)
			}
			if valueStore.skipped != test.reason {
				t.Fatalf("reason=%s want=%s", valueStore.skipped, test.reason)
			}
		})
	}
}

func TestWorkerReturnsUnexpectedCreateTaskError(t *testing.T) {
	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	journey := models.CartRecallJourney{ID: uuid.New(), UserID: uuid.New(), CartID: uuid.New()}
	valueStore := &fakeStore{state: eligibleCartState(now), member: models.MemberFacts{ID: journey.UserID}}
	expected := errors.New("database unavailable")
	notifications := &fakeNotifications{preferences: models.NotificationPreferences{MarketingConsent: true, Channels: []models.NotificationChannel{models.NotificationChannelPush}}, createErr: expected}
	worker := NewWorker(valueStore, fakeCampaignMatcher{campaign: models.Campaign{ID: uuid.New(), PromotionTitle: "Save"}, decision: models.EvaluationResult{Eligible: true}}, notifications, time.Minute)
	worker.now = func() time.Time { return now }
	if err := worker.evaluate(context.Background(), journey); !errors.Is(err, expected) {
		t.Fatalf("error=%v want=%v", err, expected)
	}
	if valueStore.skipped != "" {
		t.Fatalf("unexpected skip: %s", valueStore.skipped)
	}
}

func TestWorkerSkipsWhenMarketingConsentDisabled(t *testing.T) {
	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	journey := models.CartRecallJourney{ID: uuid.New(), UserID: uuid.New(), CartID: uuid.New()}
	valueStore := &fakeStore{state: eligibleCartState(now), member: models.MemberFacts{ID: journey.UserID}}
	notifications := &fakeNotifications{preferences: models.NotificationPreferences{MarketingConsent: false, Channels: []models.NotificationChannel{models.NotificationChannelPush}}}
	worker := NewWorker(valueStore, fakeCampaignMatcher{}, notifications, time.Minute)
	worker.now = func() time.Time { return now }
	worker.delay = time.Minute
	if err := worker.evaluate(context.Background(), journey); err != nil {
		t.Fatal(err)
	}
	if valueStore.skipped != "MARKETING_CONSENT_DISABLED" {
		t.Fatalf("unexpected reason: %s", valueStore.skipped)
	}
}

func TestWorkerCreatesOneNotificationWithSnapshot(t *testing.T) {
	now := time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)
	productID, campaignID, taskID := uuid.New(), uuid.New(), uuid.New()
	journey := models.CartRecallJourney{ID: uuid.New(), UserID: uuid.New(), CartID: uuid.New(), SourceEventID: uuid.New()}
	state := eligibleCartState(now)
	state.Items[0].Product.ID = productID
	valueStore := &fakeStore{state: state, member: models.MemberFacts{ID: journey.UserID}}
	notifications := &fakeNotifications{preferences: models.NotificationPreferences{MarketingConsent: true, Channels: []models.NotificationChannel{models.NotificationChannelPush}}, task: models.NotificationTask{ID: taskID}}
	benefit := models.BenefitResult{OriginalAmount: 1000, DiscountAmount: 100, FinalAmount: 900}
	matcher := fakeCampaignMatcher{campaign: models.Campaign{ID: campaignID, Priority: 10, PromotionTitle: "Save", RuleVersion: 2}, decision: models.EvaluationResult{Eligible: true, BenefitPreview: &benefit}}
	worker := NewWorker(valueStore, matcher, notifications, time.Minute)
	worker.now = func() time.Time { return now }
	worker.delay = time.Minute
	if err := worker.evaluate(context.Background(), journey); err != nil {
		t.Fatal(err)
	}
	if !valueStore.pending || valueStore.pendingTask != taskID {
		t.Fatal("journey was not moved to notification pending")
	}
	if len(valueStore.snapshots) != 1 || valueStore.snapshots[0].ProductID != productID || valueStore.snapshots[0].EvaluatedBenefit.DiscountAmount != 100 {
		t.Fatalf("unexpected snapshot: %+v", valueStore.snapshots)
	}
	if notifications.created.JourneyID != journey.ID || notifications.created.CampaignID == nil || *notifications.created.CampaignID != campaignID {
		t.Fatalf("unexpected notification params: %+v", notifications.created)
	}
}

func eligibleCartState(now time.Time) store.CartState {
	return store.CartState{LastChange: now.Add(-time.Hour), Items: []store.CartItemState{{Product: models.Product{ID: uuid.New(), Category: "Electronics", Price: 1000, Stock: 2, Status: models.ProductStatusActive}, Quantity: 1}}}
}
