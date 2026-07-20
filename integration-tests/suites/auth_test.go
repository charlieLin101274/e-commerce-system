//go:build integration

package suites

import (
	"net/http"
	"testing"

	"github.com/linenxing/e-commerce-system/integration-tests/testkit"
	"github.com/linenxing/e-commerce-system/models"
)

func TestAuthRegisterLoginAndCurrentUser(t *testing.T) {
	registered := testScenario.CreateCustomer(t)
	if registered.AccessToken == "" || registered.TokenType != "Bearer" {
		t.Fatalf("register returned invalid token metadata: %#v", registered)
	}
	if registered.User.Role != models.UserRoleCustomer {
		t.Fatalf("registered user role = %q, want %q", registered.User.Role, models.UserRoleCustomer)
	}

	current, err := testClient.CurrentUser(t.Context(), registered.AccessToken)
	if err != nil {
		t.Fatalf("get current user: %v", err)
	}
	if current.ID != registered.User.ID || current.Email != registered.User.Email {
		t.Fatalf("current user = %#v, want registered user %#v", current, registered.User)
	}

	loggedIn, err := testClient.Login(t.Context(), registered.User.Email, testkit.TestPassword)
	if err != nil {
		t.Fatalf("login registered customer: %v", err)
	}
	if loggedIn.User.ID != registered.User.ID || loggedIn.AccessToken == "" {
		t.Fatalf("login output = %#v, want user ID %s and an access token", loggedIn, registered.User.ID)
	}
}

func TestAuthRejectsInvalidCredentialsAndMissingToken(t *testing.T) {
	customer := testScenario.CreateCustomer(t)
	if err := testClient.ExpectError(t.Context(), http.MethodPost, "/auth/login", "", map[string]string{
		"email": customer.User.Email, "password": "Incorrect123!",
	}, http.StatusUnauthorized, "invalid_credentials"); err != nil {
		t.Fatal(err)
	}
	if err := testClient.ExpectError(t.Context(), http.MethodGet, "/users/me", "", nil, http.StatusUnauthorized, "unauthorized"); err != nil {
		t.Fatal(err)
	}
}

func TestCustomerCannotCreateAdminProduct(t *testing.T) {
	customer := testScenario.CreateCustomer(t)
	err := testClient.ExpectError(t.Context(), http.MethodPost, "/admin/products", customer.AccessToken, testkit.ProductInput{
		Name: "Forbidden Product", Price: 100, Stock: 1,
	}, http.StatusForbidden, "forbidden")
	if err != nil {
		t.Fatal(err)
	}
}
