package order

import (
	"context"
	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

type Service interface {
	Create(context.Context, uuid.UUID) (models.OrderResp, error)
	List(context.Context, uuid.UUID) ([]models.OrderResp, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (models.OrderResp, error)
}
