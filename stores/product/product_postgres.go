package product

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

func (s *PostgresStore) Create(ctx context.Context, p CreateParam) (models.Product, error) {
	const q = `INSERT INTO products (name,description,price,stock,status) VALUES ($1,$2,$3,$4,$5) RETURNING id,name,description,price,stock,status,created_at,updated_at`
	return scan(s.db.QueryRow(ctx, q, p.Name, p.Description, p.Price, p.Stock, p.Status))
}
func (s *PostgresStore) Update(ctx context.Context, p UpdateParam) (models.Product, error) {
	const q = `UPDATE products SET name=$2,description=$3,price=$4,stock=$5,status=$6,updated_at=NOW() WHERE id=$1 RETURNING id,name,description,price,stock,status,created_at,updated_at`
	v, err := scan(s.db.QueryRow(ctx, q, p.ID, p.Name, p.Description, p.Price, p.Stock, p.Status))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Product{}, ErrNotFound
	}
	return v, err
}
func (s *PostgresStore) GetByID(ctx context.Context, id uuid.UUID, lock bool) (models.Product, error) {
	q := `SELECT id,name,description,price,stock,status,created_at,updated_at FROM products WHERE id=$1`
	if lock {
		q += ` FOR UPDATE`
	}
	v, err := scan(s.db.QueryRow(ctx, q, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Product{}, ErrNotFound
	}
	return v, err
}
func (s *PostgresStore) List(ctx context.Context, includeInactive bool) ([]models.Product, error) {
	q := `SELECT id,name,description,price,stock,status,created_at,updated_at FROM products`
	if !includeInactive {
		q += ` WHERE status='active'`
	}
	q += ` ORDER BY created_at DESC`
	rows, err := s.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()
	out := []models.Product{}
	for rows.Next() {
		v, e := scan(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s *PostgresStore) DecreaseStock(ctx context.Context, id uuid.UUID, quantity int64) error {
	tag, err := s.db.Exec(ctx, `UPDATE products SET stock=stock-$2,updated_at=NOW() WHERE id=$1 AND stock >= $2`, id, quantity)
	if err != nil {
		return fmt.Errorf("decrease product stock: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrNotFound
	}
	return nil
}

type scanner interface{ Scan(...any) error }

func scan(r scanner) (models.Product, error) {
	var v models.Product
	err := r.Scan(&v.ID, &v.Name, &v.Description, &v.Price, &v.Stock, &v.Status, &v.CreatedAt, &v.UpdatedAt)
	return v, err
}
