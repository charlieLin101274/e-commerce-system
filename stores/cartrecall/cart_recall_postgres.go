package cartrecall

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/linenxing/e-commerce-system/base/logger"
	"github.com/linenxing/e-commerce-system/models"
)

const journeyColumns = `id,user_id,cart_id,source_event_id,status,evaluate_at,campaign_id,rule_version,matched_product_ids,matched_products_snapshot,notification_task_id,converted_order_id,COALESCE(cancel_reason,''),created_at,updated_at`
const journeyReturningColumns = `j.id,j.user_id,j.cart_id,j.source_event_id,j.status,j.evaluate_at,j.campaign_id,j.rule_version,j.matched_product_ids,j.matched_products_snapshot,j.notification_task_id,j.converted_order_id,COALESCE(j.cancel_reason,''),j.created_at,j.updated_at`

type PostgresStore struct{ db *pgxpool.Pool }

func NewPostgresStore(db *pgxpool.Pool) *PostgresStore { return &PostgresStore{db: db} }

func (s *PostgresStore) ConsumeEvents(ctx context.Context, now time.Time, delay time.Duration, limit int) (int, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `SELECT id,event_type,aggregate_id,payload,occurred_at FROM domain_outbox WHERE published_at IS NULL ORDER BY occurred_at,id FOR UPDATE SKIP LOCKED LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	events := []models.DomainEvent{}
	for rows.Next() {
		var event models.DomainEvent
		if err = rows.Scan(&event.ID, &event.Type, &event.AggregateID, &event.Payload, &event.OccurredAt); err != nil {
			rows.Close()
			return 0, err
		}
		events = append(events, event)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return 0, err
	}
	for _, event := range events {
		tag, insertErr := tx.Exec(ctx, `INSERT INTO event_inbox(consumer,event_id) VALUES('cart-recall-trigger',$1) ON CONFLICT DO NOTHING`, event.ID)
		if insertErr != nil {
			return 0, insertErr
		}
		if tag.RowsAffected() == 1 {
			switch event.Type {
			case "cart.item_added":
				if err = s.schedule(ctx, tx, event, now.Add(delay)); err != nil {
					return 0, err
				}
			case "cart.item_removed":
				if err = s.rescheduleExisting(ctx, tx, event, now.Add(delay)); err != nil {
					return 0, err
				}
				if err = s.cancelEmptyCart(ctx, tx, event.AggregateID); err != nil {
					return 0, err
				}
			case "order.completed":
				if err = s.completeOrder(ctx, tx, event); err != nil {
					return 0, err
				}
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE domain_outbox SET published_at=$2,processing_started_at=NULL WHERE id=$1`, event.ID, now); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(events), nil
}

func (s *PostgresStore) rescheduleExisting(ctx context.Context, tx pgx.Tx, event models.DomainEvent, evaluateAt time.Time) error {
	if _, err := tx.Exec(ctx, `UPDATE notification_tasks SET status='cancelled',failure_code='CART_CHANGED_RECENTLY',updated_at=NOW()
		WHERE id IN (SELECT notification_task_id FROM cart_recall_journeys WHERE cart_id=$1 AND status='notification_pending')
		AND status IN ('pending','processing','retry_scheduled')`, event.AggregateID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `UPDATE cart_recall_journeys SET source_event_id=$2,status='scheduled',evaluate_at=$3,campaign_id=NULL,rule_version=0,
		matched_product_ids=ARRAY[]::uuid[],matched_products_snapshot=NULL,notification_task_id=NULL,cancel_reason=NULL,processing_started_at=NULL,updated_at=NOW()
		WHERE cart_id=$1 AND status IN ('scheduled','evaluating','notification_pending')`, event.AggregateID, event.ID, evaluateAt)
	return err
}

func (s *PostgresStore) schedule(ctx context.Context, tx pgx.Tx, event models.DomainEvent, evaluateAt time.Time) error {
	var userID uuid.UUID
	if err := tx.QueryRow(ctx, `SELECT user_id FROM carts WHERE id=$1`, event.AggregateID).Scan(&userID); errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE notification_tasks SET status='cancelled',failure_code='CART_CHANGED_RECENTLY',updated_at=NOW()
		WHERE id IN (SELECT notification_task_id FROM cart_recall_journeys WHERE cart_id=$1 AND status='notification_pending')
		AND status IN ('pending','processing','retry_scheduled')`, event.AggregateID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO cart_recall_journeys(user_id,cart_id,source_event_id,evaluate_at) VALUES($1,$2,$3,$4)
		ON CONFLICT(cart_id) WHERE status IN ('scheduled','evaluating','notification_pending')
		DO UPDATE SET source_event_id=EXCLUDED.source_event_id,status='scheduled',evaluate_at=EXCLUDED.evaluate_at,campaign_id=NULL,rule_version=0,matched_product_ids=ARRAY[]::uuid[],matched_products_snapshot=NULL,notification_task_id=NULL,cancel_reason=NULL,processing_started_at=NULL,updated_at=NOW()`, userID, event.AggregateID, event.ID, evaluateAt)
	return err
}

