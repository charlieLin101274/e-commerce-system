package testkit

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
	authservice "github.com/linenxing/e-commerce-system/services/auth"
)

const TestPassword = "Integration123!"

type Scenario struct {
	Client        *Client
	AdminEmail    string
	AdminPassword string
}

func NewScenario(client *Client, environment Environment) *Scenario {
	return &Scenario{Client: client, AdminEmail: environment.AdminEmail, AdminPassword: environment.AdminPassword}
}

func (s *Scenario) CreateCustomer(t *testing.T) authservice.AuthOutput {
	t.Helper()
	suffix := strings.ToLower(uuid.NewString())
	output, err := s.Client.Register(t.Context(), fmt.Sprintf("integration-%s@example.com", suffix), TestPassword, "Integration Customer")
	if err != nil {
		t.Fatalf("register test customer: %v", err)
	}
	return output
}

func (s *Scenario) LoginAdmin(t *testing.T) authservice.AuthOutput {
	t.Helper()
	output, err := s.Client.Login(t.Context(), s.AdminEmail, s.AdminPassword)
	if err != nil {
		t.Fatalf("login test admin: %v", err)
	}
	return output
}

func (s *Scenario) CreateProduct(t *testing.T, token string, price, stock int64) models.ProductResp {
	t.Helper()
	suffix := uuid.NewString()
	output, err := s.Client.CreateProduct(t.Context(), token, ProductInput{
		Name:        "Integration Product " + suffix,
		Description: "Product created by an integration test",
		Category:    "integration",
		Price:       price,
		Stock:       stock,
	})
	if err != nil {
		t.Fatalf("create test product: %v", err)
	}
	return output
}
