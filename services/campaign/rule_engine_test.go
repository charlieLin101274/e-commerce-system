package campaign

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/models"
)

func raw(value string) json.RawMessage { return json.RawMessage(value) }

func TestRuleEngineEvaluatesNestedGroupsDeterministically(t *testing.T) {
	rule := &models.RuleGroup{Operator: "and", Conditions: []models.RuleCondition{
		{Fact: "product.price", Operator: "gte", Value: raw(`500`)},
	}, Groups: []models.RuleGroup{{Operator: "or", Conditions: []models.RuleCondition{
		{Fact: "member.level", Operator: "eq", Value: raw(`"gold"`)},
		{Fact: "member.tags", Operator: "contains", Value: raw(`"vip"`)},
	}}}}
	if errors := ValidateRule(rule, models.EvaluationContextCampaignDiscovery); len(errors) != 0 {
		t.Fatalf("unexpected validation errors: %v", errors)
	}
	eligible, decisions := evaluateRule(rule, models.EvaluationFacts{
		Member:  &models.MemberFacts{ID: uuid.New(), Tags: []string{"vip"}},
		Product: &models.ProductFacts{ID: uuid.New(), Price: 500},
	})
	if !eligible || len(decisions) != 3 {
		t.Fatalf("unexpected decision: eligible=%v decisions=%+v", eligible, decisions)
	}
}

func TestRuleEngineTreatsMissingFactAsFalse(t *testing.T) {
	rule := &models.RuleGroup{Operator: "and", Conditions: []models.RuleCondition{{Fact: "member.level", Operator: "eq", Value: raw(`"gold"`)}}}
	eligible, decisions := evaluateRule(rule, models.EvaluationFacts{})
	if eligible || len(decisions) != 1 || decisions[0].ReasonCode != "MISSING_FACT" || decisions[0].MissingFact != "member.level" {
		t.Fatalf("unexpected missing fact decision: %+v", decisions)
	}
}

func TestRuleValidationRejectsWrongTypeAndContext(t *testing.T) {
	rule := &models.RuleGroup{Operator: "and", Conditions: []models.RuleCondition{
		{Fact: "cart.total_price", Operator: "gt", Value: raw(`"100"`)},
	}}
	if errors := ValidateRule(rule, models.EvaluationContextCampaignDiscovery); len(errors) == 0 {
		t.Fatal("expected context validation error")
	}
	if errors := ValidateRule(rule, models.EvaluationContextCartRecall); len(errors) == 0 {
		t.Fatal("expected type validation error")
	}
}

func TestNormalizeRuleCanonicalizesCategoryValues(t *testing.T) {
	rule := &models.RuleGroup{Operator: "AND", Conditions: []models.RuleCondition{
		{Fact: "product.category", Operator: "EQ", Value: raw(`" Electronics "`)},
		{Fact: "product.category", Operator: "IN", Value: raw(`[" HOME ","Electronics"]`)},
	}}
	normalizeRule(rule)
	if string(rule.Conditions[0].Value) != `"electronics"` || string(rule.Conditions[1].Value) != `["home","electronics"]` {
		t.Fatalf("unexpected normalized category values: %s %s", rule.Conditions[0].Value, rule.Conditions[1].Value)
	}
}

func TestRuleValidationAllowsOnlyContainsForArrayFacts(t *testing.T) {
	rule := &models.RuleGroup{Operator: "and", Conditions: []models.RuleCondition{{Fact: "member.tags", Operator: "eq", Value: raw(`"vip"`)}}}
	if errors := ValidateRule(rule, models.EvaluationContextCampaignDiscovery); len(errors) == 0 {
		t.Fatal("expected array operator validation error")
	}
}
