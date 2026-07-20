package cartrecall

import (
	"context"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

type Service interface {
	List(context.Context) ([]models.CartRecallJourney, error)
	Get(context.Context, uuid.UUID) (models.CartRecallJourney, error)
	Cancel(context.Context, uuid.UUID) error
}
