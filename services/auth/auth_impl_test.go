package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/base/apperror"
	baseauth "github.com/linenxing/e-commerce-system/base/auth"
	"github.com/linenxing/e-commerce-system/models"
	userstore "github.com/linenxing/e-commerce-system/stores/user"
)

type fakeUserStore struct {
	usersByID    map[uuid.UUID]models.User
	usersByEmail map[string]models.User
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{
		usersByID:    make(map[uuid.UUID]models.User),
		usersByEmail: make(map[string]models.User),
	}
}

func (s *fakeUserStore) Create(_ context.Context, params userstore.CreateParams) (models.User, error) {
	if _, exists := s.usersByEmail[params.Email]; exists {
		return models.User{}, userstore.ErrEmailAlreadyExists
	}

	now := time.Now().UTC()
	user := models.User{
		ID:           uuid.New(),
		Email:        params.Email,
		PasswordHash: params.PasswordHash,
		Name:         params.Name,
		Role:         params.Role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.usersByID[user.ID] = user
	s.usersByEmail[user.Email] = user
	return user, nil
}

func (s *fakeUserStore) GetByEmail(_ context.Context, email string) (models.User, error) {
	user, exists := s.usersByEmail[email]
	if !exists {
		return models.User{}, userstore.ErrNotFound
	}
	return user, nil
}

func (s *fakeUserStore) GetByID(_ context.Context, id uuid.UUID) (models.User, error) {
	user, exists := s.usersByID[id]
	if !exists {
		return models.User{}, userstore.ErrNotFound
	}
	return user, nil
}

func newTestService(store userstore.Store) Service {
	return New(
		store,
		baseauth.NewJWTManager(
			"test-secret-that-contains-at-least-32-characters",
			"test-suite",
			time.Hour,
		),
		baseauth.NewBcryptPasswordManager(),
	)
}

func TestServiceRegisterAndLogin(t *testing.T) {
	store := newFakeUserStore()
	service := newTestService(store)

	registered, err := service.Register(context.Background(), RegisterParam{
		Email:    " Customer@Example.com ",
		Password: "secure-password",
		Name:     "Customer",
	})
	if err != nil {
		t.Fatalf("register returned error: %v", err)
	}
	if registered.User.Email != "customer@example.com" {
		t.Fatalf("unexpected normalized email: %s", registered.User.Email)
	}
	if registered.User.Role != models.UserRoleCustomer {
		t.Fatalf("unexpected role: %s", registered.User.Role)
	}
	if registered.AccessToken == "" {
		t.Fatal("expected access token")
	}

	loggedIn, err := service.Login(context.Background(), LoginParam{
		Email:    "customer@example.com",
		Password: "secure-password",
	})
	if err != nil {
		t.Fatalf("login returned error: %v", err)
	}
	if loggedIn.User.ID != registered.User.ID {
		t.Fatalf("login returned a different user: %s", loggedIn.User.ID)
	}
}

func TestServiceRegisterDuplicateEmail(t *testing.T) {
	store := newFakeUserStore()
	service := newTestService(store)
	input := RegisterParam{
		Email:    "customer@example.com",
		Password: "secure-password",
		Name:     "Customer",
	}

	if _, err := service.Register(context.Background(), input); err != nil {
		t.Fatalf("first registration returned error: %v", err)
	}

	_, err := service.Register(context.Background(), input)
	if !errors.Is(err, apperror.ErrConflict) {
		t.Fatalf("expected conflict error, got: %v", err)
	}
}

func TestServiceLoginRejectsInvalidPassword(t *testing.T) {
	store := newFakeUserStore()
	service := newTestService(store)

	_, err := service.Register(context.Background(), RegisterParam{
		Email:    "customer@example.com",
		Password: "secure-password",
		Name:     "Customer",
	})
	if err != nil {
		t.Fatalf("register returned error: %v", err)
	}

	_, err = service.Login(context.Background(), LoginParam{
		Email:    "customer@example.com",
		Password: "wrong-password",
	})
	if !errors.Is(err, apperror.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got: %v", err)
	}
}
