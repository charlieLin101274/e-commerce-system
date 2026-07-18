package product

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

var ErrNotFound = errors.New("product not found")

type CreateParam struct {
	Name, Description, Category string
	Price, Stock                int64
	Status                      models.ProductStatus
}
type UpdateParam struct {
	ID                          uuid.UUID
	Name, Description, Category string
	Price, Stock                int64
	Status                      models.ProductStatus
}

type Store interface {
	Create(context.Context, CreateParam) (models.Product, error)
	Update(context.Context, UpdateParam) (models.Product, error)
	GetByID(context.Context, uuid.UUID, bool) (models.Product, error)
	List(context.Context, bool) ([]models.Product, error)
	DecreaseStock(context.Context, uuid.UUID, int64) error
}
