//go:build integration

package suites

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/integration-tests/testkit"
	"github.com/linenxing/e-commerce-system/models"
)

func TestCampaignLifecycleAndPublicVisibility(t *testing.T) {
	admin := testScenario.LoginAdmin(t)
	product := testScenario.CreateProduct(t, admin.AccessToken, 1000, 10)
	created := testScenario.CreateDraftCampaign(t, admin.AccessToken, product.ID, nil)
	if created.Status != models.CampaignStatusDraft || created.CreatedBy != admin.User.ID {
		t.Fatalf("created campaign = %#v, want admin-owned draft", created)
	}

	created.Name += " Updated"
	updated, err := testClient.UpdateCampaign(t.Context(), admin.AccessToken, created.ID, campaignInputFrom(created))
	if err != nil {
		t.Fatalf("update draft campaign: %v", err)
	}
	if updated.Name != created.Name || updated.Status != models.CampaignStatusDraft {
		t.Fatalf("updated campaign = %#v", updated)
	}

	stored, err := testClient.GetAdminCampaign(t.Context(), admin.AccessToken, created.ID)
	if err != nil {
		t.Fatalf("get admin campaign: %v", err)
	}
	if stored.ID != created.ID || stored.Name != created.Name {
		t.Fatalf("stored campaign = %#v, want campaign %s", stored, created.ID)
	}
	adminCampaigns, err := testClient.ListAdminCampaigns(t.Context(), admin.AccessToken)
	if err != nil {
		t.Fatalf("list admin campaigns: %v", err)
	}
	if !containsAdminCampaign(adminCampaigns, created.ID) {
		t.Fatalf("admin campaign list does not contain %s", created.ID)
	}

	published, err := testClient.PublishCampaign(t.Context(), admin.AccessToken, created.ID)
	if err != nil {
		t.Fatalf("publish campaign: %v", err)
	}
	if published.Status != models.CampaignStatusRunning || published.PublishedAt == nil {
		t.Fatalf("published campaign = %#v, want running campaign", published)
	}

	publicCampaign, err := testClient.GetPublicCampaign(t.Context(), "", created.ID, &product.ID)
	if err != nil {
		t.Fatalf("get public campaign: %v", err)
	}
	if publicCampaign.ID != created.ID || publicCampaign.Status != models.CampaignStatusRunning {
		t.Fatalf("public campaign = %#v", publicCampaign)
	}
	publicCampaigns, err := testClient.ListPublicCampaigns(t.Context(), "", &product.ID)
	if err != nil {
		t.Fatalf("list public campaigns: %v", err)
	}
	if !containsCampaign(publicCampaigns, created.ID) {
		t.Fatalf("public campaign list does not contain %s", created.ID)
	}

	paused, err := testClient.PauseCampaign(t.Context(), admin.AccessToken, created.ID)
	if err != nil {
		t.Fatalf("pause campaign: %v", err)
	}
	if paused.Status != models.CampaignStatusPaused {
		t.Fatalf("paused campaign status = %q", paused.Status)
	}
	if err := testClient.ExpectError(t.Context(), http.MethodGet, "/campaigns/"+created.ID.String()+"?product_id="+product.ID.String(), "", nil, http.StatusNotFound, "not_found"); err != nil {
		t.Fatal(err)
	}

	resumed, err := testClient.ResumeCampaign(t.Context(), admin.AccessToken, created.ID)
	if err != nil {
		t.Fatalf("resume campaign: %v", err)
	}
	if resumed.Status != models.CampaignStatusRunning {
		t.Fatalf("resumed campaign status = %q", resumed.Status)
	}
	if _, err := testClient.PauseCampaign(t.Context(), admin.AccessToken, created.ID); err != nil {
		t.Fatalf("pause campaign before archive: %v", err)
	}
	archived, err := testClient.ArchiveCampaign(t.Context(), admin.AccessToken, created.ID)
	if err != nil {
		t.Fatalf("archive campaign: %v", err)
	}
	if archived.Status != models.CampaignStatusArchived {
		t.Fatalf("archived campaign status = %q", archived.Status)
	}
}

