package notification

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
	notificationstore "github.com/linenxing/e-commerce-system/stores/notification"
)

type fakeStore struct {
	template    models.NotificationTemplate
	tasks       []models.NotificationTask
	created     notificationstore.CreateTaskParams
	markedSent  int
	retried     int
	failed      int
	preferences *models.NotificationPreferences
}

func (s *fakeStore) GetTemplate(context.Context, uuid.UUID, int) (models.NotificationTemplate, error) {
	return s.template, nil
}
func (s *fakeStore) CreateTask(_ context.Context, p notificationstore.CreateTaskParams) (models.NotificationTask, bool, error) {
	s.created = p
	return models.NotificationTask{ID: uuid.New(), IdempotencyKey: p.IdempotencyKey}, true, nil
}
func (s *fakeStore) GetTask(context.Context, uuid.UUID) (models.NotificationTask, error) {
	return models.NotificationTask{}, nil
}
func (s *fakeStore) ListTasks(context.Context, *uuid.UUID) ([]models.NotificationTask, error) {
	return s.tasks, nil
}
func (s *fakeStore) ClaimTasks(context.Context, time.Time, time.Duration, int) ([]models.NotificationTask, error) {
	values := s.tasks
	s.tasks = nil
	return values, nil
}
func (s *fakeStore) MarkSent(context.Context, uuid.UUID, time.Time) error { s.markedSent++; return nil }
func (s *fakeStore) ScheduleRetry(context.Context, uuid.UUID, time.Time, string) error {
	s.retried++
	return nil
}
func (s *fakeStore) MarkFailed(context.Context, uuid.UUID, string) error         { s.failed++; return nil }
func (s *fakeStore) Retry(context.Context, uuid.UUID, time.Time) error           { return nil }
func (s *fakeStore) Open(context.Context, uuid.UUID, uuid.UUID, time.Time) error { return nil }
func (s *fakeStore) GetPreferences(context.Context, uuid.UUID) (models.NotificationPreferences, error) {
	if s.preferences != nil {
		return *s.preferences, nil
	}
	return models.NotificationPreferences{MarketingConsent: true, Channels: []models.NotificationChannel{models.NotificationChannelPush}}, nil
}
func (s *fakeStore) UpdatePreferences(_ context.Context, _ uuid.UUID, value models.NotificationPreferences) (models.NotificationPreferences, error) {
	return value, nil
}
func (s *fakeStore) RecordDelivery(context.Context, uuid.UUID, string, time.Time) (bool, error) {
	return true, nil
}

type fakeProvider struct{ err error }

func (p fakeProvider) Deliver(context.Context, models.NotificationTask, models.NotificationPayload) error {
	return p.err
}

func TestCreateTaskBuildsDeterministicIdempotencyKeyAndEscapesVariables(t *testing.T) {
	store := &fakeStore{template: models.NotificationTemplate{Channel: models.NotificationChannelPush, TitleTemplate: `Offer for {{.name}}`, BodyTemplate: `Save {{.amount}}`, DeepLinkTemplate: `ecommerce://products/{{.product_id}}`}}
	journeyID, userID, templateID := uuid.New(), uuid.New(), uuid.New()
	service := &service{store: store, now: time.Now}
	_, created, err := service.CreateTask(context.Background(), CreateTaskParam{UserID: userID, JourneyID: journeyID, JourneyType: "cart_recall", TemplateID: templateID, TemplateVersion: 2, Channel: models.NotificationChannelPush, Variables: map[string]string{"name": `<Admin>`, "amount": "100", "product_id": "abc"}})
	if err != nil || !created {
		t.Fatalf("create task: created=%v err=%v", created, err)
	}
	expected := "cart_recall:" + journeyID.String() + ":push:2"
	if store.created.IdempotencyKey != expected {
		t.Fatalf("unexpected idempotency key: %s", store.created.IdempotencyKey)
	}
	if store.created.Payload.Title != "Offer for &lt;Admin&gt;" {
		t.Fatalf("template variables must be escaped: %s", store.created.Payload.Title)
	}
}

func TestWorkerRetriesTemporaryFailure(t *testing.T) {
	task := models.NotificationTask{ID: uuid.New(), Channel: models.NotificationChannelPush, AttemptCount: 1, Payload: []byte(`{"title":"title","body":"body"}`)}
	store := &fakeStore{tasks: []models.NotificationTask{task}}
	worker := NewWorker(store, map[models.NotificationChannel]DeliveryProvider{models.NotificationChannelPush: fakeProvider{err: TemporaryDeliveryError{Err: errors.New("timeout")}}})
	if _, err := worker.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("process batch: %v", err)
	}
	if store.retried != 1 || store.failed != 0 {
		t.Fatalf("temporary error must retry: retried=%d failed=%d", store.retried, store.failed)
	}
}

