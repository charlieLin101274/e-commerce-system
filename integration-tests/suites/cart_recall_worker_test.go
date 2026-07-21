//go:build integration

package suites

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/integration-tests/testkit"
	"github.com/linenxing/e-commerce-system/models"
	authservice "github.com/linenxing/e-commerce-system/services/auth"
)

// These tests rely on APP_CART_RECALL_DELAY=1s from compose.override.yaml.
const cartRecallWaitTimeout = 20 * time.Second

func TestCartRecallWorkerEligibleJourneyReachesSent(t *testing.T) {
	admin, customer, product, campaign := setupCartRecallScenario(t, 10)
	cart, err := testClient.AddCartItem(t.Context(), customer.AccessToken, product.ID, 2)
	if err != nil {
		t.Fatalf("add cart item: %v", err)
	}

	journey := waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
		return value.Status == models.CartRecallSent
	})
	if journey.CartID != cart.ID || journey.CampaignID == nil || *journey.CampaignID != campaign.ID {
		t.Fatalf("sent journey = %#v", journey)
	}
	if journey.NotificationTaskID == nil || journey.RuleVersion != campaign.RuleVersion || len(journey.MatchedProductIDs) != 1 || journey.MatchedProductIDs[0] != product.ID {
		t.Fatalf("sent journey metadata = %#v", journey)
	}
}

func TestCartRecallWorkerReschedulesAfterCartMutation(t *testing.T) {
	admin, customer, product, _ := setupCartRecallScenario(t, 10)
	cart, err := testClient.AddCartItem(t.Context(), customer.AccessToken, product.ID, 1)
	if err != nil {
		t.Fatalf("add cart item: %v", err)
	}
	scheduled := waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
		return value.Status == models.CartRecallScheduled
	})

	if _, err := testClient.UpdateCartItem(t.Context(), customer.AccessToken, cart.Items[0].ID, 2); err != nil {
		t.Fatalf("update cart item: %v", err)
	}
	rescheduled := waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
		return value.ID == scheduled.ID && value.SourceEventID != scheduled.SourceEventID && value.EvaluateAt.After(scheduled.EvaluateAt)
	})
	if rescheduled.CartID != cart.ID {
		t.Fatalf("rescheduled journey cart ID = %s, want %s", rescheduled.CartID, cart.ID)
	}

	sent := waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
		return value.ID == scheduled.ID && value.Status == models.CartRecallSent
	})
	if sent.NotificationTaskID == nil {
		t.Fatalf("rescheduled journey was sent without notification task: %#v", sent)
	}
}

func TestCartRecallWorkerSkipsUnavailableDeliveryPreferences(t *testing.T) {
	tests := []struct {
		name        string
		preferences models.NotificationPreferences
		reason      string
	}{
		{name: "marketing consent disabled", preferences: models.NotificationPreferences{Channels: []models.NotificationChannel{models.NotificationChannelInApp}}, reason: "MARKETING_CONSENT_DISABLED"},
		{name: "notification channel disabled", preferences: models.NotificationPreferences{MarketingConsent: true}, reason: "NO_NOTIFICATION_CHANNEL"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			admin := testScenario.LoginAdmin(t)
			customer := testScenario.CreateCustomer(t)
			product := testScenario.CreateProduct(t, admin.AccessToken, 1000, 10)
			if _, err := testClient.UpdateNotificationPreferences(t.Context(), customer.AccessToken, test.preferences); err != nil {
				t.Fatalf("update notification preferences: %v", err)
			}
			if _, err := testClient.AddCartItem(t.Context(), customer.AccessToken, product.ID, 1); err != nil {
				t.Fatalf("add cart item: %v", err)
			}

			journey := waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
				return value.Status == models.CartRecallSkipped && value.CancelReason == test.reason
			})
			if journey.NotificationTaskID != nil {
				t.Fatalf("skipped journey unexpectedly has notification task: %#v", journey)
			}
		})
	}
}