func TestCampaignRuleValidationAndEvaluation(t *testing.T) {
	admin := testScenario.LoginAdmin(t)
	product := testScenario.CreateProduct(t, admin.AccessToken, 1000, 10)
	rule := &models.RuleGroup{Operator: "AND", Conditions: []models.RuleCondition{{
		ID: "minimum-price", Fact: "product.price", Operator: "GTE", Value: json.RawMessage(`500`),
	}}}
	created := testScenario.CreateDraftCampaign(t, admin.AccessToken, product.ID, rule)
	if created.RuleVersion != 1 || created.RuleContextType != models.EvaluationContextCampaignDiscovery {
		t.Fatalf("created campaign rule metadata = version %d, context %q", created.RuleVersion, created.RuleContextType)
	}
	if created.EligibilityRule == nil || created.EligibilityRule.Operator != "and" || created.EligibilityRule.Conditions[0].Operator != "gte" {
		t.Fatalf("campaign rule was not normalized: %#v", created.EligibilityRule)
	}

	validation, err := testClient.ValidateCampaignRules(t.Context(), admin.AccessToken, created.ID)
	if err != nil {
		t.Fatalf("validate campaign rules: %v", err)
	}
	if !validation.Valid || len(validation.ValidationErrors) != 0 {
		t.Fatalf("rule validation = %#v, want valid", validation)
	}

	eligible, err := testClient.EvaluateCampaignRules(t.Context(), admin.AccessToken, created.ID, models.EvaluationContextCampaignDiscovery, models.EvaluationFacts{
		Product: &models.ProductFacts{ID: product.ID, Category: product.Category, Price: product.Price, Status: product.Status},
	})
	if err != nil {
		t.Fatalf("evaluate eligible campaign rule: %v", err)
	}
	if !eligible.Eligible || eligible.ReasonCode != "ELIGIBLE" || eligible.BenefitPreview == nil || eligible.BenefitPreview.FinalAmount != 800 {
		t.Fatalf("eligible evaluation = %#v", eligible)
	}

	ineligible, err := testClient.EvaluateCampaignRules(t.Context(), admin.AccessToken, created.ID, models.EvaluationContextCampaignDiscovery, models.EvaluationFacts{
		Product: &models.ProductFacts{ID: product.ID, Category: product.Category, Price: 400, Status: product.Status},
	})
	if err != nil {
		t.Fatalf("evaluate ineligible campaign rule: %v", err)
	}
	if ineligible.Eligible || ineligible.ReasonCode != "CONDITION_NOT_MATCHED" {
		t.Fatalf("ineligible evaluation = %#v", ineligible)
	}

	if _, err := testClient.PublishCampaign(t.Context(), admin.AccessToken, created.ID); err != nil {
		t.Fatalf("publish rule campaign: %v", err)
	}
	publicDecision, err := testClient.EvaluatePublicCampaign(t.Context(), "", created.ID, &product.ID)
	if err != nil {
		t.Fatalf("evaluate public campaign: %v", err)
	}
	if !publicDecision.Eligible || publicDecision.BenefitPreview == nil || publicDecision.BenefitPreview.DiscountAmount != 200 {
		t.Fatalf("public campaign evaluation = %#v", publicDecision)
	}
}

func TestCampaignRejectsInvalidScopeAndRule(t *testing.T) {
	admin := testScenario.LoginAdmin(t)
	now := time.Now().UTC()
	input := testkit.CampaignInput{
		Name: "Invalid Integration Campaign", StartsAt: now, EndsAt: now.Add(time.Hour),
		PromotionTitle: "Invalid Promotion", BenefitType: models.BenefitTypeFixedAmount, BenefitValue: 100,
	}
	if err := testClient.ExpectError(t.Context(), http.MethodPost, "/admin/campaigns", admin.AccessToken, input, http.StatusBadRequest, "invalid_request"); err != nil {
		t.Fatal(err)
	}

	input.Categories = []string{"integration"}
	input.ContextType = models.EvaluationContextCampaignDiscovery
	input.EligibilityRule = &models.RuleGroup{Operator: "and", Conditions: []models.RuleCondition{{
		Fact: "cart.total_price", Operator: "gte", Value: json.RawMessage(`100`),
	}}}
	if err := testClient.ExpectError(t.Context(), http.MethodPost, "/admin/campaigns", admin.AccessToken, input, http.StatusBadRequest, "invalid_request"); err != nil {
		t.Fatal(err)
	}
}

func campaignInputFrom(campaign testkit.AdminCampaign) testkit.CampaignInput {
	return testkit.CampaignInput{
		Name: campaign.Name, Description: campaign.Description, Priority: campaign.Priority,
		StartsAt: campaign.StartsAt, EndsAt: campaign.EndsAt,
		PromotionTitle: campaign.PromotionTitle, PromotionDescription: campaign.PromotionDescription,
		BenefitType: campaign.BenefitType, BenefitValue: campaign.BenefitValue,
		MaximumDiscountAmount: campaign.MaximumDiscountAmount, ProductIDs: campaign.ProductIDs,
		Categories: campaign.Categories, ContextType: campaign.RuleContextType, EligibilityRule: campaign.EligibilityRule,
	}
}

func containsAdminCampaign(campaigns []testkit.AdminCampaign, id uuid.UUID) bool {
	for _, campaign := range campaigns {
		if campaign.ID == id {
			return true
		}
	}
	return false
}

func containsCampaign(campaigns []models.Campaign, id uuid.UUID) bool {
	for _, campaign := range campaigns {
		if campaign.ID == id {
			return true
		}
	}
	return false
}
