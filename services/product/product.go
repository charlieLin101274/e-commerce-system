package product

import (
	"context"
	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

type CreateParam struct {
	Name, Description, Category string
	Price, Stock                int64
}
type UpdateParam struct {
	ID                          uuid.UUID
	Name, Description, Category string
	Price, Stock                int64
	Status                      models.ProductStatus
}
type Service interface {
	Create(context.Context, CreateParam) (models.ProductResp, error)
	Update(context.Context, UpdateParam) (models.ProductResp, error)
	Disable(context.Context, uuid.UUID) error
	List(context.Context, bool) ([]models.ProductResp, error)
	Get(context.Context, uuid.UUID, bool) (models.ProductResp, error)
}
