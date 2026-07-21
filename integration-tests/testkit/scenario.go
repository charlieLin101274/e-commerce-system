package testkit

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

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

func (s *Scenario) CreateDraftCampaign(t *testing.T, token string, productID uuid.UUID, rule *models.RuleGroup) AdminCampaign {
	t.Helper()
	suffix := uuid.NewString()
	output, err := s.Client.CreateCampaign(t.Context(), token, CampaignInput{
		Name:                 "Integration Campaign " + suffix,
		Description:          "Campaign created by an integration test",
		Priority:             10,
		StartsAt:             time.Now().UTC().Add(-5 * time.Minute),
		EndsAt:               time.Now().UTC().Add(time.Hour),
		PromotionTitle:       "Integration Promotion " + suffix,
		PromotionDescription: "Promotion created by an integration test",
		BenefitType:          models.BenefitTypeFixedAmount,
		BenefitValue:         200,
		ProductIDs:           []uuid.UUID{productID},
		ContextType:          models.EvaluationContextCampaignDiscovery,
		EligibilityRule:      rule,
	})
	if err != nil {
		t.Fatalf("create test campaign: %v", err)
	}
	return output
}

func (s *Scenario) CreateCartRecallCampaign(t *testing.T, token string, productID uuid.UUID) AdminCampaign {
	t.Helper()
	suffix := uuid.NewString()
	rule := &models.RuleGroup{Operator: "and", Conditions: []models.RuleCondition{{
		ID: "has-cart-item", Fact: "cart.item_count", Operator: "gte", Value: json.RawMessage(`1`),
	}}}
	output, err := s.Client.CreateCampaign(t.Context(), token, CampaignInput{
		Name:                 "Integration Cart Recall " + suffix,
		Description:          "Cart recall campaign created by an integration test",
		Priority:             100,
		StartsAt:             time.Now().UTC().Add(-5 * time.Minute),
		EndsAt:               time.Now().UTC().Add(time.Hour),
		PromotionTitle:       "Complete your purchase",
		PromotionDescription: "Your cart is waiting",
		BenefitType:          models.BenefitTypeFixedAmount,
		BenefitValue:         100,
		ProductIDs:           []uuid.UUID{productID},
		ContextType:          models.EvaluationContextCartRecall,
		EligibilityRule:      rule,
	})
	if err != nil {
		t.Fatalf("create cart recall campaign: %v", err)
	}
	return output
}
