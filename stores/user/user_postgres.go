package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	basepostgres "github.com/linenxing/e-commerce-system/base/postgres"
	"github.com/linenxing/e-commerce-system/models"
)

const uniqueViolationCode = "23505"

type PostgresStore struct {
	db basepostgres.DBTX
}

func NewPostgresStore(db basepostgres.DBTX) *PostgresStore {
	return &PostgresStore{db: db}
}

func (s *PostgresStore) Create(ctx context.Context, params CreateParams) (models.User, error) {
	const query = `
		INSERT INTO users (email, password_hash, name, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, password_hash, name, role, member_level, member_tags, created_at, updated_at`

	user, err := scanUser(s.db.QueryRow(
		ctx,
		query,
		params.Email,
		params.PasswordHash,
		params.Name,
		params.Role,
	))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode {
			return models.User{}, ErrEmailAlreadyExists
		}
		return models.User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (s *PostgresStore) GetByEmail(ctx context.Context, email string) (models.User, error) {
	const query = `
		SELECT id, email, password_hash, name, role, member_level, member_tags, created_at, updated_at
		FROM users
		WHERE email = $1`

	user, err := scanUser(s.db.QueryRow(ctx, query, email))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

func (s *PostgresStore) GetByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	const query = `
		SELECT id, email, password_hash, name, role, member_level, member_tags, created_at, updated_at
		FROM users
		WHERE id = $1`

	user, err := scanUser(s.db.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, ErrNotFound
	}
	if err != nil {
		return models.User{}, fmt.Errorf("get user by ID: %w", err)
	}
	return user, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (models.User, error) {
	var user models.User
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Name,
		&user.Role,
		&user.MemberLevel,
		&user.MemberTags,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	return user, err
}
