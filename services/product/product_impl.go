package product

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/base/apperror"
	"github.com/linenxing/e-commerce-system/models"
	productstore "github.com/linenxing/e-commerce-system/stores/product"
	"strings"
)

type service struct{ store productstore.Store }

func New(store productstore.Store) Service { return &service{store: store} }
func (s *service) Create(ctx context.Context, p CreateParam) (models.ProductResp, error) {
	if strings.TrimSpace(p.Name) == "" || p.Price < 0 || p.Stock < 0 {
		return models.ProductResp{}, apperror.ErrInvalidInput
	}
	v, err := s.store.Create(ctx, productstore.CreateParam{Name: strings.TrimSpace(p.Name), Description: strings.TrimSpace(p.Description), Price: p.Price, Stock: p.Stock, Status: models.ProductStatusActive})
	return toResp(v), err
}
func (s *service) Update(ctx context.Context, p UpdateParam) (models.ProductResp, error) {
	if strings.TrimSpace(p.Name) == "" || p.Price < 0 || p.Stock < 0 || (p.Status != models.ProductStatusActive && p.Status != models.ProductStatusInactive) {
		return models.ProductResp{}, apperror.ErrInvalidInput
	}
	v, err := s.store.Update(ctx, productstore.UpdateParam{ID: p.ID, Name: strings.TrimSpace(p.Name), Description: strings.TrimSpace(p.Description), Price: p.Price, Stock: p.Stock, Status: p.Status})
	if errors.Is(err, productstore.ErrNotFound) {
		return models.ProductResp{}, apperror.ErrNotFound
	}
	return toResp(v), err
}
func (s *service) Disable(ctx context.Context, id uuid.UUID) error {
	v, err := s.store.GetByID(ctx, id, false)
	if errors.Is(err, productstore.ErrNotFound) {
		return apperror.ErrNotFound
	}
	if err != nil {
		return err
	}
	_, err = s.store.Update(ctx, productstore.UpdateParam{ID: id, Name: v.Name, Description: v.Description, Price: v.Price, Stock: v.Stock, Status: models.ProductStatusInactive})
	return err
}
func (s *service) List(ctx context.Context, admin bool) ([]models.ProductResp, error) {
	items, err := s.store.List(ctx, admin)
	if err != nil {
		return nil, err
	}
	out := make([]models.ProductResp, 0, len(items))
	for _, v := range items {
		out = append(out, toResp(v))
	}
	return out, nil
}
func (s *service) Get(ctx context.Context, id uuid.UUID, admin bool) (models.ProductResp, error) {
	v, err := s.store.GetByID(ctx, id, false)
	if errors.Is(err, productstore.ErrNotFound) || (!admin && err == nil && v.Status != models.ProductStatusActive) {
		return models.ProductResp{}, apperror.ErrNotFound
	}
	return toResp(v), err
}
func toResp(v models.Product) models.ProductResp {
	return models.ProductResp{ID: v.ID, Name: v.Name, Description: v.Description, Price: v.Price, Stock: v.Stock, Status: v.Status, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
