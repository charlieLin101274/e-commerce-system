package user

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

var (
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrNotFound           = errors.New("user not found")
)

type CreateParams struct {
	Email        string
	PasswordHash string
	Name         string
	Role         models.UserRole
}

type Store interface {
	Create(ctx context.Context, params CreateParams) (models.User, error)
	GetByEmail(ctx context.Context, email string) (models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (models.User, error)
}
