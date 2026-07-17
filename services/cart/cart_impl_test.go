package cart

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/base/apperror"
	"github.com/linenxing/e-commerce-system/models"
	cartstore "github.com/linenxing/e-commerce-system/stores/cart"
	productstore "github.com/linenxing/e-commerce-system/stores/product"
)

type fakeCartStore struct {
	cart        models.Cart
	items       []cartstore.ItemWithProduct
	upsertCalls int
}

func (s *fakeCartStore) GetOrCreate(context.Context, uuid.UUID) (models.Cart, error) {
	return s.cart, nil
}
func (s *fakeCartStore) ListItems(context.Context, uuid.UUID, bool) ([]cartstore.ItemWithProduct, error) {
	return s.items, nil
}
func (s *fakeCartStore) UpsertItem(context.Context, uuid.UUID, uuid.UUID, int64) (models.CartItem, error) {
	s.upsertCalls++
	return models.CartItem{}, nil
}
func (s *fakeCartStore) UpdateItem(context.Context, uuid.UUID, uuid.UUID, int64) (models.CartItem, error) {
	return models.CartItem{}, nil
}
func (s *fakeCartStore) DeleteItem(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (s *fakeCartStore) Clear(context.Context, uuid.UUID) error                 { return nil }

type fakeProductStore struct{ product models.Product }

func (s *fakeProductStore) Create(context.Context, productstore.CreateParam) (models.Product, error) {
	return models.Product{}, nil
}
func (s *fakeProductStore) Update(context.Context, productstore.UpdateParam) (models.Product, error) {
	return models.Product{}, nil
}
func (s *fakeProductStore) GetByID(context.Context, uuid.UUID, bool) (models.Product, error) {
	return s.product, nil
}
func (s *fakeProductStore) List(context.Context, bool) ([]models.Product, error)  { return nil, nil }
func (s *fakeProductStore) DecreaseStock(context.Context, uuid.UUID, int64) error { return nil }

func TestAddItemValidatesAccumulatedQuantityBeforeWrite(t *testing.T) {
	productID := uuid.New()
	cartID := uuid.New()
	cart := &fakeCartStore{cart: models.Cart{ID: cartID}, items: []cartstore.ItemWithProduct{{Item: models.CartItem{ProductID: productID, Quantity: 4}}}}
	products := &fakeProductStore{product: models.Product{ID: productID, Stock: 5, Status: models.ProductStatusActive}}
	_, err := New(cart, products).AddItem(context.Background(), AddItemParam{UserID: uuid.New(), ProductID: productID, Quantity: 2})
	if !errors.Is(err, apperror.ErrInsufficientStock) {
		t.Fatalf("expected insufficient stock, got %v", err)
	}
	if cart.upsertCalls != 0 {
		t.Fatalf("expected no write, got %d calls", cart.upsertCalls)
	}
}
