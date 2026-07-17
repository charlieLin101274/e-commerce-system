package models

import (
	"time"

	"github.com/google/uuid"
)

type ProductStatus string

const (
	ProductStatusActive   ProductStatus = "active"
	ProductStatusInactive ProductStatus = "inactive"
)

type Product struct {
	ID          uuid.UUID     `db:"id"`
	Name        string        `db:"name"`
	Description string        `db:"description"`
	Price       int64         `db:"price"`
	Stock       int64         `db:"stock"`
	Status      ProductStatus `db:"status"`
	CreatedAt   time.Time     `db:"created_at"`
	UpdatedAt   time.Time     `db:"updated_at"`
}

type ProductResp struct {
	ID          uuid.UUID     `json:"id" format:"uuid"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Price       int64         `json:"price" example:"1000"`
	Stock       int64         `json:"stock" example:"10"`
	Status      ProductStatus `json:"status" enums:"active,inactive"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}
