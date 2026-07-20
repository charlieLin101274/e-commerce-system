package cart

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	basepostgres "github.com/linenxing/e-commerce-system/base/postgres"
	"github.com/linenxing/e-commerce-system/models"
)

type PostgresStore struct{ db basepostgres.DBTX }

func NewPostgresStore(db basepostgres.DBTX) *PostgresStore { return &PostgresStore{db: db} }
func (s *PostgresStore) GetOrCreate(ctx context.Context, userID uuid.UUID) (models.Cart, error) {
	const q = `INSERT INTO carts(user_id) VALUES($1) ON CONFLICT(user_id) DO UPDATE SET user_id=EXCLUDED.user_id RETURNING id,user_id,created_at,updated_at`
	var v models.Cart
	err := s.db.QueryRow(ctx, q, userID).Scan(&v.ID, &v.UserID, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}
func (s *PostgresStore) ListItems(ctx context.Context, cartID uuid.UUID, lock bool) ([]ItemWithProduct, error) {
	q := `SELECT ci.id,ci.cart_id,ci.product_id,ci.quantity,ci.created_at,ci.updated_at,p.id,p.name,p.description,p.price,p.stock,p.status,p.created_at,p.updated_at FROM cart_items ci JOIN products p ON p.id=ci.product_id WHERE ci.cart_id=$1 ORDER BY ci.created_at`
	if lock {
		q += ` FOR UPDATE OF ci,p`
	}
	rows, err := s.db.Query(ctx, q, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ItemWithProduct{}
	for rows.Next() {
		var v ItemWithProduct
		err = rows.Scan(&v.Item.ID, &v.Item.CartID, &v.Item.ProductID, &v.Item.Quantity, &v.Item.CreatedAt, &v.Item.UpdatedAt, &v.Product.ID, &v.Product.Name, &v.Product.Description, &v.Product.Price, &v.Product.Stock, &v.Product.Status, &v.Product.CreatedAt, &v.Product.UpdatedAt)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *PostgresStore) UpsertItem(ctx context.Context, cartID, productID uuid.UUID, quantity int64) (models.CartItem, error) {
	const q = `WITH changed AS (
		INSERT INTO cart_items(cart_id,product_id,quantity) VALUES($1,$2,$3)
		ON CONFLICT(cart_id,product_id) DO UPDATE SET quantity=cart_items.quantity+EXCLUDED.quantity,updated_at=NOW()
		RETURNING id,cart_id,product_id,quantity,created_at,updated_at
	), touched AS (
		UPDATE carts SET updated_at=NOW() WHERE id=$1
	), emitted AS (
		INSERT INTO domain_outbox(event_type,aggregate_id,payload)
		SELECT 'cart.item_added',cart_id,jsonb_build_object('cart_id',cart_id,'product_id',product_id) FROM changed
	)
	SELECT id,cart_id,product_id,quantity,created_at,updated_at FROM changed`
	return scanItem(s.db.QueryRow(ctx, q, cartID, productID, quantity))
}
func (s *PostgresStore) UpdateItem(ctx context.Context, cartID, itemID uuid.UUID, quantity int64) (models.CartItem, error) {
	const q = `WITH changed AS (
		UPDATE cart_items SET quantity=$3,updated_at=NOW() WHERE cart_id=$1 AND id=$2
		RETURNING id,cart_id,product_id,quantity,created_at,updated_at
	), touched AS (
		UPDATE carts SET updated_at=NOW() WHERE id=$1
	), emitted AS (
		INSERT INTO domain_outbox(event_type,aggregate_id,payload)
		SELECT 'cart.item_added',cart_id,jsonb_build_object('cart_id',cart_id,'product_id',product_id) FROM changed
	)
	SELECT id,cart_id,product_id,quantity,created_at,updated_at FROM changed`
	v, err := scanItem(s.db.QueryRow(ctx, q, cartID, itemID, quantity))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.CartItem{}, ErrNotFound
	}
	return v, err
}
func (s *PostgresStore) DeleteItem(ctx context.Context, cartID, itemID uuid.UUID) error {
	tag, err := s.db.Exec(ctx, `WITH removed AS (
		DELETE FROM cart_items WHERE cart_id=$1 AND id=$2 RETURNING cart_id,product_id
	), touched AS (
		UPDATE carts SET updated_at=NOW() WHERE id=$1 AND EXISTS(SELECT 1 FROM removed)
	) INSERT INTO domain_outbox(event_type,aggregate_id,payload)
	SELECT 'cart.item_removed',cart_id,jsonb_build_object('cart_id',cart_id,'product_id',product_id) FROM removed`, cartID, itemID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}
func (s *PostgresStore) Clear(ctx context.Context, cartID uuid.UUID) error {
	_, err := s.db.Exec(ctx, `DELETE FROM cart_items WHERE cart_id=$1`, cartID)
	return err
}

type scanner interface{ Scan(...any) error }

func scanItem(r scanner) (models.CartItem, error) {
	var v models.CartItem
	err := r.Scan(&v.ID, &v.CartID, &v.ProductID, &v.Quantity, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return v, fmt.Errorf("scan cart item: %w", err)
	}
	return v, nil
}
