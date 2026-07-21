//go:build integration

package suites

import (
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/linenxing/e-commerce-system/integration-tests/testkit"
	"github.com/linenxing/e-commerce-system/models"
	campaignstore "github.com/linenxing/e-commerce-system/stores/campaign"
)

func TestCampaignPublicPaginationAndDatabaseFiltering(t *testing.T) {
	admin := testScenario.LoginAdmin(t)
	product := testScenario.CreateProduct(t, admin.AccessToken, 1000, 10)
	created := make([]testkit.AdminCampaign, 0, 3)
	for priority := 1; priority <= 3; priority++ {
		campaign := testScenario.CreateDraftCampaign(t, admin.AccessToken, product.ID, nil)
		campaign.Priority = priority
		campaign, err := testClient.UpdateCampaign(t.Context(), admin.AccessToken, campaign.ID, campaignInputFrom(campaign))
		if err != nil {
			t.Fatalf("update campaign priority: %v", err)
		}
		campaign, err = testClient.PublishCampaign(t.Context(), admin.AccessToken, campaign.ID)
		if err != nil {
			t.Fatalf("publish campaign: %v", err)
		}
		created = append(created, campaign)
	}

	first, err := testClient.ListPublicCampaignPage(t.Context(), "", &product.ID, 1, 0)
	if err != nil {
		t.Fatalf("list first campaign page: %v", err)
	}
	second, err := testClient.ListPublicCampaignPage(t.Context(), "", &product.ID, 1, 1)
	if err != nil {
		t.Fatalf("list second campaign page: %v", err)
	}
	if len(first) != 1 || first[0].ID != created[2].ID {
		t.Fatalf("first page = %#v, want highest priority campaign %s", first, created[2].ID)
	}
	if len(second) != 1 || second[0].ID != created[1].ID {
		t.Fatalf("second page = %#v, want second priority campaign %s", second, created[1].ID)
	}

	unrelated := testScenario.CreateProduct(t, admin.AccessToken, 1000, 10)
	filtered, err := testClient.ListPublicCampaignPage(t.Context(), "", &unrelated.ID, 20, 0)
	if err != nil {
		t.Fatalf("list unrelated product campaigns: %v", err)
	}
	for _, campaign := range filtered {
		if campaign.ID == created[0].ID || campaign.ID == created[1].ID || campaign.ID == created[2].ID {
			t.Fatalf("database scope filtering returned unrelated campaign %s", campaign.ID)
		}
	}
}

func TestCampaignContextFilteringKeepsScopeOnlyCampaigns(t *testing.T) {
	admin := testScenario.LoginAdmin(t)
	product := testScenario.CreateProduct(t, admin.AccessToken, 1000, 10)
	now := time.Now().UTC()
	input := testkit.CampaignInput{
		Name: "Scope-only Cart Recall", Priority: 10, StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour),
		PromotionTitle: "Return to cart", BenefitType: models.BenefitTypeFixedAmount, BenefitValue: 100,
		ProductIDs: []uuid.UUID{product.ID}, ContextType: models.EvaluationContextCartRecall,
	}
	created, err := testClient.CreateCampaign(t.Context(), admin.AccessToken, input)
	if err != nil {
		t.Fatalf("create scope-only cart recall campaign: %v", err)
	}
	if created.RuleVersion != 1 || created.RuleContextType != models.EvaluationContextCartRecall || created.EligibilityRule != nil {
		t.Fatalf("scope-only campaign rule metadata = %#v", created)
	}
	if _, err = testClient.PublishCampaign(t.Context(), admin.AccessToken, created.ID); err != nil {
		t.Fatalf("publish scope-only cart recall campaign: %v", err)
	}

	pool, err := pgxpool.New(t.Context(), testEnvironment.DatabaseURL)
	if err != nil {
		t.Fatalf("connect integration database: %v", err)
	}
	defer pool.Close()
	store := campaignstore.NewPostgresStore(pool)
	candidates, err := store.ListPublicCandidates(t.Context(), campaignstore.CandidateQuery{
		Now: now, ProductID: &product.ID, Category: product.Category,
		ContextType: models.EvaluationContextCartRecall, Limit: 20,
	})
	if err != nil {
		t.Fatalf("list cart recall candidates: %v", err)
	}
	if !containsCampaign(candidates, created.ID) {
		t.Fatalf("cart recall candidates do not contain scope-only campaign %s", created.ID)
	}
	publicCampaigns, err := testClient.ListPublicCampaigns(t.Context(), "", &product.ID)
	if err != nil {
		t.Fatalf("list discovery campaigns: %v", err)
	}
	if containsCampaign(publicCampaigns, created.ID) {
		t.Fatalf("cart recall campaign %s leaked into discovery list", created.ID)
	}
}

func TestCampaignListRejectsLimitAboveMaximum(t *testing.T) {
	if err := testClient.ExpectError(t.Context(), http.MethodGet, "/campaigns?limit=21", "", nil, http.StatusBadRequest, "invalid_request"); err != nil {
		t.Fatal(err)
	}
}

func TestCampaignConcurrentDraftUpdateAllowsSingleWinner(t *testing.T) {
	admin := testScenario.LoginAdmin(t)
	product := testScenario.CreateProduct(t, admin.AccessToken, 1000, 10)
	created := testScenario.CreateDraftCampaign(t, admin.AccessToken, product.ID, nil)

	pool, err := pgxpool.New(t.Context(), testEnvironment.DatabaseURL)
	if err != nil {
		t.Fatalf("connect integration database: %v", err)
	}
	defer pool.Close()
	store := campaignstore.NewPostgresStore(pool)
	stale, err := store.GetByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("load campaign before concurrent update: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var wait sync.WaitGroup
	for attempt := 1; attempt <= 2; attempt++ {
		wait.Add(1)
		go func(priority int) {
			defer wait.Done()
			value := stale
			value.Priority = priority
			<-start
			_, err := store.Update(t.Context(), value)
			results <- err
		}(attempt)
	}
	close(start)
	wait.Wait()
	close(results)

	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("concurrent updates succeeded %d times, want exactly one winner", succeeded)
	}
}

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
