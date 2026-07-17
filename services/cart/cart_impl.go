package cart

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/base/apperror"
	"github.com/linenxing/e-commerce-system/models"
	cartstore "github.com/linenxing/e-commerce-system/stores/cart"
	productstore "github.com/linenxing/e-commerce-system/stores/product"
)

type service struct {
	cart    cartstore.Store
	product productstore.Store
}

func New(c cartstore.Store, p productstore.Store) Service { return &service{cart: c, product: p} }
func (s *service) Get(ctx context.Context, userID uuid.UUID) (models.CartResp, error) {
	c, err := s.cart.GetOrCreate(ctx, userID)
	if err != nil {
		return models.CartResp{}, err
	}
	return s.response(ctx, c)
}
func (s *service) AddItem(ctx context.Context, p AddItemParam) (models.CartResp, error) {
	if p.Quantity <= 0 {
		return models.CartResp{}, apperror.ErrInvalidInput
	}
	product, err := s.product.GetByID(ctx, p.ProductID, false)
	if errors.Is(err, productstore.ErrNotFound) || product.Status != models.ProductStatusActive {
		return models.CartResp{}, apperror.ErrNotFound
	}
	if err != nil {
		return models.CartResp{}, err
	}
	c, err := s.cart.GetOrCreate(ctx, p.UserID)
	if err != nil {
		return models.CartResp{}, err
	}
	items, err := s.cart.ListItems(ctx, c.ID, false)
	if err != nil {
		return models.CartResp{}, err
	}
	quantity := p.Quantity
	for _, item := range items {
		if item.Item.ProductID == p.ProductID {
			quantity += item.Item.Quantity
			break
		}
	}
	if quantity > product.Stock {
		return models.CartResp{}, apperror.ErrInsufficientStock
	}
	if _, err := s.cart.UpsertItem(ctx, c.ID, p.ProductID, p.Quantity); err != nil {
		return models.CartResp{}, err
	}
	return s.response(ctx, c)
}
func (s *service) UpdateItem(ctx context.Context, p UpdateItemParam) (models.CartResp, error) {
	if p.Quantity <= 0 {
		return models.CartResp{}, apperror.ErrInvalidInput
	}
	c, err := s.cart.GetOrCreate(ctx, p.UserID)
	if err != nil {
		return models.CartResp{}, err
	}
	items, err := s.cart.ListItems(ctx, c.ID, false)
	if err != nil {
		return models.CartResp{}, err
	}
	var target *cartstore.ItemWithProduct
	for i := range items {
		if items[i].Item.ID == p.ItemID {
			target = &items[i]
			break
		}
	}
	if target == nil {
		return models.CartResp{}, apperror.ErrNotFound
	}
	if target.Product.Status != models.ProductStatusActive {
		return models.CartResp{}, apperror.ErrNotFound
	}
	if p.Quantity > target.Product.Stock {
		return models.CartResp{}, apperror.ErrInsufficientStock
	}
	_, err = s.cart.UpdateItem(ctx, c.ID, p.ItemID, p.Quantity)
	if errors.Is(err, cartstore.ErrNotFound) {
		return models.CartResp{}, apperror.ErrNotFound
	}
	if err != nil {
		return models.CartResp{}, err
	}
	return s.response(ctx, c)
}
func (s *service) RemoveItem(ctx context.Context, p RemoveItemParam) error {
	c, err := s.cart.GetOrCreate(ctx, p.UserID)
	if err != nil {
		return err
	}
	err = s.cart.DeleteItem(ctx, c.ID, p.ItemID)
	if errors.Is(err, cartstore.ErrNotFound) {
		return apperror.ErrNotFound
	}
	return err
}
func (s *service) response(ctx context.Context, c models.Cart) (models.CartResp, error) {
	items, err := s.cart.ListItems(ctx, c.ID, false)
	if err != nil {
		return models.CartResp{}, err
	}
	out := models.CartResp{ID: c.ID, Items: []models.CartItemResp{}}
	for _, v := range items {
		sub := v.Product.Price * v.Item.Quantity
		out.TotalPrice += sub
		out.Items = append(out.Items, models.CartItemResp{ID: v.Item.ID, Product: productResp(v.Product), Quantity: v.Item.Quantity, Subtotal: sub})
	}
	return out, nil
}
func productResp(v models.Product) models.ProductResp {
	return models.ProductResp{ID: v.ID, Name: v.Name, Description: v.Description, Price: v.Price, Stock: v.Stock, Status: v.Status, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt}
}
