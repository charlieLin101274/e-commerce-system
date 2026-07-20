package campaign

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/linenxing/e-commerce-system/models"
)

const (
	maxRuleDepth      = 5
	maxRuleConditions = 100
)

var allowedFacts = map[models.EvaluationContextType]map[string]string{
	models.EvaluationContextCampaignDiscovery: {
		"member.id": "string", "member.level": "string", "member.tags": "strings",
		"product.id": "string", "product.category": "string", "product.price": "number", "product.status": "string",
	},
	models.EvaluationContextCartRecall: {
		"member.id": "string", "member.level": "string", "member.tags": "strings",
		"product.id": "string", "product.category": "string", "product.price": "number", "product.status": "string",
		"cart.total_price": "number", "cart.item_count": "number",
	},
}

func ValidateRule(rule *models.RuleGroup, contextType models.EvaluationContextType) []string {
	if rule == nil {
		return nil
	}
	if _, ok := allowedFacts[contextType]; !ok {
		return []string{"unsupported context_type"}
	}
	errors := make([]string, 0)
	conditionCount := 0
	var visit func(models.RuleGroup, int, string)
	visit = func(group models.RuleGroup, depth int, path string) {
		if depth > maxRuleDepth {
			errors = append(errors, path+": maximum rule depth exceeded")
			return
		}
		if group.Operator != "and" && group.Operator != "or" {
			errors = append(errors, path+": group operator must be and or or")
		}
		if len(group.Conditions)+len(group.Groups) == 0 {
			errors = append(errors, path+": rule group must not be empty")
		}
		for index, condition := range group.Conditions {
			conditionCount++
			conditionPath := fmt.Sprintf("%s.conditions[%d]", path, index)
			factType, ok := allowedFacts[contextType][condition.Fact]
			if !ok {
				errors = append(errors, conditionPath+": fact is not allowed in this context")
				continue
			}
			if err := validateConditionValue(condition, factType); err != nil {
				errors = append(errors, conditionPath+": "+err.Error())
			}
		}
		for index, child := range group.Groups {
			visit(child, depth+1, fmt.Sprintf("%s.groups[%d]", path, index))
		}
	}
	visit(*rule, 1, "eligibility_rule")
	if conditionCount > maxRuleConditions {
		errors = append(errors, "eligibility_rule: maximum condition count exceeded")
	}
	return errors
}

func validateConditionValue(condition models.RuleCondition, factType string) error {
	allowed := map[string]bool{"eq": true, "gt": true, "gte": true, "lt": true, "lte": true, "in": true, "contains": true}
	if !allowed[condition.Operator] {
		return fmt.Errorf("unsupported operator")
	}
	if condition.Operator == "contains" && factType != "strings" {
		return fmt.Errorf("contains requires an array fact")
	}
	if factType == "strings" && condition.Operator != "contains" {
		return fmt.Errorf("array facts only support contains")
	}
	if (condition.Operator == "gt" || condition.Operator == "gte" || condition.Operator == "lt" || condition.Operator == "lte") && factType != "number" {
		return fmt.Errorf("comparison operator requires a numeric fact")
	}
	if condition.Operator == "in" {
		var values []json.RawMessage
		if json.Unmarshal(condition.Value, &values) != nil || len(values) == 0 {
			return fmt.Errorf("in requires a non-empty array value")
		}
		for _, value := range values {
			if !rawMatchesType(value, factType) {
				return fmt.Errorf("value type does not match fact type")
			}
		}
		return nil
	}
	if !rawMatchesType(condition.Value, factType) {
		return fmt.Errorf("value type does not match fact type")
	}
	return nil
}

func rawMatchesType(raw json.RawMessage, factType string) bool {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return false
	}
	switch factType {
	case "number":
		_, ok := value.(json.Number)
		return ok
	case "string", "strings":
		_, ok := value.(string)
		return ok
	default:
		return false
	}
}

