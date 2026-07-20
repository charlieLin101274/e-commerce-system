//go:build integration

package suites

import (
	"net/http"
	"testing"

	"github.com/linenxing/e-commerce-system/integration-tests/testkit"
	"github.com/linenxing/e-commerce-system/models"
)

func TestProductLifecycleAndPublicVisibility(t *testing.T) {
	admin := testScenario.LoginAdmin(t)
	created := testScenario.CreateProduct(t, admin.AccessToken, 1250, 8)
	if created.Status != models.ProductStatusActive || created.Category != "integration" {
		t.Fatalf("created product = %#v, want active integration product", created)
	}

	publicProduct, err := testClient.GetProduct(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("get public product: %v", err)
	}
	if publicProduct.ID != created.ID || publicProduct.Price != 1250 {
		t.Fatalf("public product = %#v, want created product %s", publicProduct, created.ID)
	}

	updated, err := testClient.UpdateProduct(t.Context(), admin.AccessToken, created.ID, testkit.ProductInput{
		Name:        created.Name + " Updated",
		Description: created.Description,
		Category:    " Updated-Category ",
		Price:       1500,
		Stock:       12,
		Status:      models.ProductStatusActive,
	})
	if err != nil {
		t.Fatalf("update product: %v", err)
	}
	if updated.Price != 1500 || updated.Stock != 12 || updated.Category != "updated-category" {
		t.Fatalf("updated product = %#v", updated)
	}

	if err := testClient.DisableProduct(t.Context(), admin.AccessToken, created.ID); err != nil {
		t.Fatalf("disable product: %v", err)
	}
	if err := testClient.ExpectError(t.Context(), http.MethodGet, "/products/"+created.ID.String(), "", nil, http.StatusNotFound, "not_found"); err != nil {
		t.Fatal(err)
	}
}

func TestProductListIncludesCreatedActiveProduct(t *testing.T) {
	admin := testScenario.LoginAdmin(t)
	created := testScenario.CreateProduct(t, admin.AccessToken, 900, 3)

	products, err := testClient.ListProducts(t.Context())
	if err != nil {
		t.Fatalf("list public products: %v", err)
	}
	for _, product := range products {
		if product.ID == created.ID {
			return
		}
	}
	t.Fatalf("public product list does not contain created product %s", created.ID)
}
