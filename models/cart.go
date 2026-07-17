package models

import (
	"time"

	"github.com/google/uuid"
)

type Cart struct {
	ID        uuid.UUID `db:"id"`
	UserID    uuid.UUID `db:"user_id"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type CartItem struct {
	ID        uuid.UUID `db:"id"`
	CartID    uuid.UUID `db:"cart_id"`
	ProductID uuid.UUID `db:"product_id"`
	Quantity  int64     `db:"quantity"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type CartItemResp struct {
	ID       uuid.UUID   `json:"id"`
	Product  ProductResp `json:"product"`
	Quantity int64       `json:"quantity"`
	Subtotal int64       `json:"subtotal"`
}

type CartResp struct {
	ID         uuid.UUID      `json:"id"`
	Items      []CartItemResp `json:"items"`
	TotalPrice int64          `json:"total_price"`
}
