package cart

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

var ErrNotFound = errors.New("cart item not found")

type ItemWithProduct struct {
	Item    models.CartItem
	Product models.Product
}
type Store interface {
	GetOrCreate(context.Context, uuid.UUID) (models.Cart, error)
	ListItems(context.Context, uuid.UUID, bool) ([]ItemWithProduct, error)
	UpsertItem(context.Context, uuid.UUID, uuid.UUID, int64) (models.CartItem, error)
	UpdateItem(context.Context, uuid.UUID, uuid.UUID, int64) (models.CartItem, error)
	DeleteItem(context.Context, uuid.UUID, uuid.UUID) error
	Clear(context.Context, uuid.UUID) error
}
