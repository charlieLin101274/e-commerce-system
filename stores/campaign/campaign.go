package campaign

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

var ErrNotFound = errors.New("campaign not found")
var ErrConflict = errors.New("campaign was concurrently modified")

type Store interface {
	Create(context.Context, models.Campaign) (models.Campaign, error)
	Update(context.Context, models.Campaign) (models.Campaign, error)
	GetByID(context.Context, uuid.UUID) (models.Campaign, error)
	List(context.Context) ([]models.Campaign, error)
	GetProductCategories(context.Context, []uuid.UUID) (map[uuid.UUID]string, error)
}
