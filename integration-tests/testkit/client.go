package testkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/linenxing/e-commerce-system/base/response"
	"github.com/linenxing/e-commerce-system/models"
	authservice "github.com/linenxing/e-commerce-system/services/auth"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type ProductInput struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Category    string               `json:"category"`
	Price       int64                `json:"price"`
	Stock       int64                `json:"stock"`
	Status      models.ProductStatus `json:"status,omitempty"`
}

type CampaignInput struct {
	Name                  string                       `json:"name"`
	Description           string                       `json:"description"`
	Priority              int                          `json:"priority"`
	StartsAt              time.Time                    `json:"starts_at"`
	EndsAt                time.Time                    `json:"ends_at"`
	PromotionTitle        string                       `json:"promotion_title"`
	PromotionDescription  string                       `json:"promotion_description"`
	BenefitType           models.BenefitType           `json:"benefit_type"`
	BenefitValue          int64                        `json:"benefit_value"`
	MaximumDiscountAmount *int64                       `json:"maximum_discount_amount,omitempty"`
	ProductIDs            []uuid.UUID                  `json:"product_ids"`
	Categories            []string                     `json:"categories"`
	ContextType           models.EvaluationContextType `json:"context_type,omitempty"`
	EligibilityRule       *models.RuleGroup            `json:"eligibility_rule,omitempty"`
}

type AdminCampaign struct {
	models.Campaign
	RuleVersion     int                          `json:"rule_version"`
	RuleContextType models.EvaluationContextType `json:"rule_context_type,omitempty"`
	EligibilityRule *models.RuleGroup            `json:"eligibility_rule,omitempty"`
}

type RuleValidation struct {
	Valid            bool     `json:"valid"`
	ValidationErrors []string `json:"validation_errors"`
}

type Notification struct {
	ID        uuid.UUID  `json:"id"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	DeepLink  string     `json:"deep_link,omitempty"`
	OpenedAt  *time.Time `json:"opened_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func NewClient(environment Environment) *Client {
	return &Client{baseURL: strings.TrimRight(environment.BaseURL, "/"), httpClient: environment.HTTPClient}
}

func (c *Client) Health(ctx context.Context) error {
	return c.do(ctx, http.MethodGet, "/health", "", nil, http.StatusOK, nil)
}

func (c *Client) Register(ctx context.Context, email, password, name string) (authservice.AuthOutput, error) {
	var output authservice.AuthOutput
	err := c.do(ctx, http.MethodPost, "/auth/register", "", map[string]string{"email": email, "password": password, "name": name}, http.StatusCreated, &output)
	return output, err
}

func (c *Client) Login(ctx context.Context, email, password string) (authservice.AuthOutput, error) {
	var output authservice.AuthOutput
	err := c.do(ctx, http.MethodPost, "/auth/login", "", map[string]string{"email": email, "password": password}, http.StatusOK, &output)
	return output, err
}

