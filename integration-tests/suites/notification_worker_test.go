//go:build integration

package suites

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/linenxing/e-commerce-system/integration-tests/testkit"
	"github.com/linenxing/e-commerce-system/models"
)

func TestNotificationWorkerDeliversCartRecallInAppNotification(t *testing.T) {
	admin := testScenario.LoginAdmin(t)
	customer := testScenario.CreateCustomer(t)
	product := testScenario.CreateProduct(t, admin.AccessToken, 1000, 10)
	campaign := testScenario.CreateCartRecallCampaign(t, admin.AccessToken, product.ID)
	if _, err := testClient.PublishCampaign(t.Context(), admin.AccessToken, campaign.ID); err != nil {
		t.Fatalf("publish cart recall campaign: %v", err)
	}

	preferences, err := testClient.UpdateNotificationPreferences(t.Context(), customer.AccessToken, models.NotificationPreferences{
		MarketingConsent: true,
		Channels:         []models.NotificationChannel{models.NotificationChannelInApp},
	})
	if err != nil {
		t.Fatalf("update notification preferences: %v", err)
	}
	if !preferences.MarketingConsent || len(preferences.Channels) != 1 || preferences.Channels[0] != models.NotificationChannelInApp {
		t.Fatalf("updated notification preferences = %#v", preferences)
	}

	if _, err := testClient.AddCartItem(t.Context(), customer.AccessToken, product.ID, 1); err != nil {
		t.Fatalf("add cart item to trigger recall: %v", err)
	}

	waitContext, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	var delivered models.NotificationTask
	err = testkit.Eventually(waitContext, 250*time.Millisecond, func() (bool, error) {
		tasks, listErr := testClient.ListAdminNotificationTasks(waitContext, admin.AccessToken)
		if listErr != nil {
			return false, listErr
		}
		for _, task := range tasks {
			if task.UserID == customer.User.ID && task.CampaignID != nil && *task.CampaignID == campaign.ID {
				delivered = task
				return task.Status == models.NotificationTaskDelivered, nil
			}
		}
		return false, fmt.Errorf("notification task for customer %s and campaign %s was not found", customer.User.ID, campaign.ID)
	})
	if err != nil {
		t.Fatalf("wait for notification worker delivery: %v", err)
	}
	if delivered.Channel != models.NotificationChannelInApp || delivered.JourneyType != "cart_recall" || delivered.AttemptCount < 1 || delivered.SentAt == nil {
		t.Fatalf("delivered notification task = %#v", delivered)
	}

	var notification testkit.Notification
	err = testkit.Eventually(waitContext, 250*time.Millisecond, func() (bool, error) {
		notifications, listErr := testClient.ListNotifications(waitContext, customer.AccessToken)
		if listErr != nil {
			return false, listErr
		}
		for _, item := range notifications {
			if item.ID == delivered.ID {
				notification = item
				return true, nil
			}
		}
		return false, fmt.Errorf("delivered notification %s is not visible to customer", delivered.ID)
	})
	if err != nil {
		t.Fatalf("wait for customer notification: %v", err)
	}
	if notification.Title != campaign.PromotionTitle || !strings.Contains(notification.DeepLink, product.ID.String()) || notification.OpenedAt != nil {
		t.Fatalf("customer notification = %#v", notification)
	}

	if err := testClient.OpenNotification(t.Context(), customer.AccessToken, notification.ID); err != nil {
		t.Fatalf("open notification: %v", err)
	}
	opened, err := testClient.GetAdminNotificationTask(t.Context(), admin.AccessToken, notification.ID)
	if err != nil {
		t.Fatalf("get opened notification task: %v", err)
	}
	if opened.Status != models.NotificationTaskOpened || opened.OpenedAt == nil {
		t.Fatalf("opened notification task = %#v", opened)
	}
}
