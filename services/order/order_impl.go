package order

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/linenxing/e-commerce-system/base/apperror"
	"github.com/linenxing/e-commerce-system/models"
	cartstore "github.com/linenxing/e-commerce-system/stores/cart"
	orderstore "github.com/linenxing/e-commerce-system/stores/order"
	productstore "github.com/linenxing/e-commerce-system/stores/product"
)

type service struct {
	db     *pgxpool.Pool
	orders orderstore.Store
}

func New(db *pgxpool.Pool, orders orderstore.Store) Service { return &service{db: db, orders: orders} }
func (s *service) Create(ctx context.Context, userID uuid.UUID) (models.OrderResp, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return models.OrderResp{}, err
	}
	defer tx.Rollback(ctx)
	cStore := cartstore.NewPostgresStore(tx)
	pStore := productstore.NewPostgresStore(tx)
	oStore := orderstore.NewPostgresStore(tx)
	cart, err := cStore.GetOrCreate(ctx, userID)
	if err != nil {
		return models.OrderResp{}, err
	}
	items, err := cStore.ListItems(ctx, cart.ID, true)
	if err != nil {
		return models.OrderResp{}, err
	}
	if len(items) == 0 {
		return models.OrderResp{}, apperror.ErrInvalidInput
	}
	var total int64
	params := make([]orderstore.CreateItemParam, 0, len(items))
	for _, v := range items {
		if v.Product.Status != models.ProductStatusActive || v.Item.Quantity > v.Product.Stock {
			return models.OrderResp{}, apperror.ErrInsufficientStock
		}
		total += v.Product.Price * v.Item.Quantity
		params = append(params, orderstore.CreateItemParam{ProductID: v.Product.ID, ProductName: v.Product.Name, Price: v.Product.Price, Quantity: v.Item.Quantity})
	}
	created, err := oStore.Create(ctx, userID, total)
	if err != nil {
		return models.OrderResp{}, err
	}
	if err = oStore.CreateItems(ctx, created.ID, params); err != nil {
		return models.OrderResp{}, err
	}
	for _, v := range items {
		if err = pStore.DecreaseStock(ctx, v.Product.ID, v.Item.Quantity); err != nil {
			return models.OrderResp{}, apperror.ErrInsufficientStock
		}
	}
	if err = cStore.Clear(ctx, cart.ID); err != nil {
		return models.OrderResp{}, err
	}
	productIDs := make([]uuid.UUID, 0, len(params))
	for _, item := range params {
		productIDs = append(productIDs, item.ProductID)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO domain_outbox(event_type,aggregate_id,payload) VALUES(
		'order.completed',$1,jsonb_build_object('order_id',$1,'user_id',$2,'cart_id',$3,'product_ids',$4::uuid[])
	)`, created.ID, userID, cart.ID, productIDs); err != nil {
		return models.OrderResp{}, fmt.Errorf("create order completed outbox event: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return models.OrderResp{}, fmt.Errorf("commit order: %w", err)
	}
	resp := orderResp(created)
	resp.Items = make([]models.OrderItemResp, 0, len(params))
	for _, v := range params {
		resp.Items = append(resp.Items, models.OrderItemResp{ProductID: v.ProductID, ProductName: v.ProductName, Price: v.Price, Quantity: v.Quantity, Subtotal: v.Price * v.Quantity})
	}
	return resp, nil
}
func (s *service) List(ctx context.Context, userID uuid.UUID) ([]models.OrderResp, error) {
	items, err := s.orders.List(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]models.OrderResp, 0, len(items))
	for _, v := range items {
		out = append(out, orderResp(v))
	}
	return out, nil
}
func (s *service) Get(ctx context.Context, userID, orderID uuid.UUID) (models.OrderResp, error) {
	v, items, err := s.orders.Get(ctx, userID, orderID)
	if errors.Is(err, orderstore.ErrNotFound) {
		return models.OrderResp{}, apperror.ErrNotFound
	}
	if err != nil {
		return models.OrderResp{}, err
	}
	out := orderResp(v)
	out.Items = []models.OrderItemResp{}
	for _, i := range items {
		out.Items = append(out.Items, models.OrderItemResp{ProductID: i.ProductID, ProductName: i.ProductName, Price: i.Price, Quantity: i.Quantity, Subtotal: i.Price * i.Quantity})
	}
	return out, nil
}
func orderResp(v models.Order) models.OrderResp {
	return models.OrderResp{ID: v.ID, TotalPrice: v.TotalPrice, Status: v.Status, CreatedAt: v.CreatedAt}
}
