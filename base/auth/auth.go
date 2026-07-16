package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}

type TokenManager interface {
	Generate(ctx context.Context, userID uuid.UUID, role string) (string, error)
	Verify(ctx context.Context, token string) (Claims, error)
}

type PasswordManager interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type JWTManager struct {
	secret     []byte
	issuer     string
	expiration time.Duration
}

func NewJWTManager(secret, issuer string, expiration time.Duration) *JWTManager {
	return &JWTManager{
		secret:     []byte(secret),
		issuer:     issuer,
		expiration: expiration,
	}
}

func (m *JWTManager) Generate(_ context.Context, userID uuid.UUID, role string) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.expiration)),
		},
	}

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign JWT: %w", err)
	}
	return token, nil
}

func (m *JWTManager) Verify(_ context.Context, rawToken string) (Claims, error) {
	token, err := jwt.ParseWithClaims(
		rawToken,
		&Claims{},
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidToken
			}
			return m.secret, nil
		},
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !token.Valid {
		return Claims{}, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok {
		return Claims{}, ErrInvalidToken
	}
	return *claims, nil
}

type BcryptPasswordManager struct {
	cost int
}

func NewBcryptPasswordManager() *BcryptPasswordManager {
	return &BcryptPasswordManager{cost: bcrypt.DefaultCost}
}

func (m *BcryptPasswordManager) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), m.cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func (m *BcryptPasswordManager) Compare(hash, password string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return errors.New("password does not match")
	}
	return nil
}
