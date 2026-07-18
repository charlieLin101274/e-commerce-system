package campaign

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/linenxing/e-commerce-system/models"
)

type PostgresStore struct{ db *pgxpool.Pool }

func NewPostgresStore(db *pgxpool.Pool) *PostgresStore { return &PostgresStore{db: db} }

const campaignColumns = `c.id,c.name,c.description,c.status,c.priority,c.starts_at,c.ends_at,
c.promotion_title,c.promotion_description,c.benefit_type,c.benefit_value,c.maximum_discount_amount,
c.created_by,c.created_at,c.updated_at,c.published_at,
COALESCE((SELECT array_agg(cp.product_id ORDER BY cp.product_id) FROM campaign_products cp WHERE cp.campaign_id=c.id), ARRAY[]::uuid[]),
COALESCE((SELECT array_agg(cc.category ORDER BY cc.category) FROM campaign_categories cc WHERE cc.campaign_id=c.id), ARRAY[]::varchar[])`

func (s *PostgresStore) Create(ctx context.Context, value models.Campaign) (models.Campaign, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return models.Campaign{}, fmt.Errorf("begin create campaign: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `INSERT INTO campaigns (name,description,status,priority,starts_at,ends_at,promotion_title,promotion_description,benefit_type,benefit_value,maximum_discount_amount,created_by)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id,created_at,updated_at`
	if err := tx.QueryRow(ctx, query, value.Name, value.Description, value.Status, value.Priority, value.StartsAt, value.EndsAt, value.PromotionTitle, value.PromotionDescription, value.BenefitType, value.BenefitValue, value.MaximumDiscountAmount, value.CreatedBy).Scan(&value.ID, &value.CreatedAt, &value.UpdatedAt); err != nil {
		return models.Campaign{}, fmt.Errorf("insert campaign: %w", err)
	}
	if err := insertScopes(ctx, tx, value); err != nil {
		return models.Campaign{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Campaign{}, fmt.Errorf("commit create campaign: %w", err)
	}
	return value, nil
}

func (s *PostgresStore) Update(ctx context.Context, value models.Campaign) (models.Campaign, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return models.Campaign{}, fmt.Errorf("begin update campaign: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	const query = `UPDATE campaigns SET name=$2,description=$3,status=$4,priority=$5,starts_at=$6,ends_at=$7,promotion_title=$8,promotion_description=$9,benefit_type=$10,benefit_value=$11,maximum_discount_amount=$12,published_at=$13,updated_at=NOW() WHERE id=$1 AND updated_at=$14 RETURNING updated_at`
	if err := tx.QueryRow(ctx, query, value.ID, value.Name, value.Description, value.Status, value.Priority, value.StartsAt, value.EndsAt, value.PromotionTitle, value.PromotionDescription, value.BenefitType, value.BenefitValue, value.MaximumDiscountAmount, value.PublishedAt, value.UpdatedAt).Scan(&value.UpdatedAt); errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if checkErr := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM campaigns WHERE id=$1)`, value.ID).Scan(&exists); checkErr != nil {
			return models.Campaign{}, fmt.Errorf("check campaign after update conflict: %w", checkErr)
		}
		if exists {
			return models.Campaign{}, ErrConflict
		}
		return models.Campaign{}, ErrNotFound
	} else if err != nil {
		return models.Campaign{}, fmt.Errorf("update campaign: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM campaign_products WHERE campaign_id=$1`, value.ID); err != nil {
		return models.Campaign{}, fmt.Errorf("replace campaign products: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM campaign_categories WHERE campaign_id=$1`, value.ID); err != nil {
		return models.Campaign{}, fmt.Errorf("replace campaign categories: %w", err)
	}
	if err := insertScopes(ctx, tx, value); err != nil {
		return models.Campaign{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return models.Campaign{}, fmt.Errorf("commit update campaign: %w", err)
	}
	return value, nil
}

func insertScopes(ctx context.Context, tx pgx.Tx, value models.Campaign) error {
	for _, productID := range value.ProductIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO campaign_products (campaign_id,product_id) VALUES ($1,$2)`, value.ID, productID); err != nil {
			return fmt.Errorf("insert campaign product: %w", err)
		}
	}
	for _, category := range value.Categories {
		if _, err := tx.Exec(ctx, `INSERT INTO campaign_categories (campaign_id,category) VALUES ($1,$2)`, value.ID, category); err != nil {
			return fmt.Errorf("insert campaign category: %w", err)
		}
	}
	return nil
}

func (s *PostgresStore) GetByID(ctx context.Context, id uuid.UUID) (models.Campaign, error) {
	value, err := scanCampaign(s.db.QueryRow(ctx, `SELECT `+campaignColumns+` FROM campaigns c WHERE c.id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Campaign{}, ErrNotFound
	}
	return value, err
}

func (s *PostgresStore) List(ctx context.Context) ([]models.Campaign, error) {
	rows, err := s.db.Query(ctx, `SELECT `+campaignColumns+` FROM campaigns c ORDER BY c.priority DESC,c.id`)
	if err != nil {
		return nil, fmt.Errorf("list campaigns: %w", err)
	}
	defer rows.Close()
	result := make([]models.Campaign, 0)
	for rows.Next() {
		value, err := scanCampaign(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *PostgresStore) GetProductCategories(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	result := make(map[uuid.UUID]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	rows, err := s.db.Query(ctx, `SELECT id,category FROM products WHERE id=ANY($1) AND status='active'`, ids)
	if err != nil {
		return nil, fmt.Errorf("get product categories: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id uuid.UUID
		var category string
		if err := rows.Scan(&id, &category); err != nil {
			return nil, fmt.Errorf("scan product category: %w", err)
		}
		result[id] = category
	}
	return result, rows.Err()
}

type scanner interface{ Scan(...any) error }

func scanCampaign(row scanner) (models.Campaign, error) {
	var value models.Campaign
	err := row.Scan(&value.ID, &value.Name, &value.Description, &value.Status, &value.Priority, &value.StartsAt, &value.EndsAt, &value.PromotionTitle, &value.PromotionDescription, &value.BenefitType, &value.BenefitValue, &value.MaximumDiscountAmount, &value.CreatedBy, &value.CreatedAt, &value.UpdatedAt, &value.PublishedAt, &value.ProductIDs, &value.Categories)
	return value, err
}