func (s *PostgresStore) cancelEmptyCart(ctx context.Context, tx pgx.Tx, cartID uuid.UUID) error {
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM cart_items WHERE cart_id=$1)`, cartID).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return nil
	}
	rows, err := tx.Query(ctx, `WITH cancelled AS (
		UPDATE cart_recall_journeys SET status='cancelled',cancel_reason='CART_EMPTY',updated_at=NOW()
		WHERE cart_id=$1 AND status IN ('scheduled','evaluating','notification_pending')
		RETURNING source_event_id,id,campaign_id,notification_task_id,cancel_reason
	), tasks AS (UPDATE notification_tasks SET status='cancelled',failure_code='CART_EMPTY',updated_at=NOW()
		WHERE id IN (SELECT notification_task_id FROM cancelled) AND status IN ('pending','processing','retry_scheduled')
	) SELECT source_event_id,id,campaign_id,notification_task_id,cancel_reason FROM cancelled`, cartID)
	if err != nil {
		return err
	}
	_, err = logCancelledRows(ctx, rows)
	return err
}

func (s *PostgresStore) completeOrder(ctx context.Context, tx pgx.Tx, event models.DomainEvent) error {
	var payload struct {
		OrderID    uuid.UUID   `json:"order_id"`
		UserID     uuid.UUID   `json:"user_id"`
		ProductIDs []uuid.UUID `json:"product_ids"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `WITH candidates AS (
		SELECT j.id,(j.status='sent' AND n.sent_at IS NOT NULL AND n.sent_at <= $4
			AND n.sent_at >= $4 - INTERVAL '24 hours' AND j.matched_product_ids && $3::uuid[]) AS converts
		FROM cart_recall_journeys j LEFT JOIN notification_tasks n ON n.id=j.notification_task_id
		WHERE j.user_id=$1 AND j.status IN ('scheduled','evaluating','notification_pending','sent')
	), transitioned AS (UPDATE cart_recall_journeys j SET
		status=CASE WHEN candidate.converts THEN 'converted'::cart_recall_status ELSE 'cancelled'::cart_recall_status END,
		converted_order_id=CASE WHEN candidate.converts THEN $2 ELSE j.converted_order_id END,
		cancel_reason=CASE WHEN candidate.converts THEN NULL ELSE 'ORDER_ALREADY_COMPLETED' END,
		updated_at=$4 FROM candidates candidate WHERE j.id=candidate.id
		RETURNING j.id,j.campaign_id,j.notification_task_id,j.converted_order_id,j.matched_product_ids,j.status
	), cancelled_tasks AS (UPDATE notification_tasks SET status='cancelled',failure_code='ORDER_ALREADY_COMPLETED',updated_at=$4
		WHERE id IN (SELECT notification_task_id FROM transitioned WHERE status='cancelled') AND status IN ('pending','processing','retry_scheduled')
	) SELECT id,campaign_id,notification_task_id,converted_order_id,matched_product_ids,status FROM transitioned`, payload.UserID, payload.OrderID, payload.ProductIDs, event.OccurredAt)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var journeyID uuid.UUID
		var campaignID, taskID, orderID *uuid.UUID
		var productIDs []uuid.UUID
		var status models.CartRecallStatus
		if err = rows.Scan(&journeyID, &campaignID, &taskID, &orderID, &productIDs, &status); err != nil {
			return err
		}
		if status == models.CartRecallConverted {
			logger.FromContext(ctx).Info().Str("event_id", event.ID.String()).Str("campaign_id", optionalID(campaignID)).Str("journey_type", "cart_recall").Str("journey_id", journeyID.String()).Str("notification_task_id", optionalID(taskID)).Str("order_id", optionalID(orderID)).Interface("matched_product_ids", productIDs).Str("decision", "converted").Msg("cart recall converted")
		} else {
			logStoreDecision(ctx, event.ID, journeyID, campaignID, taskID, "cancelled", "ORDER_ALREADY_COMPLETED")
		}
	}
	return rows.Err()
}

