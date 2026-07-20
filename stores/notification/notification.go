package notification

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

var (
	ErrNotFound         = errors.New("notification task not found")
	ErrConsentDisabled  = errors.New("marketing consent disabled")
	ErrChannelDisabled  = errors.New("notification channel disabled")
	ErrFrequencyLimited = errors.New("notification frequency limited")
	ErrConflict         = errors.New("notification task conflict")
)

type CreateTaskParams struct {
	UserID          uuid.UUID
	CampaignID      *uuid.UUID
	JourneyType     string
	JourneyID       uuid.UUID
	TemplateID      uuid.UUID
	TemplateVersion int
	Channel         models.NotificationChannel
	IdempotencyKey  string
	ScheduledAt     time.Time
	Payload         models.NotificationPayload
}

type Store interface {
	GetTemplate(context.Context, uuid.UUID, int) (models.NotificationTemplate, error)
	CreateTask(context.Context, CreateTaskParams) (models.NotificationTask, bool, error)
	GetTask(context.Context, uuid.UUID) (models.NotificationTask, error)
	ListTasks(context.Context, *uuid.UUID) ([]models.NotificationTask, error)
	ClaimTasks(context.Context, time.Time, time.Duration, int) ([]models.NotificationTask, error)
	MarkSent(context.Context, uuid.UUID, time.Time) error
	ScheduleRetry(context.Context, uuid.UUID, time.Time, string) error
	MarkFailed(context.Context, uuid.UUID, string) error
	Retry(context.Context, uuid.UUID, time.Time) error
	Open(context.Context, uuid.UUID, uuid.UUID, time.Time) error
	GetPreferences(context.Context, uuid.UUID) (models.NotificationPreferences, error)
	UpdatePreferences(context.Context, uuid.UUID, models.NotificationPreferences) (models.NotificationPreferences, error)
	RecordDelivery(context.Context, uuid.UUID, string, time.Time) (bool, error)
}
