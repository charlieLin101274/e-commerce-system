package testkit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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
