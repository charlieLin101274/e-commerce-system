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
	ID           uuid.UUID `db:"id"`
	Email        string    `db:"email"`
	PasswordHash string    `db:"password_hash"`
	Name         string    `db:"name"`
	Role         UserRole  `db:"role"`
	MemberLevel  string    `db:"member_level"`
	MemberTags   []string  `db:"member_tags"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

type UserResp struct {
	ID        uuid.UUID `json:"id" format:"uuid"`
	Email     string    `json:"email" example:"customer@example.com"`
	Name      string    `json:"name" example:"Customer"`
	Role      UserRole  `json:"role" enums:"customer,admin"`
	CreatedAt time.Time `json:"created_at" format:"date-time"`
}
