package models

import (
	"time"

	"github.com/google/uuid"
)

type UserRole string

const (
	UserRoleCustomer UserRole = "customer"
	UserRoleAdmin    UserRole = "admin"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Name         string
	Role         UserRole
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
