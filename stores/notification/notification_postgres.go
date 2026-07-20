package notification

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/linenxing/e-commerce-system/models"
)

const taskColumns = `id,user_id,campaign_id,journey_type,journey_id,template_id,template_version,channel,status,idempotency_key,scheduled_at,attempt_count,next_attempt_at,sent_at,opened_at,COALESCE(failure_code,''),payload,created_at,updated_at`

type PostgresStore struct{ db *pgxpool.Pool }

func NewPostgresStore(db *pgxpool.Pool) *PostgresStore { return &PostgresStore{db: db} }

func (s *PostgresStore) GetTemplate(ctx context.Context, id uuid.UUID, version int) (models.NotificationTemplate, error) {
	var value models.NotificationTemplate
	err := s.db.QueryRow(ctx, `SELECT id,channel,title_template,body_template,deep_link_template,version,status FROM notification_templates WHERE id=$1 AND version=$2 AND status='active'`, id, version).Scan(&value.ID, &value.Channel, &value.TitleTemplate, &value.BodyTemplate, &value.DeepLinkTemplate, &value.Version, &value.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	return value, err
}

func (s *PostgresStore) CreateTask(ctx context.Context, p CreateTaskParams) (models.NotificationTask, bool, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return models.NotificationTask{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	value, lookupErr := scanTask(tx.QueryRow(ctx, `SELECT `+taskColumns+` FROM notification_tasks WHERE idempotency_key=$1`, p.IdempotencyKey))
	if lookupErr == nil {
		if err = tx.Commit(ctx); err != nil {
			return models.NotificationTask{}, false, err
		}
		return value, false, nil
	}
	if !errors.Is(lookupErr, pgx.ErrNoRows) {
		return models.NotificationTask{}, false, lookupErr
	}
	var consent bool
	var channels []string
	if err = tx.QueryRow(ctx, `SELECT marketing_consent,notification_channels FROM users WHERE id=$1 FOR UPDATE`, p.UserID).Scan(&consent, &channels); errors.Is(err, pgx.ErrNoRows) {
		return models.NotificationTask{}, false, ErrNotFound
	} else if err != nil {
		return models.NotificationTask{}, false, err
	}
	if !consent {
		return models.NotificationTask{}, false, ErrConsentDisabled
	}
	if !contains(channels, string(p.Channel)) {
		return models.NotificationTask{}, false, ErrChannelDisabled
	}
	var campaignCount, dailyCount int
	if p.CampaignID != nil {
		err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM notification_tasks WHERE user_id=$1 AND campaign_id=$2 AND status NOT IN ('failed','cancelled') AND created_at >= $3`, p.UserID, p.CampaignID, p.ScheduledAt.Add(-24*time.Hour)).Scan(&campaignCount)
		if err != nil {
			return models.NotificationTask{}, false, err
		}
	}
	if err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM notification_tasks WHERE user_id=$1 AND status NOT IN ('failed','cancelled') AND created_at >= $2`, p.UserID, p.ScheduledAt.Add(-24*time.Hour)).Scan(&dailyCount); err != nil {
		return models.NotificationTask{}, false, err
	}
	if campaignCount >= 1 || dailyCount >= 2 {
		return models.NotificationTask{}, false, ErrFrequencyLimited
	}
	payload, err := json.Marshal(p.Payload)
	if err != nil {
		return models.NotificationTask{}, false, err
	}
	query := `INSERT INTO notification_tasks(user_id,campaign_id,journey_type,journey_id,template_id,template_version,channel,idempotency_key,scheduled_at,next_attempt_at,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$9,$10) ON CONFLICT(idempotency_key) DO NOTHING RETURNING ` + taskColumns
	value, err = scanTask(tx.QueryRow(ctx, query, p.UserID, p.CampaignID, p.JourneyType, p.JourneyID, p.TemplateID, p.TemplateVersion, p.Channel, p.IdempotencyKey, p.ScheduledAt, payload))
	created := true
	if errors.Is(err, pgx.ErrNoRows) {
		created = false
		value, err = scanTask(tx.QueryRow(ctx, `SELECT `+taskColumns+` FROM notification_tasks WHERE idempotency_key=$1`, p.IdempotencyKey))
	}
	if err != nil {
		return models.NotificationTask{}, false, mapConflict(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return models.NotificationTask{}, false, err
	}
	return value, created, nil
}

func (s *PostgresStore) GetTask(ctx context.Context, id uuid.UUID) (models.NotificationTask, error) {
	value, err := scanTask(s.db.QueryRow(ctx, `SELECT `+taskColumns+` FROM notification_tasks WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	return value, err
}

func (s *PostgresStore) ListTasks(ctx context.Context, userID *uuid.UUID) ([]models.NotificationTask, error) {
	query, args := `SELECT `+taskColumns+` FROM notification_tasks`, []any{}
	if userID != nil {
		query += ` WHERE user_id=$1`
		args = append(args, *userID)
	}
	query += ` ORDER BY created_at DESC,id`
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []models.NotificationTask{}
	for rows.Next() {
		value, scanErr := scanTask(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *PostgresStore) ClaimTasks(ctx context.Context, now time.Time, processingTimeout time.Duration, limit int) ([]models.NotificationTask, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	query := `WITH candidates AS (
		SELECT n.id FROM notification_tasks n
		WHERE (((n.status IN ('pending','retry_scheduled') AND n.next_attempt_at <= $1 AND n.scheduled_at <= $1) OR (n.status='processing' AND n.processing_started_at <= $2)))
		AND (n.journey_type <> 'cart_recall' OR EXISTS (
			SELECT 1 FROM cart_recall_journeys j JOIN campaigns c ON c.id=j.campaign_id
			WHERE j.id=n.journey_id AND j.notification_task_id=n.id AND j.status='notification_pending'
			AND c.status IN ('scheduled','running') AND c.starts_at <= $1 AND c.ends_at > $1
			AND EXISTS (SELECT 1 FROM cart_items ci JOIN products p ON p.id=ci.product_id
				WHERE ci.cart_id=j.cart_id AND ci.product_id=ANY(j.matched_product_ids)
				AND p.status='active' AND p.stock >= ci.quantity)
		))
		ORDER BY n.next_attempt_at,n.id FOR UPDATE OF n SKIP LOCKED LIMIT $3
	) UPDATE notification_tasks n SET status='processing',processing_started_at=$1,attempt_count=n.attempt_count+1,updated_at=$1 FROM candidates c WHERE n.id=c.id RETURNING n.id,n.user_id,n.campaign_id,n.journey_type,n.journey_id,n.template_id,n.template_version,n.channel,n.status,n.idempotency_key,n.scheduled_at,n.attempt_count,n.next_attempt_at,n.sent_at,n.opened_at,COALESCE(n.failure_code,''),n.payload,n.created_at,n.updated_at`
	rows, err := tx.Query(ctx, query, now, now.Add(-processingTimeout), limit)
	if err != nil {
		return nil, err
	}
	result := []models.NotificationTask{}
	for rows.Next() {
		value, scanErr := scanTask(rows)
		if scanErr != nil {
			rows.Close()
			return nil, scanErr
		}
		result = append(result, value)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *PostgresStore) MarkSent(ctx context.Context, id uuid.UUID, now time.Time) error {
	return expectOne(s.db.Exec(ctx, `UPDATE notification_tasks SET status='delivered',sent_at=COALESCE(sent_at,$2),failure_code=NULL,processing_started_at=NULL,updated_at=$2 WHERE id=$1 AND status='processing'`, id, now))
}
func (s *PostgresStore) ScheduleRetry(ctx context.Context, id uuid.UUID, next time.Time, code string) error {
	return expectOne(s.db.Exec(ctx, `UPDATE notification_tasks SET status='retry_scheduled',next_attempt_at=$2,failure_code=$3,processing_started_at=NULL,updated_at=NOW() WHERE id=$1 AND status='processing'`, id, next, code))
}
func (s *PostgresStore) MarkFailed(ctx context.Context, id uuid.UUID, code string) error {
	return expectOne(s.db.Exec(ctx, `UPDATE notification_tasks SET status='failed',failure_code=$2,processing_started_at=NULL,updated_at=NOW() WHERE id=$1 AND status='processing'`, id, code))
}
func (s *PostgresStore) Retry(ctx context.Context, id uuid.UUID, now time.Time) error {
	return expectOne(s.db.Exec(ctx, `UPDATE notification_tasks SET status='retry_scheduled',next_attempt_at=$2,attempt_count=0,failure_code=NULL,updated_at=$2 WHERE id=$1 AND status='failed'`, id, now))
}
func (s *PostgresStore) Open(ctx context.Context, id, userID uuid.UUID, now time.Time) error {
	return expectOne(s.db.Exec(ctx, `UPDATE notification_tasks SET status='opened',opened_at=COALESCE(opened_at,$3),updated_at=$3 WHERE id=$1 AND user_id=$2 AND status IN ('sent','delivered','opened')`, id, userID, now))
}
func (s *PostgresStore) GetPreferences(ctx context.Context, userID uuid.UUID) (models.NotificationPreferences, error) {
	var value models.NotificationPreferences
	err := s.db.QueryRow(ctx, `SELECT marketing_consent,notification_channels FROM users WHERE id=$1`, userID).Scan(&value.MarketingConsent, &value.Channels)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	return value, err
}
func (s *PostgresStore) UpdatePreferences(ctx context.Context, userID uuid.UUID, value models.NotificationPreferences) (models.NotificationPreferences, error) {
	err := s.db.QueryRow(ctx, `UPDATE users SET marketing_consent=$2,notification_channels=$3,updated_at=NOW() WHERE id=$1 RETURNING marketing_consent,notification_channels`, userID, value.MarketingConsent, value.Channels).Scan(&value.MarketingConsent, &value.Channels)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	return value, err
}

func (s *PostgresStore) RecordDelivery(ctx context.Context, taskID uuid.UUID, idempotencyKey string, now time.Time) (bool, error) {
	tag, err := s.db.Exec(ctx, `INSERT INTO notification_delivery_receipts(task_id,idempotency_key,delivered_at) VALUES($1,$2,$3) ON CONFLICT(idempotency_key) DO NOTHING`, taskID, idempotencyKey, now)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

type scanner interface{ Scan(...any) error }

func scanTask(row scanner) (models.NotificationTask, error) {
	var value models.NotificationTask
	err := row.Scan(&value.ID, &value.UserID, &value.CampaignID, &value.JourneyType, &value.JourneyID, &value.TemplateID, &value.TemplateVersion, &value.Channel, &value.Status, &value.IdempotencyKey, &value.ScheduledAt, &value.AttemptCount, &value.NextAttemptAt, &value.SentAt, &value.OpenedAt, &value.FailureCode, &value.Payload, &value.CreatedAt, &value.UpdatedAt)
	return value, err
}
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func expectOne(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}
func mapConflict(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}
