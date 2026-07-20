package cartrecall

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

var (
	ErrNotFound = errors.New("cart recall journey not found")
	ErrConflict = errors.New("cart recall journey conflict")
)

type CartState struct {
	Cart       models.Cart
	Items      []CartItemState
	LastChange time.Time
}

type CartItemState struct {
	Product  models.Product
	Quantity int64
}

type Store interface {
	ConsumeEvents(context.Context, time.Time, time.Duration, int) (int, error)
	ClaimDue(context.Context, time.Time, time.Duration, int) ([]models.CartRecallJourney, error)
	GetCartState(context.Context, uuid.UUID) (CartState, error)
	GetMemberFacts(context.Context, uuid.UUID) (models.MemberFacts, error)
	MarkSkipped(context.Context, uuid.UUID, string) error
	MarkCancelled(context.Context, uuid.UUID, string) error
	MarkNotificationPending(context.Context, uuid.UUID, models.Campaign, []models.CartRecallProductSnapshot, uuid.UUID) error
	CancelInvalidPending(context.Context, time.Time) (int, error)
	SyncDelivered(context.Context, time.Time) (int, error)
	List(context.Context) ([]models.CartRecallJourney, error)
	Get(context.Context, uuid.UUID) (models.CartRecallJourney, error)
	Cancel(context.Context, uuid.UUID, string) error
}
