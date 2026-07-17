package models

import (
	"time"

	"github.com/google/uuid"
)

type OrderStatus string

const OrderStatusCompleted OrderStatus = "completed"

type Order struct {
	ID         uuid.UUID   `db:"id"`
	UserID     uuid.UUID   `db:"user_id"`
	TotalPrice int64       `db:"total_price"`
	Status     OrderStatus `db:"status"`
	CreatedAt  time.Time   `db:"created_at"`
	UpdatedAt  time.Time   `db:"updated_at"`
}

type OrderItem struct {
	ID          uuid.UUID `db:"id"`
	OrderID     uuid.UUID `db:"order_id"`
	ProductID   uuid.UUID `db:"product_id"`
	ProductName string    `db:"product_name"`
	Price       int64     `db:"price"`
	Quantity    int64     `db:"quantity"`
	CreatedAt   time.Time `db:"created_at"`
}

type OrderItemResp struct {
	ProductID   uuid.UUID `json:"product_id"`
	ProductName string    `json:"product_name"`
	Price       int64     `json:"price"`
	Quantity    int64     `json:"quantity"`
	Subtotal    int64     `json:"subtotal"`
}

type OrderResp struct {
	ID         uuid.UUID       `json:"id"`
	TotalPrice int64           `json:"total_price"`
	Status     OrderStatus     `json:"status"`
	Items      []OrderItemResp `json:"items,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}
