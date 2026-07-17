package product

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/base/apperror"
	"github.com/linenxing/e-commerce-system/models"
	productstore "github.com/linenxing/e-commerce-system/stores/product"
)

type fakeStore struct{ products map[uuid.UUID]models.Product }

func (s *fakeStore) Create(_ context.Context, p productstore.CreateParam) (models.Product, error) {
	v := models.Product{ID: uuid.New(), Name: p.Name, Description: p.Description, Price: p.Price, Stock: p.Stock, Status: p.Status, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	s.products[v.ID] = v
	return v, nil
}
func (s *fakeStore) Update(_ context.Context, p productstore.UpdateParam) (models.Product, error) {
	v, ok := s.products[p.ID]
	if !ok {
		return models.Product{}, productstore.ErrNotFound
	}
	v.Name, v.Description, v.Price, v.Stock, v.Status = p.Name, p.Description, p.Price, p.Stock, p.Status
	s.products[v.ID] = v
	return v, nil
}
func (s *fakeStore) GetByID(_ context.Context, id uuid.UUID, _ bool) (models.Product, error) {
	v, ok := s.products[id]
	if !ok {
		return models.Product{}, productstore.ErrNotFound
	}
	return v, nil
}
func (s *fakeStore) List(_ context.Context, includeInactive bool) ([]models.Product, error) {
	out := []models.Product{}
	for _, v := range s.products {
		if includeInactive || v.Status == models.ProductStatusActive {
			out = append(out, v)
		}
	}
	return out, nil
}
func (s *fakeStore) DecreaseStock(context.Context, uuid.UUID, int64) error { return nil }

func TestCreateProduct(t *testing.T) {
	s := &fakeStore{products: map[uuid.UUID]models.Product{}}
	got, err := New(s).Create(context.Background(), CreateParam{Name: " Product ", Price: 100, Stock: 2})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	if got.Name != "Product" || got.Status != models.ProductStatusActive {
		t.Fatalf("unexpected product: %+v", got)
	}
}

func TestCreateProductRejectsNegativePrice(t *testing.T) {
	s := &fakeStore{products: map[uuid.UUID]models.Product{}}
	_, err := New(s).Create(context.Background(), CreateParam{Name: "Product", Price: -1})
	if !errors.Is(err, apperror.ErrInvalidInput) {
		t.Fatalf("expected invalid input, got %v", err)
	}
}

func TestCustomerCannotGetInactiveProduct(t *testing.T) {
	id := uuid.New()
	s := &fakeStore{products: map[uuid.UUID]models.Product{id: {ID: id, Name: "Hidden", Status: models.ProductStatusInactive}}}
	_, err := New(s).Get(context.Background(), id, false)
	if !errors.Is(err, apperror.ErrNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}