func (c *Client) CurrentUser(ctx context.Context, token string) (models.UserResp, error) {
	var output models.UserResp
	err := c.do(ctx, http.MethodGet, "/users/me", token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) CreateProduct(ctx context.Context, token string, input ProductInput) (models.ProductResp, error) {
	var output models.ProductResp
	err := c.do(ctx, http.MethodPost, "/admin/products", token, input, http.StatusCreated, &output)
	return output, err
}

func (c *Client) UpdateProduct(ctx context.Context, token string, id uuid.UUID, input ProductInput) (models.ProductResp, error) {
	var output models.ProductResp
	err := c.do(ctx, http.MethodPut, "/admin/products/"+id.String(), token, input, http.StatusOK, &output)
	return output, err
}

func (c *Client) DisableProduct(ctx context.Context, token string, id uuid.UUID) error {
	return c.do(ctx, http.MethodDelete, "/admin/products/"+id.String(), token, nil, http.StatusNoContent, nil)
}

func (c *Client) GetProduct(ctx context.Context, id uuid.UUID) (models.ProductResp, error) {
	var output models.ProductResp
	err := c.do(ctx, http.MethodGet, "/products/"+id.String(), "", nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) ListProducts(ctx context.Context) ([]models.ProductResp, error) {
	var output []models.ProductResp
	err := c.do(ctx, http.MethodGet, "/products", "", nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) GetCart(ctx context.Context, token string) (models.CartResp, error) {
	var output models.CartResp
	err := c.do(ctx, http.MethodGet, "/cart", token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) AddCartItem(ctx context.Context, token string, productID uuid.UUID, quantity int64) (models.CartResp, error) {
	var output models.CartResp
	err := c.do(ctx, http.MethodPost, "/cart/items", token, map[string]any{"product_id": productID, "quantity": quantity}, http.StatusOK, &output)
	return output, err
}

func (c *Client) UpdateCartItem(ctx context.Context, token string, itemID uuid.UUID, quantity int64) (models.CartResp, error) {
	var output models.CartResp
	err := c.do(ctx, http.MethodPut, "/cart/items/"+itemID.String(), token, map[string]int64{"quantity": quantity}, http.StatusOK, &output)
	return output, err
}

func (c *Client) RemoveCartItem(ctx context.Context, token string, itemID uuid.UUID) error {
	return c.do(ctx, http.MethodDelete, "/cart/items/"+itemID.String(), token, nil, http.StatusNoContent, nil)
}

func (c *Client) CreateOrder(ctx context.Context, token string) (models.OrderResp, error) {
	var output models.OrderResp
	err := c.do(ctx, http.MethodPost, "/orders", token, nil, http.StatusCreated, &output)
	return output, err
}

func (c *Client) ListOrders(ctx context.Context, token string) ([]models.OrderResp, error) {
	var output []models.OrderResp
	err := c.do(ctx, http.MethodGet, "/orders", token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) GetOrder(ctx context.Context, token string, id uuid.UUID) (models.OrderResp, error) {
	var output models.OrderResp
	err := c.do(ctx, http.MethodGet, "/orders/"+id.String(), token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) CreateCampaign(ctx context.Context, token string, input CampaignInput) (AdminCampaign, error) {
	var output AdminCampaign
	err := c.do(ctx, http.MethodPost, "/admin/campaigns", token, input, http.StatusCreated, &output)
	return output, err
}

func (c *Client) UpdateCampaign(ctx context.Context, token string, id uuid.UUID, input CampaignInput) (AdminCampaign, error) {
	var output AdminCampaign
	err := c.do(ctx, http.MethodPut, "/admin/campaigns/"+id.String(), token, input, http.StatusOK, &output)
	return output, err
}

func (c *Client) GetAdminCampaign(ctx context.Context, token string, id uuid.UUID) (AdminCampaign, error) {
	var output AdminCampaign
	err := c.do(ctx, http.MethodGet, "/admin/campaigns/"+id.String(), token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) ListAdminCampaigns(ctx context.Context, token string) ([]AdminCampaign, error) {
	var output []AdminCampaign
	err := c.do(ctx, http.MethodGet, "/admin/campaigns", token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) PublishCampaign(ctx context.Context, token string, id uuid.UUID) (AdminCampaign, error) {
	return c.transitionCampaign(ctx, token, id, "publish")
}

func (c *Client) PauseCampaign(ctx context.Context, token string, id uuid.UUID) (AdminCampaign, error) {
	return c.transitionCampaign(ctx, token, id, "pause")
}

func (c *Client) ResumeCampaign(ctx context.Context, token string, id uuid.UUID) (AdminCampaign, error) {
	return c.transitionCampaign(ctx, token, id, "resume")
}

func (c *Client) ArchiveCampaign(ctx context.Context, token string, id uuid.UUID) (AdminCampaign, error) {
	return c.transitionCampaign(ctx, token, id, "archive")
}

func (c *Client) ValidateCampaignRules(ctx context.Context, token string, id uuid.UUID) (RuleValidation, error) {
	var output RuleValidation
	err := c.do(ctx, http.MethodPost, "/admin/campaigns/"+id.String()+"/rules/validate", token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) EvaluateCampaignRules(ctx context.Context, token string, id uuid.UUID, contextType models.EvaluationContextType, facts models.EvaluationFacts) (models.EvaluationResult, error) {
	var output models.EvaluationResult
	body := map[string]any{"context_type": contextType, "facts": facts}
	err := c.do(ctx, http.MethodPost, "/admin/campaigns/"+id.String()+"/rules/evaluate", token, body, http.StatusOK, &output)
	return output, err
}

func (c *Client) ListPublicCampaigns(ctx context.Context, token string, productID *uuid.UUID) ([]models.Campaign, error) {
	var output []models.Campaign
	err := c.do(ctx, http.MethodGet, campaignPath("/campaigns", productID), token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) ListPublicCampaignPage(ctx context.Context, token string, productID *uuid.UUID, limit, offset int) ([]models.Campaign, error) {
	var output []models.Campaign
	path := campaignPath("/campaigns", productID)
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	path += fmt.Sprintf("%slimit=%d&offset=%d", separator, limit, offset)
	err := c.do(ctx, http.MethodGet, path, token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) GetPublicCampaign(ctx context.Context, token string, id uuid.UUID, productID *uuid.UUID) (models.Campaign, error) {
	var output models.Campaign
	err := c.do(ctx, http.MethodGet, campaignPath("/campaigns/"+id.String(), productID), token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) EvaluatePublicCampaign(ctx context.Context, token string, id uuid.UUID, productID *uuid.UUID) (models.EvaluationResult, error) {
	var output models.EvaluationResult
	err := c.do(ctx, http.MethodPost, campaignPath("/campaigns/"+id.String()+"/evaluate", productID), token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) GetNotificationPreferences(ctx context.Context, token string) (models.NotificationPreferences, error) {
	var output models.NotificationPreferences
	err := c.do(ctx, http.MethodGet, "/me/notification-preferences", token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) UpdateNotificationPreferences(ctx context.Context, token string, preferences models.NotificationPreferences) (models.NotificationPreferences, error) {
	var output models.NotificationPreferences
	err := c.do(ctx, http.MethodPut, "/me/notification-preferences", token, preferences, http.StatusOK, &output)
	return output, err
}

func (c *Client) ListNotifications(ctx context.Context, token string) ([]Notification, error) {
	var output []Notification
	err := c.do(ctx, http.MethodGet, "/notifications", token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) OpenNotification(ctx context.Context, token string, id uuid.UUID) error {
	return c.do(ctx, http.MethodPost, "/notifications/"+id.String()+"/open", token, nil, http.StatusNoContent, nil)
}

func (c *Client) ListAdminNotificationTasks(ctx context.Context, token string) ([]models.NotificationTask, error) {
	var output []models.NotificationTask
	err := c.do(ctx, http.MethodGet, "/admin/notification-tasks", token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) GetAdminNotificationTask(ctx context.Context, token string, id uuid.UUID) (models.NotificationTask, error) {
	var output models.NotificationTask
	err := c.do(ctx, http.MethodGet, "/admin/notification-tasks/"+id.String(), token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) ListCartRecallJourneys(ctx context.Context, token string) ([]models.CartRecallJourney, error) {
	var output []models.CartRecallJourney
	err := c.do(ctx, http.MethodGet, "/admin/cart-recall-journeys", token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) GetCartRecallJourney(ctx context.Context, token string, id uuid.UUID) (models.CartRecallJourney, error) {
	var output models.CartRecallJourney
	err := c.do(ctx, http.MethodGet, "/admin/cart-recall-journeys/"+id.String(), token, nil, http.StatusOK, &output)
	return output, err
}

func (c *Client) CancelCartRecallJourney(ctx context.Context, token string, id uuid.UUID) error {
	return c.do(ctx, http.MethodPost, "/admin/cart-recall-journeys/"+id.String()+"/cancel", token, nil, http.StatusNoContent, nil)
}

func (c *Client) transitionCampaign(ctx context.Context, token string, id uuid.UUID, action string) (AdminCampaign, error) {
	var output AdminCampaign
	err := c.do(ctx, http.MethodPost, "/admin/campaigns/"+id.String()+"/"+action, token, nil, http.StatusOK, &output)
	return output, err
}

func campaignPath(path string, productID *uuid.UUID) string {
	if productID == nil {
		return path
	}
	values := url.Values{"product_id": []string{productID.String()}}
	return path + "?" + values.Encode()
}

func (c *Client) ExpectError(ctx context.Context, method, path, token string, body any, status int, code string) error {
	var output response.ErrorBody
	if err := c.do(ctx, method, path, token, body, status, &output); err != nil {
		return err
	}
	if output.Code != code {
		return fmt.Errorf("expected error code %q, got %q", code, output.Code)
	}
	return nil
}

func (c *Client) do(ctx context.Context, method, path, token string, body any, expectedStatus int, output any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode %s %s request: %w", method, path, err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return fmt.Errorf("create %s %s request: %w", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send %s %s request: %w", method, path, err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read %s %s response: %w", method, path, err)
	}
	if resp.StatusCode != expectedStatus {
		return fmt.Errorf("%s %s returned %s, expected %d: %s", method, path, resp.Status, expectedStatus, strings.TrimSpace(string(payload)))
	}
	if output == nil || len(payload) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, output); err != nil {
		return fmt.Errorf("decode %s %s response: %w", method, path, err)
	}
	return nil
}
