//go:build integration

package suites

import (
	"testing"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

func TestCartItemLifecycle(t *testing.T) {
	admin := testScenario.LoginAdmin(t)
	customer := testScenario.CreateCustomer(t)
	product := testScenario.CreateProduct(t, admin.AccessToken, 400, 10)

	cart, err := testClient.AddCartItem(t.Context(), customer.AccessToken, product.ID, 2)
	if err != nil {
		t.Fatalf("add cart item: %v", err)
	}
	if len(cart.Items) != 1 || cart.Items[0].Quantity != 2 || cart.TotalPrice != 800 {
		t.Fatalf("cart after add = %#v", cart)
	}

	cart, err = testClient.UpdateCartItem(t.Context(), customer.AccessToken, cart.Items[0].ID, 3)
	if err != nil {
		t.Fatalf("update cart item: %v", err)
	}
	if len(cart.Items) != 1 || cart.Items[0].Quantity != 3 || cart.TotalPrice != 1200 {
		t.Fatalf("cart after update = %#v", cart)
	}

	if err := testClient.RemoveCartItem(t.Context(), customer.AccessToken, cart.Items[0].ID); err != nil {
		t.Fatalf("remove cart item: %v", err)
	}
	cart, err = testClient.GetCart(t.Context(), customer.AccessToken)
	if err != nil {
		t.Fatalf("get empty cart: %v", err)
	}
	if len(cart.Items) != 0 || cart.TotalPrice != 0 {
		t.Fatalf("cart after remove = %#v, want empty cart", cart)
	}
}

func TestCreateOrderFromCart(t *testing.T) {
	admin := testScenario.LoginAdmin(t)
	customer := testScenario.CreateCustomer(t)
	product := testScenario.CreateProduct(t, admin.AccessToken, 750, 5)
	if _, err := testClient.AddCartItem(t.Context(), customer.AccessToken, product.ID, 2); err != nil {
		t.Fatalf("add product to cart: %v", err)
	}

	created, err := testClient.CreateOrder(t.Context(), customer.AccessToken)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if created.Status != models.OrderStatusCompleted || created.TotalPrice != 1500 || len(created.Items) != 1 {
		t.Fatalf("created order = %#v", created)
	}
	if created.Items[0].ProductID != product.ID || created.Items[0].Quantity != 2 || created.Items[0].Subtotal != 1500 {
		t.Fatalf("created order item = %#v", created.Items[0])
	}

	cart, err := testClient.GetCart(t.Context(), customer.AccessToken)
	if err != nil {
		t.Fatalf("get cart after order: %v", err)
	}
	if len(cart.Items) != 0 {
		t.Fatalf("cart after order = %#v, want empty cart", cart)
	}

	stored, err := testClient.GetOrder(t.Context(), customer.AccessToken, created.ID)
	if err != nil {
		t.Fatalf("get created order: %v", err)
	}
	if stored.ID != created.ID || len(stored.Items) != 1 {
		t.Fatalf("stored order = %#v, want order %s", stored, created.ID)
	}

	orders, err := testClient.ListOrders(t.Context(), customer.AccessToken)
	if err != nil {
		t.Fatalf("list customer orders: %v", err)
	}
	if !containsOrder(orders, created.ID) {
		t.Fatalf("order list does not contain created order %s", created.ID)
	}

	remaining, err := testClient.GetProduct(t.Context(), product.ID)
	if err != nil {
		t.Fatalf("get product after order: %v", err)
	}
	if remaining.Stock != 3 {
		t.Fatalf("product stock after order = %d, want 3", remaining.Stock)
	}
}

func containsOrder(orders []models.OrderResp, id uuid.UUID) bool {
	for _, order := range orders {
		if order.ID == id {
			return true
		}
	}
	return false
}
