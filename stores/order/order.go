package order

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

var ErrNotFound = errors.New("order not found")

type CreateItemParam struct {
	ProductID       uuid.UUID
	ProductName     string
	Price, Quantity int64
}
type Store interface {
	Create(context.Context, uuid.UUID, int64) (models.Order, error)
	CreateItems(context.Context, uuid.UUID, []CreateItemParam) error
	List(context.Context, uuid.UUID) ([]models.Order, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (models.Order, []models.OrderItem, error)
}
