package cart

import (
	"context"
	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

type AddItemParam struct {
	UserID, ProductID uuid.UUID
	Quantity          int64
}
type UpdateItemParam struct {
	UserID, ItemID uuid.UUID
	Quantity       int64
}
type RemoveItemParam struct{ UserID, ItemID uuid.UUID }
type Service interface {
	Get(context.Context, uuid.UUID) (models.CartResp, error)
	AddItem(context.Context, AddItemParam) (models.CartResp, error)
	UpdateItem(context.Context, UpdateItemParam) (models.CartResp, error)
	RemoveItem(context.Context, RemoveItemParam) error
}
