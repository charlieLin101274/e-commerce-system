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

type AuthOutput struct {
	AccessToken string          `json:"access_token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	TokenType   string          `json:"token_type" example:"Bearer"`
	User        models.UserResp `json:"user"`
}

type Service interface {
	Register(ctx context.Context, input RegisterParam) (AuthOutput, error)
	Login(ctx context.Context, input LoginParam) (AuthOutput, error)
	GetCurrentUser(ctx context.Context, userID uuid.UUID) (models.UserResp, error)
}
