package notification

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

var (
	ErrConsentDisabled  = errors.New("marketing consent disabled")
	ErrChannelDisabled  = errors.New("notification channel disabled")
	ErrFrequencyLimited = errors.New("notification frequency limited")
)

type CreateTaskParam struct {
	UserID          uuid.UUID
	CampaignID      *uuid.UUID
	JourneyType     string
	JourneyID       uuid.UUID
	TemplateID      uuid.UUID
	TemplateVersion int
	Channel         models.NotificationChannel
	ScheduledAt     time.Time
	Variables       map[string]string
}

type Service interface {
	CreateTask(context.Context, CreateTaskParam) (models.NotificationTask, bool, error)
	ListForUser(context.Context, uuid.UUID) ([]models.NotificationTask, error)
	Open(context.Context, uuid.UUID, uuid.UUID) error
	ListAdmin(context.Context) ([]models.NotificationTask, error)
	GetAdmin(context.Context, uuid.UUID) (models.NotificationTask, error)
	Retry(context.Context, uuid.UUID) error
	GetPreferences(context.Context, uuid.UUID) (models.NotificationPreferences, error)
	UpdatePreferences(context.Context, uuid.UUID, models.NotificationPreferences) (models.NotificationPreferences, error)
}

type DeliveryProvider interface {
	Deliver(context.Context, models.NotificationTask, models.NotificationPayload) error
}

type DeliveryRecorder interface {
	RecordDelivery(context.Context, uuid.UUID, string, time.Time) (bool, error)
}

type TemporaryDeliveryError struct{ Err error }

func (e TemporaryDeliveryError) Error() string { return e.Err.Error() }
func (e TemporaryDeliveryError) Unwrap() error { return e.Err }