func TestCartRecallWorkerSkipsWhenNoCampaignIsEligible(t *testing.T) {
	admin := testScenario.LoginAdmin(t)
	customer := testScenario.CreateCustomer(t)
	product := testScenario.CreateProduct(t, admin.AccessToken, 1000, 10)
	enableInAppNotifications(t, customer.AccessToken)
	if _, err := testClient.AddCartItem(t.Context(), customer.AccessToken, product.ID, 1); err != nil {
		t.Fatalf("add cart item: %v", err)
	}

	journey := waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
		return value.Status == models.CartRecallSkipped && value.CancelReason == "NO_ELIGIBLE_CAMPAIGN"
	})
	if journey.CampaignID != nil || journey.NotificationTaskID != nil {
		t.Fatalf("ineligible journey contains campaign or task: %#v", journey)
	}
}

func TestCartRecallWorkerCancelsWhenCartBecomesEmpty(t *testing.T) {
	admin, customer, product, _ := setupCartRecallScenario(t, 10)
	cart, err := testClient.AddCartItem(t.Context(), customer.AccessToken, product.ID, 1)
	if err != nil {
		t.Fatalf("add cart item: %v", err)
	}
	if err := testClient.RemoveCartItem(t.Context(), customer.AccessToken, cart.Items[0].ID); err != nil {
		t.Fatalf("remove cart item: %v", err)
	}

	journey := waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
		return value.Status == models.CartRecallCancelled && value.CancelReason == "CART_EMPTY"
	})
	if journey.NotificationTaskID != nil {
		t.Fatalf("empty-cart journey unexpectedly has notification task: %#v", journey)
	}
}

func TestCartRecallWorkerSkipsInactiveAndOutOfStockProducts(t *testing.T) {
	t.Run("inactive product", func(t *testing.T) {
		admin, customer, product, _ := setupCartRecallScenario(t, 10)
		if _, err := testClient.AddCartItem(t.Context(), customer.AccessToken, product.ID, 1); err != nil {
			t.Fatalf("add cart item: %v", err)
		}
		if err := testClient.DisableProduct(t.Context(), admin.AccessToken, product.ID); err != nil {
			t.Fatalf("disable product before recall evaluation: %v", err)
		}
		waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
			return value.Status == models.CartRecallSkipped && value.CancelReason == "PRODUCT_INACTIVE"
		})
	})

	t.Run("insufficient stock", func(t *testing.T) {
		admin, customer, product, _ := setupCartRecallScenario(t, 2)
		buyer := testScenario.CreateCustomer(t)
		if _, err := testClient.AddCartItem(t.Context(), customer.AccessToken, product.ID, 2); err != nil {
			t.Fatalf("add recall cart item: %v", err)
		}
		if _, err := testClient.AddCartItem(t.Context(), buyer.AccessToken, product.ID, 1); err != nil {
			t.Fatalf("add competing cart item: %v", err)
		}
		if _, err := testClient.CreateOrder(t.Context(), buyer.AccessToken); err != nil {
			t.Fatalf("create competing order: %v", err)
		}
		waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
			return value.Status == models.CartRecallSkipped && value.CancelReason == "OUT_OF_STOCK"
		})
	})
}

func TestCartRecallWorkerCancelsWhenOrderCompletesBeforeDelivery(t *testing.T) {
	admin, customer, product, _ := setupCartRecallScenario(t, 10)
	if _, err := testClient.AddCartItem(t.Context(), customer.AccessToken, product.ID, 1); err != nil {
		t.Fatalf("add cart item: %v", err)
	}
	order, err := testClient.CreateOrder(t.Context(), customer.AccessToken)
	if err != nil {
		t.Fatalf("create order before recall delivery: %v", err)
	}

	journey := waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
		return value.Status == models.CartRecallCancelled && value.CancelReason == "ORDER_ALREADY_COMPLETED"
	})
	if journey.ConvertedOrderID != nil || journey.NotificationTaskID != nil {
		t.Fatalf("pre-delivery order was incorrectly attributed: journey=%#v order=%s", journey, order.ID)
	}
}

func TestCartRecallWorkerAttributesOrderAfterNotification(t *testing.T) {
	admin, customer, product, _ := setupCartRecallScenario(t, 10)
	if _, err := testClient.AddCartItem(t.Context(), customer.AccessToken, product.ID, 1); err != nil {
		t.Fatalf("add cart item: %v", err)
	}
	sent := waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
		return value.Status == models.CartRecallSent
	})
	order, err := testClient.CreateOrder(t.Context(), customer.AccessToken)
	if err != nil {
		t.Fatalf("create attributed order: %v", err)
	}

	converted := waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
		return value.ID == sent.ID && value.Status == models.CartRecallConverted
	})
	if converted.ConvertedOrderID == nil || *converted.ConvertedOrderID != order.ID {
		t.Fatalf("converted journey = %#v, want order %s", converted, order.ID)
	}
}

