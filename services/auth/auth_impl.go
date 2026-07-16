package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/base/apperror"
	baseauth "github.com/linenxing/e-commerce-system/base/auth"
	"github.com/linenxing/e-commerce-system/models"
	userstore "github.com/linenxing/e-commerce-system/stores/user"
)

const minimumPasswordLength = 8

type service struct {
	userStore       userstore.Store
	tokenManager    baseauth.TokenManager
	passwordManager baseauth.PasswordManager
}

func New(
	userStore userstore.Store,
	tokenManager baseauth.TokenManager,
	passwordManager baseauth.PasswordManager,
) Service {
	return &service{
		userStore:       userStore,
		tokenManager:    tokenManager,
		passwordManager: passwordManager,
	}
}

func (s *service) Register(ctx context.Context, input RegisterParam) (AuthOutput, error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Name = strings.TrimSpace(input.Name)

	if input.Email == "" || !strings.Contains(input.Email, "@") || input.Name == "" {
		return AuthOutput{}, apperror.ErrInvalidInput
	}
	if len(input.Password) < minimumPasswordLength {
		return AuthOutput{}, apperror.ErrInvalidInput
	}

	passwordHash, err := s.passwordManager.Hash(input.Password)
	if err != nil {
		return AuthOutput{}, fmt.Errorf("hash user password: %w", err)
	}

	user, err := s.userStore.Create(ctx, userstore.CreateParams{
		Email:        input.Email,
		PasswordHash: passwordHash,
		Name:         input.Name,
		Role:         models.UserRoleCustomer,
	})
	if errors.Is(err, userstore.ErrEmailAlreadyExists) {
		return AuthOutput{}, apperror.ErrConflict
	}
	if err != nil {
		return AuthOutput{}, fmt.Errorf("create user: %w", err)
	}

	return s.buildAuthOutput(ctx, user)
}

func (s *service) Login(ctx context.Context, input LoginParam) (AuthOutput, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	user, err := s.userStore.GetByEmail(ctx, email)
	if errors.Is(err, userstore.ErrNotFound) {
		return AuthOutput{}, apperror.ErrUnauthorized
	}
	if err != nil {
		return AuthOutput{}, fmt.Errorf("get login user: %w", err)
	}

	if err := s.passwordManager.Compare(user.PasswordHash, input.Password); err != nil {
		return AuthOutput{}, apperror.ErrUnauthorized
	}

	return s.buildAuthOutput(ctx, user)
}

func (s *service) GetCurrentUser(ctx context.Context, userID uuid.UUID) (UserOutput, error) {
	user, err := s.userStore.GetByID(ctx, userID)
	if errors.Is(err, userstore.ErrNotFound) {
		return UserOutput{}, apperror.ErrNotFound
	}
	if err != nil {
		return UserOutput{}, fmt.Errorf("get current user: %w", err)
	}
	return toUserOutput(user), nil
}

func (s *service) buildAuthOutput(ctx context.Context, user models.User) (AuthOutput, error) {
	accessToken, err := s.tokenManager.Generate(ctx, user.ID, string(user.Role))
	if err != nil {
		return AuthOutput{}, fmt.Errorf("generate access token: %w", err)
	}

	return AuthOutput{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		User:        toUserOutput(user),
	}, nil
}

func toUserOutput(user models.User) UserOutput {
	return UserOutput{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		CreatedAt: user.CreatedAt.UTC().Format(time.RFC3339),
	}
}
