package testkit

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultBaseURL       = "http://localhost:18080"
	defaultAdminEmail    = "admin@example.com"
	defaultAdminPassword = "Admin123!"
)

type Environment struct {
	BaseURL       string
	AdminEmail    string
	AdminPassword string
	HTTPClient    *http.Client
}

func LoadEnvironment() Environment {
	return Environment{
		BaseURL:       envOrDefault("INTEGRATION_API_URL", defaultBaseURL),
		AdminEmail:    envOrDefault("INTEGRATION_ADMIN_EMAIL", defaultAdminEmail),
		AdminPassword: envOrDefault("INTEGRATION_ADMIN_PASSWORD", defaultAdminPassword),
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (e Environment) WaitForAPI(ctx context.Context) error {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	var lastErr error
	for {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(e.BaseURL, "/")+"/health", nil)
		if err != nil {
			return fmt.Errorf("create health request: %w", err)
		}
		resp, err := e.HTTPClient.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("health endpoint returned %s", resp.Status)
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("wait for API readiness: %w: %v", ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