func TestCartRecallWorkerEnforcesCampaignFrequencyLimit(t *testing.T) {
	admin, customer, product, _ := setupCartRecallScenario(t, 20)
	firstCart, err := testClient.AddCartItem(t.Context(), customer.AccessToken, product.ID, 1)
	if err != nil {
		t.Fatalf("add first cart item: %v", err)
	}
	first := waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
		return value.Status == models.CartRecallSent
	})
	if _, err := testClient.UpdateCartItem(t.Context(), customer.AccessToken, firstCart.Items[0].ID, 2); err != nil {
		t.Fatalf("mutate cart for second recall: %v", err)
	}

	second := waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
		return value.ID != first.ID && value.Status == models.CartRecallSkipped && value.CancelReason == "FREQUENCY_LIMIT_REACHED"
	})
	if second.NotificationTaskID != nil {
		t.Fatalf("frequency-limited journey unexpectedly has task: %#v", second)
	}
}

func TestCartRecallWorkerSelectsCampaignDeterministically(t *testing.T) {
	tests := []struct {
		name       string
		priorities [2]int
	}{
		{name: "higher priority wins", priorities: [2]int{10, 20}},
		{name: "lexical campaign ID breaks priority tie", priorities: [2]int{10, 10}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			admin := testScenario.LoginAdmin(t)
			customer := testScenario.CreateCustomer(t)
			product := testScenario.CreateProduct(t, admin.AccessToken, 1000, 10)
			first := testScenario.CreateCartRecallCampaignWithPriority(t, admin.AccessToken, product.ID, test.priorities[0])
			second := testScenario.CreateCartRecallCampaignWithPriority(t, admin.AccessToken, product.ID, test.priorities[1])
			for _, campaign := range []testkit.AdminCampaign{first, second} {
				if _, err := testClient.PublishCampaign(t.Context(), admin.AccessToken, campaign.ID); err != nil {
					t.Fatalf("publish campaign %s: %v", campaign.ID, err)
				}
			}
			enableInAppNotifications(t, customer.AccessToken)
			if _, err := testClient.AddCartItem(t.Context(), customer.AccessToken, product.ID, 1); err != nil {
				t.Fatalf("add cart item: %v", err)
			}

			expected := first
			if second.Priority > first.Priority || (second.Priority == first.Priority && second.ID.String() < first.ID.String()) {
				expected = second
			}
			journey := waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
				return value.Status == models.CartRecallSent
			})
			if journey.CampaignID == nil || *journey.CampaignID != expected.ID {
				t.Fatalf("selected campaign=%v want=%s (first=%s/%d second=%s/%d)", journey.CampaignID, expected.ID, first.ID, first.Priority, second.ID, second.Priority)
			}
		})
	}
}

func TestUnrelatedOrderDoesNotCancelSentCartRecall(t *testing.T) {
	admin, customer, recalledProduct, _ := setupCartRecallScenario(t, 10)
	firstCart, err := testClient.AddCartItem(t.Context(), customer.AccessToken, recalledProduct.ID, 1)
	if err != nil {
		t.Fatalf("add recalled product: %v", err)
	}
	sent := waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
		return value.Status == models.CartRecallSent
	})
	if err := testClient.RemoveCartItem(t.Context(), customer.AccessToken, firstCart.Items[0].ID); err != nil {
		t.Fatalf("remove recalled product: %v", err)
	}

	unrelatedProduct := testScenario.CreateProduct(t, admin.AccessToken, 500, 10)
	if _, err := testClient.AddCartItem(t.Context(), customer.AccessToken, unrelatedProduct.ID, 1); err != nil {
		t.Fatalf("add unrelated product: %v", err)
	}
	if _, err := testClient.CreateOrder(t.Context(), customer.AccessToken); err != nil {
		t.Fatalf("create unrelated order: %v", err)
	}
	waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
		return value.ID != sent.ID && value.Status == models.CartRecallCancelled && value.CancelReason == "ORDER_ALREADY_COMPLETED"
	})

	unchanged, err := testClient.GetCartRecallJourney(t.Context(), admin.AccessToken, sent.ID)
	if err != nil {
		t.Fatalf("get original sent journey: %v", err)
	}
	if unchanged.Status != models.CartRecallSent || unchanged.ConvertedOrderID != nil || unchanged.CancelReason != "" {
		t.Fatalf("unrelated order changed sent journey: %#v", unchanged)
	}
}

