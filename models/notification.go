package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type NotificationChannel string

const (
	NotificationChannelInApp NotificationChannel = "in_app"
	NotificationChannelPush  NotificationChannel = "push"
)

type NotificationTaskStatus string

const (
	NotificationTaskPending        NotificationTaskStatus = "pending"
	NotificationTaskProcessing     NotificationTaskStatus = "processing"
	NotificationTaskSent           NotificationTaskStatus = "sent"
	NotificationTaskDelivered      NotificationTaskStatus = "delivered"
	NotificationTaskOpened         NotificationTaskStatus = "opened"
	NotificationTaskRetryScheduled NotificationTaskStatus = "retry_scheduled"
	NotificationTaskFailed         NotificationTaskStatus = "failed"
	NotificationTaskCancelled      NotificationTaskStatus = "cancelled"
)

type NotificationTemplate struct {
	ID               uuid.UUID           `json:"id"`
	Channel          NotificationChannel `json:"channel"`
	TitleTemplate    string              `json:"title_template"`
	BodyTemplate     string              `json:"body_template"`
	DeepLinkTemplate string              `json:"deep_link_template"`
	Version          int                 `json:"version"`
	Status           string              `json:"status"`
}

type NotificationPayload struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	DeepLink string `json:"deep_link"`
}

type NotificationTask struct {
	ID              uuid.UUID              `json:"id"`
	UserID          uuid.UUID              `json:"user_id,omitempty"`
	CampaignID      *uuid.UUID             `json:"campaign_id,omitempty"`
	JourneyType     string                 `json:"journey_type"`
	JourneyID       uuid.UUID              `json:"journey_id"`
	TemplateID      uuid.UUID              `json:"template_id"`
	TemplateVersion int                    `json:"template_version"`
	Channel         NotificationChannel    `json:"channel"`
	Status          NotificationTaskStatus `json:"status"`
	IdempotencyKey  string                 `json:"idempotency_key,omitempty"`
	ScheduledAt     time.Time              `json:"scheduled_at"`
	AttemptCount    int                    `json:"attempt_count"`
	NextAttemptAt   time.Time              `json:"next_attempt_at"`
	SentAt          *time.Time             `json:"sent_at,omitempty"`
	OpenedAt        *time.Time             `json:"opened_at,omitempty"`
	FailureCode     string                 `json:"failure_code,omitempty"`
	Payload         json.RawMessage        `json:"payload"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

type NotificationPreferences struct {
	MarketingConsent bool                  `json:"marketing_consent"`
	Channels         []NotificationChannel `json:"notification_channels"`
}