func evaluateRule(rule *models.RuleGroup, facts models.EvaluationFacts) (bool, []models.ConditionDecision) {
	if rule == nil {
		return true, nil
	}
	decisions := make([]models.ConditionDecision, 0)
	var evaluateGroup func(models.RuleGroup, string) bool
	evaluateGroup = func(group models.RuleGroup, path string) bool {
		matches := make([]bool, 0, len(group.Conditions)+len(group.Groups))
		for index, condition := range group.Conditions {
			id := condition.ID
			if id == "" {
				id = fmt.Sprintf("%s.%d", path, index)
			}
			actual, exists := factValue(facts, condition.Fact)
			decision := models.ConditionDecision{ConditionID: id}
			if !exists {
				decision.ReasonCode, decision.MissingFact = "MISSING_FACT", condition.Fact
			} else {
				decision.Matched = compare(actual, condition.Operator, condition.Value)
				if !decision.Matched {
					decision.ReasonCode = "CONDITION_NOT_MATCHED"
				}
			}
			decisions = append(decisions, decision)
			matches = append(matches, decision.Matched)
		}
		for index, child := range group.Groups {
			matches = append(matches, evaluateGroup(child, fmt.Sprintf("%s.g%d", path, index)))
		}
		if group.Operator == "or" {
			return slices.Contains(matches, true)
		}
		return !slices.Contains(matches, false)
	}
	return evaluateGroup(*rule, "root"), decisions
}

func factValue(facts models.EvaluationFacts, name string) (any, bool) {
	switch name {
	case "member.id":
		if facts.Member != nil {
			return facts.Member.ID.String(), true
		}
	case "member.level":
		if facts.Member != nil {
			return facts.Member.Level, true
		}
	case "member.tags":
		if facts.Member != nil {
			return facts.Member.Tags, true
		}
	case "product.id":
		if facts.Product != nil {
			return facts.Product.ID.String(), true
		}
	case "product.category":
		if facts.Product != nil {
			return facts.Product.Category, true
		}
	case "product.price":
		if facts.Product != nil {
			return facts.Product.Price, true
		}
	case "product.status":
		if facts.Product != nil {
			return string(facts.Product.Status), true
		}
	case "cart.total_price":
		if facts.Cart != nil {
			return facts.Cart.TotalPrice, true
		}
	case "cart.item_count":
		if facts.Cart != nil {
			return facts.Cart.ItemCount, true
		}
	}
	return nil, false
}

func compare(actual any, operator string, raw json.RawMessage) bool {
	if operator == "in" {
		var values []json.RawMessage
		_ = json.Unmarshal(raw, &values)
		for _, value := range values {
			if compare(actual, "eq", value) {
				return true
			}
		}
		return false
	}
	switch value := actual.(type) {
	case string:
		var expected string
		return json.Unmarshal(raw, &expected) == nil && operator == "eq" && value == expected
	case []string:
		var expected string
		return json.Unmarshal(raw, &expected) == nil && operator == "contains" && slices.Contains(value, expected)
	case int64:
		var expected int64
		if json.Unmarshal(raw, &expected) != nil {
			return false
		}
		switch operator {
		case "eq":
			return value == expected
		case "gt":
			return value > expected
		case "gte":
			return value >= expected
		case "lt":
			return value < expected
		case "lte":
			return value <= expected
		}
	}
	return false
}

func firstFailure(decisions []models.ConditionDecision) (string, string) {
	for _, decision := range decisions {
		if !decision.Matched {
			return decision.ConditionID, decision.ReasonCode
		}
	}
	return "", "NOT_ELIGIBLE"
}

func missingFacts(decisions []models.ConditionDecision) []string {
	result := make([]string, 0)
	for _, decision := range decisions {
		if decision.MissingFact != "" && !slices.Contains(result, decision.MissingFact) {
			result = append(result, decision.MissingFact)
		}
	}
	return result
}

func normalizeRule(rule *models.RuleGroup) {
	if rule == nil {
		return
	}
	rule.Operator = strings.ToLower(strings.TrimSpace(rule.Operator))
	for index := range rule.Conditions {
		rule.Conditions[index].Fact = strings.ToLower(strings.TrimSpace(rule.Conditions[index].Fact))
		rule.Conditions[index].Operator = strings.ToLower(strings.TrimSpace(rule.Conditions[index].Operator))
		if rule.Conditions[index].Fact == "product.category" {
			rule.Conditions[index].Value = normalizeCategoryValue(rule.Conditions[index].Value, rule.Conditions[index].Operator)
		}
	}
	for index := range rule.Groups {
		normalizeRule(&rule.Groups[index])
	}
}

func normalizeCategoryValue(raw json.RawMessage, operator string) json.RawMessage {
	if operator == "in" {
		var values []string
		if json.Unmarshal(raw, &values) != nil {
			return raw
		}
		for index := range values {
			values[index] = normalizeCategory(values[index])
		}
		value, err := json.Marshal(values)
		if err == nil {
			return value
		}
		return raw
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return raw
	}
	normalized, err := json.Marshal(normalizeCategory(value))
	if err == nil {
		return normalized
	}
	return raw
}