func TestCartRecallAdminCanCancelScheduledJourney(t *testing.T) {
	admin, customer, product, _ := setupCartRecallScenario(t, 10)
	if _, err := testClient.AddCartItem(t.Context(), customer.AccessToken, product.ID, 1); err != nil {
		t.Fatalf("add cart item: %v", err)
	}
	scheduled := waitForCartRecallJourney(t, admin.AccessToken, customer.User.ID, func(value models.CartRecallJourney) bool {
		return value.Status == models.CartRecallScheduled
	})
	if err := testClient.CancelCartRecallJourney(t.Context(), admin.AccessToken, scheduled.ID); err != nil {
		t.Fatalf("cancel scheduled journey: %v", err)
	}
	if err := testClient.CancelCartRecallJourney(t.Context(), admin.AccessToken, scheduled.ID); err != nil {
		t.Fatalf("repeat cancel must be idempotent: %v", err)
	}
	cancelled, err := testClient.GetCartRecallJourney(t.Context(), admin.AccessToken, scheduled.ID)
	if err != nil {
		t.Fatalf("get cancelled journey: %v", err)
	}
	if cancelled.Status != models.CartRecallCancelled || cancelled.CancelReason != "ADMIN_CANCELLED" {
		t.Fatalf("admin-cancelled journey = %#v", cancelled)
	}
}

func setupCartRecallScenario(t *testing.T, stock int64) (authservice.AuthOutput, authservice.AuthOutput, models.ProductResp, testkit.AdminCampaign) {
	t.Helper()
	admin := testScenario.LoginAdmin(t)
	customer := testScenario.CreateCustomer(t)
	product := testScenario.CreateProduct(t, admin.AccessToken, 1000, stock)
	campaign := testScenario.CreateCartRecallCampaign(t, admin.AccessToken, product.ID)
	if _, err := testClient.PublishCampaign(t.Context(), admin.AccessToken, campaign.ID); err != nil {
		t.Fatalf("publish cart recall campaign: %v", err)
	}
	enableInAppNotifications(t, customer.AccessToken)
	return admin, customer, product, campaign
}

func enableInAppNotifications(t *testing.T, token string) {
	t.Helper()
	if _, err := testClient.UpdateNotificationPreferences(t.Context(), token, models.NotificationPreferences{
		MarketingConsent: true,
		Channels:         []models.NotificationChannel{models.NotificationChannelInApp},
	}); err != nil {
		t.Fatalf("enable in-app notifications: %v", err)
	}
}

func waitForCartRecallJourney(t *testing.T, token string, userID uuid.UUID, predicate func(models.CartRecallJourney) bool) models.CartRecallJourney {
	t.Helper()
	waitContext, cancel := context.WithTimeout(t.Context(), cartRecallWaitTimeout)
	defer cancel()
	var matched models.CartRecallJourney
	var lastObserved []models.CartRecallJourney
	err := testkit.Eventually(waitContext, 200*time.Millisecond, func() (bool, error) {
		journeys, listErr := testClient.ListCartRecallJourneys(waitContext, token)
		if listErr != nil {
			return false, listErr
		}
		lastObserved = lastObserved[:0]
		for _, journey := range journeys {
			if journey.UserID != userID {
				continue
			}
			lastObserved = append(lastObserved, journey)
			if predicate(journey) {
				matched = journey
				return true, nil
			}
		}
		return false, fmt.Errorf("no matching cart recall journey for user %s; observed: %s", userID, summarizeJourneys(lastObserved))
	})
	if err != nil {
		t.Fatalf("wait for cart recall journey: %v", err)
	}
	return matched
}

func summarizeJourneys(journeys []models.CartRecallJourney) string {
	if len(journeys) == 0 {
		return "none"
	}
	result := ""
	for index, journey := range journeys {
		if index > 0 {
			result += ", "
		}
		result += fmt.Sprintf("%s(status=%s reason=%s campaign=%v task=%v)", journey.ID, journey.Status, journey.CancelReason, journey.CampaignID, journey.NotificationTaskID)
	}
	return result
}
