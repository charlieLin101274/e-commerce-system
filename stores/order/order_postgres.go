package order

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
func (s *PostgresStore) Create(ctx context.Context, userID uuid.UUID, total int64) (models.Order, error) {
	var v models.Order
	err := s.db.QueryRow(ctx, `INSERT INTO orders(user_id,total_price,status) VALUES($1,$2,'completed') RETURNING id,user_id,total_price,status,created_at,updated_at`, userID, total).Scan(&v.ID, &v.UserID, &v.TotalPrice, &v.Status, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}
func (s *PostgresStore) CreateItems(ctx context.Context, orderID uuid.UUID, items []CreateItemParam) error {
	for _, v := range items {
		_, err := s.db.Exec(ctx, `INSERT INTO order_items(order_id,product_id,product_name,price,quantity) VALUES($1,$2,$3,$4,$5)`, orderID, v.ProductID, v.ProductName, v.Price, v.Quantity)
		if err != nil {
			return fmt.Errorf("create order item: %w", err)
		}
	}
	return nil
}
func (s *PostgresStore) List(ctx context.Context, userID uuid.UUID) ([]models.Order, error) {
	rows, err := s.db.Query(ctx, `SELECT id,user_id,total_price,status,created_at,updated_at FROM orders WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.Order{}
	for rows.Next() {
		var v models.Order
		if err := rows.Scan(&v.ID, &v.UserID, &v.TotalPrice, &v.Status, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *PostgresStore) Get(ctx context.Context, userID, orderID uuid.UUID) (models.Order, []models.OrderItem, error) {
	var v models.Order
	err := s.db.QueryRow(ctx, `SELECT id,user_id,total_price,status,created_at,updated_at FROM orders WHERE id=$1 AND user_id=$2`, orderID, userID).Scan(&v.ID, &v.UserID, &v.TotalPrice, &v.Status, &v.CreatedAt, &v.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, nil, ErrNotFound
	}
	if err != nil {
		return v, nil, err
	}
	rows, err := s.db.Query(ctx, `SELECT id,order_id,product_id,product_name,price,quantity,created_at FROM order_items WHERE order_id=$1 ORDER BY created_at`, orderID)
	if err != nil {
		return v, nil, err
	}
	defer rows.Close()
	items := []models.OrderItem{}
	for rows.Next() {
		var i models.OrderItem
		if err := rows.Scan(&i.ID, &i.OrderID, &i.ProductID, &i.ProductName, &i.Price, &i.Quantity, &i.CreatedAt); err != nil {
			return v, nil, err
		}
		items = append(items, i)
	}
	return v, items, rows.Err()
}