func TestWorkerStopsRetryingAtMaxAttempts(t *testing.T) {
	task := models.NotificationTask{ID: uuid.New(), Channel: models.NotificationChannelPush, AttemptCount: defaultMaxAttempts, Payload: []byte(`{"title":"title","body":"body"}`)}
	store := &fakeStore{tasks: []models.NotificationTask{task}}
	worker := NewWorker(store, map[models.NotificationChannel]DeliveryProvider{models.NotificationChannelPush: fakeProvider{err: TemporaryDeliveryError{Err: errors.New("timeout")}}})
	if _, err := worker.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("process batch: %v", err)
	}
	if store.failed != 1 || store.retried != 0 {
		t.Fatalf("exhausted task must fail: retried=%d failed=%d", store.retried, store.failed)
	}
}

func TestWorkerRejectsRevokedConsentBeforeDelivery(t *testing.T) {
	task := models.NotificationTask{ID: uuid.New(), UserID: uuid.New(), Channel: models.NotificationChannelPush, AttemptCount: 1, Payload: []byte(`{"title":"title","body":"body"}`)}
	preferences := models.NotificationPreferences{MarketingConsent: false, Channels: []models.NotificationChannel{models.NotificationChannelPush}}
	store := &fakeStore{tasks: []models.NotificationTask{task}, preferences: &preferences}
	worker := NewWorker(store, map[models.NotificationChannel]DeliveryProvider{models.NotificationChannelPush: fakeProvider{}})
	if _, err := worker.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("process batch: %v", err)
	}
	if store.failed != 1 || store.markedSent != 0 {
		t.Fatalf("revoked consent must fail before delivery: failed=%d sent=%d", store.failed, store.markedSent)
	}
}

func TestDeepLinkUsesTextRenderingAndHostAllowlist(t *testing.T) {
	tmpl := models.NotificationTemplate{TitleTemplate: "Title", BodyTemplate: "Body", DeepLinkTemplate: `ecommerce://products/1?source={{.source}}`}
	payload, err := renderPayload(tmpl, map[string]string{"source": "campaign&slot=hero"})
	if err != nil {
		t.Fatalf("render payload: %v", err)
	}
	if payload.DeepLink != "ecommerce://products/1?source=campaign%26slot%3Dhero" {
		t.Fatalf("deep link variables must be URL escaped: %s", payload.DeepLink)
	}
	tmpl.DeepLinkTemplate = `https://attacker.example/path`
	if _, err = renderPayload(tmpl, nil); err == nil {
		t.Fatal("external host must be rejected")
	}
}

func TestSeedTemplatesMatchDeepLinkRendererContract(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	migrationPath := filepath.Join(filepath.Dir(filename), "..", "..", "infra", "postgres", "migrations", "000008_create_notifications.up.sql")
	migration, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read notification migration: %v", err)
	}
	pattern := regexp.MustCompile(`'(?:in_app|push)', '\{\{\.title\}\}', '\{\{\.body\}\}', '([^']+)'`)
	matches := pattern.FindAllSubmatch(migration, -1)
	if len(matches) != 2 {
		t.Fatalf("expected two seeded notification templates, got %d", len(matches))
	}
	productID := uuid.NewString()
	for _, match := range matches {
		tmpl := models.NotificationTemplate{TitleTemplate: "{{.title}}", BodyTemplate: "{{.body}}", DeepLinkTemplate: string(match[1])}
		payload, renderErr := renderPayload(tmpl, map[string]string{"title": "Recall", "body": "Return to cart", "product_id": productID})
		if renderErr != nil {
			t.Fatalf("render seeded template %q: %v", tmpl.DeepLinkTemplate, renderErr)
		}
		expected := "ecommerce://products/" + productID
		if payload.DeepLink != expected {
			t.Fatalf("unexpected seeded deep link: got %q want %q", payload.DeepLink, expected)
		}
	}
}

func TestListForUserOnlyReturnsDeliveredInAppTasks(t *testing.T) {
	store := &fakeStore{tasks: []models.NotificationTask{{ID: uuid.New(), Channel: models.NotificationChannelPush, Status: models.NotificationTaskDelivered}, {ID: uuid.New(), Channel: models.NotificationChannelInApp, Status: models.NotificationTaskPending}, {ID: uuid.New(), Channel: models.NotificationChannelInApp, Status: models.NotificationTaskDelivered}}}
	service := &service{store: store, now: time.Now}
	values, err := service.ListForUser(context.Background(), uuid.New())
	if err != nil || len(values) != 1 {
		t.Fatalf("unexpected visible notifications: values=%v err=%v", values, err)
	}
}