func (s *PostgresStore) ClaimDue(ctx context.Context, now time.Time, timeout time.Duration, limit int) ([]models.CartRecallJourney, error) {
	rows, err := s.db.Query(ctx, `WITH due AS (SELECT id FROM cart_recall_journeys WHERE (status='scheduled' AND evaluate_at <= $1) OR (status='evaluating' AND processing_started_at <= $2) ORDER BY evaluate_at,id FOR UPDATE SKIP LOCKED LIMIT $3)
		UPDATE cart_recall_journeys j SET status='evaluating',processing_started_at=$1,updated_at=$1 FROM due WHERE j.id=due.id RETURNING `+journeyReturningColumns, now, now.Add(-timeout), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []models.CartRecallJourney{}
	for rows.Next() {
		value, scanErr := scanJourney(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *PostgresStore) GetCartState(ctx context.Context, id uuid.UUID) (CartState, error) {
	var value CartState
	if err := s.db.QueryRow(ctx, `SELECT id,user_id,created_at,updated_at FROM carts WHERE id=$1`, id).Scan(&value.Cart.ID, &value.Cart.UserID, &value.Cart.CreatedAt, &value.Cart.UpdatedAt); errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	} else if err != nil {
		return value, err
	}
	value.LastChange = value.Cart.UpdatedAt
	rows, err := s.db.Query(ctx, `SELECT p.id,p.name,p.description,p.category,p.price,p.stock,p.status,p.created_at,p.updated_at,ci.quantity,ci.updated_at FROM cart_items ci JOIN products p ON p.id=ci.product_id WHERE ci.cart_id=$1 ORDER BY p.id`, id)
	if err != nil {
		return value, err
	}
	defer rows.Close()
	for rows.Next() {
		var item CartItemState
		var changed time.Time
		if err = rows.Scan(&item.Product.ID, &item.Product.Name, &item.Product.Description, &item.Product.Category, &item.Product.Price, &item.Product.Stock, &item.Product.Status, &item.Product.CreatedAt, &item.Product.UpdatedAt, &item.Quantity, &changed); err != nil {
			return value, err
		}
		if changed.After(value.LastChange) {
			value.LastChange = changed
		}
		value.Items = append(value.Items, item)
	}
	return value, rows.Err()
}

func (s *PostgresStore) GetMemberFacts(ctx context.Context, id uuid.UUID) (models.MemberFacts, error) {
	var value models.MemberFacts
	err := s.db.QueryRow(ctx, `SELECT id,member_level,member_tags FROM users WHERE id=$1`, id).Scan(&value.ID, &value.Level, &value.Tags)
	if errors.Is(err, pgx.ErrNoRows) {
		return value, ErrNotFound
	}
	return value, err
}

func (s *PostgresStore) MarkSkipped(ctx context.Context, id uuid.UUID, reason string) error {
	return updateOne(s.db.Exec(ctx, `UPDATE cart_recall_journeys SET status='skipped',cancel_reason=$2,processing_started_at=NULL,updated_at=NOW() WHERE id=$1 AND status='evaluating'`, id, reason))
}
func (s *PostgresStore) MarkCancelled(ctx context.Context, id uuid.UUID, reason string) error {
	rows, err := s.db.Query(ctx, `WITH cancelled AS (
		UPDATE cart_recall_journeys SET status='cancelled',cancel_reason=$2,processing_started_at=NULL,updated_at=NOW()
		WHERE id=$1 AND status IN ('scheduled','evaluating','notification_pending')
		RETURNING source_event_id,id,campaign_id,notification_task_id,cancel_reason
	), tasks AS (
		UPDATE notification_tasks SET status='cancelled',failure_code=$2,updated_at=NOW()
		WHERE id IN (SELECT notification_task_id FROM cancelled) AND status IN ('pending','processing','retry_scheduled')
	) SELECT source_event_id,id,campaign_id,notification_task_id,cancel_reason FROM cancelled`, id, reason)
	if err != nil {
		return err
	}
	count, err := logCancelledRows(ctx, rows)
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrConflict
	}
	return nil
}

func (s *PostgresStore) MarkNotificationPending(ctx context.Context, id uuid.UUID, campaign models.Campaign, snapshots []models.CartRecallProductSnapshot, taskID uuid.UUID) error {
	data, err := json.Marshal(snapshots)
	if err != nil {
		return err
	}
	ids := make([]uuid.UUID, 0, len(snapshots))
	for _, snapshot := range snapshots {
		ids = append(ids, snapshot.ProductID)
	}
	return updateOne(s.db.Exec(ctx, `UPDATE cart_recall_journeys SET status='notification_pending',campaign_id=$2,rule_version=$3,matched_product_ids=$4,matched_products_snapshot=$5,notification_task_id=$6,processing_started_at=NULL,updated_at=NOW() WHERE id=$1 AND status='evaluating'`, id, campaign.ID, campaign.RuleVersion, ids, data, taskID))
}

func (s *PostgresStore) CancelInvalidPending(ctx context.Context, now time.Time) (int, error) {
	rows, err := s.db.Query(ctx, `WITH invalid AS (
		SELECT j.id,j.notification_task_id,CASE
			WHEN c.id IS NULL OR c.status NOT IN ('scheduled','running') OR c.starts_at > $1 OR c.ends_at <= $1 THEN 'CAMPAIGN_ENDED'
			WHEN NOT EXISTS (SELECT 1 FROM cart_items ci JOIN products p ON p.id=ci.product_id WHERE ci.cart_id=j.cart_id AND ci.product_id=ANY(j.matched_product_ids) AND p.status='active') THEN 'PRODUCT_INACTIVE'
			ELSE 'OUT_OF_STOCK' END AS reason
		FROM cart_recall_journeys j LEFT JOIN campaigns c ON c.id=j.campaign_id
		WHERE j.status='notification_pending' AND (
			c.id IS NULL OR c.status NOT IN ('scheduled','running') OR c.starts_at > $1 OR c.ends_at <= $1 OR
			NOT EXISTS (SELECT 1 FROM cart_items ci JOIN products p ON p.id=ci.product_id WHERE ci.cart_id=j.cart_id AND ci.product_id=ANY(j.matched_product_ids) AND p.status='active' AND p.stock >= ci.quantity)
		)
	), tasks AS (
		UPDATE notification_tasks n SET status='cancelled',failure_code=i.reason,updated_at=$1 FROM invalid i
		WHERE n.id=i.notification_task_id AND n.status IN ('pending','processing','retry_scheduled')
	), cancelled AS (
		UPDATE cart_recall_journeys j SET status='cancelled',cancel_reason=i.reason,updated_at=$1 FROM invalid i WHERE j.id=i.id
		RETURNING j.source_event_id,j.id,j.campaign_id,j.notification_task_id,j.cancel_reason
	) SELECT source_event_id,id,campaign_id,notification_task_id,cancel_reason FROM cancelled`, now)
	if err != nil {
		return 0, err
	}
	count, err := logCancelledRows(ctx, rows)
	return count, err
}

func (s *PostgresStore) SyncDelivered(ctx context.Context, now time.Time) (int, error) {
	tag, err := s.db.Exec(ctx, `UPDATE cart_recall_journeys j SET status='sent',updated_at=COALESCE(n.sent_at,$1) FROM notification_tasks n WHERE j.notification_task_id=n.id AND j.status='notification_pending' AND n.status IN ('sent','delivered','opened')`, now)
	return int(tag.RowsAffected()), err
}
func (s *PostgresStore) List(ctx context.Context) ([]models.CartRecallJourney, error) {
	rows, err := s.db.Query(ctx, `SELECT `+journeyColumns+` FROM cart_recall_journeys ORDER BY created_at DESC,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.CartRecallJourney{}
	for rows.Next() {
		v, e := scanJourney(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *PostgresStore) Get(ctx context.Context, id uuid.UUID) (models.CartRecallJourney, error) {
	v, err := scanJourney(s.db.QueryRow(ctx, `SELECT `+journeyColumns+` FROM cart_recall_journeys WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, err
}
func (s *PostgresStore) Cancel(ctx context.Context, id uuid.UUID, reason string) error {
	return s.MarkCancelled(ctx, id, reason)
}

type scanner interface{ Scan(...any) error }

func scanJourney(row scanner) (models.CartRecallJourney, error) {
	var v models.CartRecallJourney
	err := row.Scan(&v.ID, &v.UserID, &v.CartID, &v.SourceEventID, &v.Status, &v.EvaluateAt, &v.CampaignID, &v.RuleVersion, &v.MatchedProductIDs, &v.MatchedProductsSnapshot, &v.NotificationTaskID, &v.ConvertedOrderID, &v.CancelReason, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}
func updateOne(tag pgconn.CommandTag, err error) error {
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func optionalID(value *uuid.UUID) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func logCancelledRows(ctx context.Context, rows pgx.Rows) (int, error) {
	defer rows.Close()
	count := 0
	for rows.Next() {
		var eventID, journeyID uuid.UUID
		var campaignID, taskID *uuid.UUID
		var reason string
		if err := rows.Scan(&eventID, &journeyID, &campaignID, &taskID, &reason); err != nil {
			return count, err
		}
		count++
		logStoreDecision(ctx, eventID, journeyID, campaignID, taskID, "cancelled", reason)
	}
	return count, rows.Err()
}

func logStoreDecision(ctx context.Context, eventID, journeyID uuid.UUID, campaignID, taskID *uuid.UUID, decision, reason string) {
	logger.FromContext(ctx).Info().
		Str("event_id", eventID.String()).
		Str("campaign_id", optionalID(campaignID)).
		Str("journey_type", "cart_recall").
		Str("journey_id", journeyID.String()).
		Str("notification_task_id", optionalID(taskID)).
		Str("decision", decision).
		Str("reason_code", reason).
		Msg("cart recall decision")
}
