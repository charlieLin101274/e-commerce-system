package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

type RegisterParam struct {
	Email    string
	Password string
	Name     string
}

type LoginParam struct {
	Email    string
	Password string
}

type UserOutput struct {
	ID        uuid.UUID       `json:"id"`
	Email     string          `json:"email"`
	Name      string          `json:"name"`
	Role      models.UserRole `json:"role"`
	CreatedAt string          `json:"created_at"`
}

type AuthOutput struct {
	AccessToken string     `json:"access_token"`
	TokenType   string     `json:"token_type"`
	User        UserOutput `json:"user"`
}

type Service interface {
	Register(ctx context.Context, input RegisterParam) (AuthOutput, error)
	Login(ctx context.Context, input LoginParam) (AuthOutput, error)
	GetCurrentUser(ctx context.Context, userID uuid.UUID) (UserOutput, error)
}
