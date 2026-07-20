package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"net/url"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/base/apperror"
	"github.com/linenxing/e-commerce-system/base/logger"
	"github.com/linenxing/e-commerce-system/models"
	notificationstore "github.com/linenxing/e-commerce-system/stores/notification"
)

const (
	defaultMaxAttempts       = 5
	defaultBatchSize         = 20
	defaultProcessingTimeout = 5 * time.Minute
)

type service struct {
	store notificationstore.Store
	now   func() time.Time
}

func New(store notificationstore.Store) Service { return &service{store: store, now: time.Now} }

func (s *service) CreateTask(ctx context.Context, p CreateTaskParam) (models.NotificationTask, bool, error) {
	if p.UserID == uuid.Nil || p.JourneyID == uuid.Nil || p.TemplateID == uuid.Nil || strings.TrimSpace(p.JourneyType) == "" || p.TemplateVersion <= 0 || (p.Channel != models.NotificationChannelInApp && p.Channel != models.NotificationChannelPush) {
		return models.NotificationTask{}, false, apperror.ErrInvalidInput
	}
	tmpl, err := s.store.GetTemplate(ctx, p.TemplateID, p.TemplateVersion)
	if err != nil {
		return models.NotificationTask{}, false, mapStoreError(err)
	}
	if tmpl.Channel != p.Channel {
		return models.NotificationTask{}, false, apperror.ErrInvalidInput
	}
	payload, err := renderPayload(tmpl, p.Variables)
	if err != nil {
		return models.NotificationTask{}, false, apperror.ErrInvalidInput
	}
	if p.ScheduledAt.IsZero() {
		p.ScheduledAt = s.now()
	}
	key := fmt.Sprintf("%s:%s:%s:%d", p.JourneyType, p.JourneyID, p.Channel, p.TemplateVersion)
	value, created, err := s.store.CreateTask(ctx, notificationstore.CreateTaskParams{UserID: p.UserID, CampaignID: p.CampaignID, JourneyType: p.JourneyType, JourneyID: p.JourneyID, TemplateID: p.TemplateID, TemplateVersion: p.TemplateVersion, Channel: p.Channel, IdempotencyKey: key, ScheduledAt: p.ScheduledAt, Payload: payload})
	return value, created, mapStoreError(err)
}

func (s *service) ListForUser(ctx context.Context, userID uuid.UUID) ([]models.NotificationTask, error) {
	values, err := s.store.ListTasks(ctx, &userID)
	if err != nil {
		return nil, err
	}
	result := make([]models.NotificationTask, 0, len(values))
	for _, value := range values {
		if value.Channel == models.NotificationChannelInApp && (value.Status == models.NotificationTaskDelivered || value.Status == models.NotificationTaskOpened) {
			result = append(result, value)
		}
	}
	return result, nil
}
func (s *service) ListAdmin(ctx context.Context) ([]models.NotificationTask, error) {
	return s.store.ListTasks(ctx, nil)
}
func (s *service) GetAdmin(ctx context.Context, id uuid.UUID) (models.NotificationTask, error) {
	value, err := s.store.GetTask(ctx, id)
	return value, mapStoreError(err)
}
func (s *service) Open(ctx context.Context, id, userID uuid.UUID) error {
	return mapStoreError(s.store.Open(ctx, id, userID, s.now()))
}
func (s *service) Retry(ctx context.Context, id uuid.UUID) error {
	return mapStoreError(s.store.Retry(ctx, id, s.now()))
}
func (s *service) GetPreferences(ctx context.Context, userID uuid.UUID) (models.NotificationPreferences, error) {
	value, err := s.store.GetPreferences(ctx, userID)
	return value, mapStoreError(err)
}
func (s *service) UpdatePreferences(ctx context.Context, userID uuid.UUID, value models.NotificationPreferences) (models.NotificationPreferences, error) {
	seen := map[models.NotificationChannel]bool{}
	channels := make([]models.NotificationChannel, 0, len(value.Channels))
	for _, channel := range value.Channels {
		if channel != models.NotificationChannelInApp && channel != models.NotificationChannelPush {
			return models.NotificationPreferences{}, apperror.ErrInvalidInput
		}
		if !seen[channel] {
			seen[channel] = true
			channels = append(channels, channel)
		}
	}
	value.Channels = channels
	result, err := s.store.UpdatePreferences(ctx, userID, value)
	return result, mapStoreError(err)
}

type Worker struct {
	store             notificationstore.Store
	providers         map[models.NotificationChannel]DeliveryProvider
	now               func() time.Time
	maxAttempts       int
	batchSize         int
	processingTimeout time.Duration
}

func NewWorker(store notificationstore.Store, providers map[models.NotificationChannel]DeliveryProvider) *Worker {
	return &Worker{store: store, providers: providers, now: time.Now, maxAttempts: defaultMaxAttempts, batchSize: defaultBatchSize, processingTimeout: defaultProcessingTimeout}
}

func (w *Worker) ProcessBatch(ctx context.Context) (int, error) {
	now := w.now()
	tasks, err := w.store.ClaimTasks(ctx, now, w.processingTimeout, w.batchSize)
	if err != nil {
		return 0, err
	}
	for _, task := range tasks {
		if err = w.deliver(ctx, task); err != nil {
			return len(tasks), err
		}
	}
	return len(tasks), nil
}

func (w *Worker) deliver(ctx context.Context, task models.NotificationTask) error {
	preferences, err := w.store.GetPreferences(ctx, task.UserID)
	if err != nil {
		if task.AttemptCount < w.maxAttempts {
			return w.store.ScheduleRetry(ctx, task.ID, w.now().Add(retryDelay(task.ID, task.AttemptCount)), "PREFERENCE_LOOKUP_FAILED")
		}
		return w.store.MarkFailed(ctx, task.ID, "PREFERENCE_LOOKUP_FAILED")
	}
	if !preferences.MarketingConsent {
		return w.store.MarkFailed(ctx, task.ID, "MARKETING_CONSENT_DISABLED")
	}
	if !hasChannel(preferences.Channels, task.Channel) {
		return w.store.MarkFailed(ctx, task.ID, "NO_NOTIFICATION_CHANNEL")
	}
	provider, exists := w.providers[task.Channel]
	if !exists {
		return w.store.MarkFailed(ctx, task.ID, "PROVIDER_PERMANENT_FAILURE")
	}
	var payload models.NotificationPayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil {
		return w.store.MarkFailed(ctx, task.ID, "INVALID_PAYLOAD")
	}
	err = provider.Deliver(ctx, task, payload)
	if err == nil {
		return w.store.MarkSent(ctx, task.ID, w.now())
	}
	var temporary TemporaryDeliveryError
	if errors.As(err, &temporary) && task.AttemptCount < w.maxAttempts {
		return w.store.ScheduleRetry(ctx, task.ID, w.now().Add(retryDelay(task.ID, task.AttemptCount)), "PROVIDER_TEMPORARY_FAILURE")
	}
	code := "PROVIDER_PERMANENT_FAILURE"
	if errors.As(err, &temporary) {
		code = "PROVIDER_TEMPORARY_FAILURE"
	}
	return w.store.MarkFailed(ctx, task.ID, code)
}

func retryDelay(taskID uuid.UUID, attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	if attempt > 8 {
		attempt = 8
	}
	base := time.Minute * time.Duration(1<<(attempt-1))
	jitter := time.Duration(taskID[15]%30) * time.Second
	return base + jitter
}

type MockProvider struct {
	recorder DeliveryRecorder
	now      func() time.Time
}

func NewMockProvider(recorder DeliveryRecorder) *MockProvider {
	return &MockProvider{recorder: recorder, now: time.Now}
}
func (p *MockProvider) Deliver(ctx context.Context, task models.NotificationTask, _ models.NotificationPayload) error {
	created, err := p.recorder.RecordDelivery(ctx, task.ID, task.IdempotencyKey, p.now())
	if err != nil {
		return TemporaryDeliveryError{Err: err}
	}
	if !created {
		return nil
	}
	logger.FromContext(ctx).Info().Str("notification_task_id", task.ID.String()).Str("campaign_id", idString(task.CampaignID)).Str("journey_type", task.JourneyType).Str("journey_id", task.JourneyID.String()).Str("channel", string(task.Channel)).Str("decision", "delivered").Msg("mock notification delivered")
	return nil
}

func renderPayload(tmpl models.NotificationTemplate, variables map[string]string) (models.NotificationPayload, error) {
	title, err := render(tmpl.TitleTemplate, variables)
	if err != nil {
		return models.NotificationPayload{}, err
	}
	body, err := render(tmpl.BodyTemplate, variables)
	if err != nil {
		return models.NotificationPayload{}, err
	}
	deepLink, err := renderDeepLink(tmpl.DeepLinkTemplate, variables)
	if err != nil {
		return models.NotificationPayload{}, err
	}
	if deepLink != "" {
		parsed, parseErr := url.Parse(deepLink)
		if parseErr != nil || parsed.Scheme != "ecommerce" || !allowedDeepLinkHost(parsed.Host) || parsed.User != nil {
			return models.NotificationPayload{}, errors.New("deep link is not allowed")
		}
	}
	if strings.TrimSpace(title) == "" || strings.TrimSpace(body) == "" {
		return models.NotificationPayload{}, errors.New("title and body are required")
	}
	return models.NotificationPayload{Title: title, Body: body, DeepLink: deepLink}, nil
}
func render(source string, variables map[string]string) (string, error) {
	parsed, err := htmltemplate.New("notification").Option("missingkey=error").Parse(source)
	if err != nil {
		return "", err
	}
	var output bytes.Buffer
	err = parsed.Execute(&output, variables)
	return output.String(), err
}

func renderDeepLink(source string, variables map[string]string) (string, error) {
	escapedVariables := make(map[string]string, len(variables))
	for name, value := range variables {
		escapedVariables[name] = url.QueryEscape(value)
	}
	parsed, err := texttemplate.New("deep_link").Option("missingkey=error").Parse(source)
	if err != nil {
		return "", err
	}
	var output bytes.Buffer
	err = parsed.Execute(&output, escapedVariables)
	return output.String(), err
}

func allowedDeepLinkHost(host string) bool {
	switch strings.ToLower(host) {
	case "products", "campaigns", "cart":
		return true
	default:
		return false
	}
}

func hasChannel(channels []models.NotificationChannel, target models.NotificationChannel) bool {
	for _, channel := range channels {
		if channel == target {
			return true
		}
	}
	return false
}
func mapStoreError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, notificationstore.ErrNotFound):
		return apperror.ErrNotFound
	case errors.Is(err, notificationstore.ErrConsentDisabled), errors.Is(err, notificationstore.ErrChannelDisabled):
		return apperror.ErrForbidden
	case errors.Is(err, notificationstore.ErrFrequencyLimited), errors.Is(err, notificationstore.ErrConflict):
		return apperror.ErrConflict
	default:
		return err
	}
}
func idString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
