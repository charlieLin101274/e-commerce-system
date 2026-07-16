package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadUsesStructTagDefaults(t *testing.T) {
	t.Setenv("APP_DATABASE_URL", "postgres://localhost/ecommerce")
	t.Setenv("APP_JWT_SECRET", strings.Repeat("a", minimumJWTSecretLength))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Environment != "development" {
		t.Fatalf("unexpected environment: %s", cfg.Environment)
	}
	if cfg.HTTP.Address != ":8080" {
		t.Fatalf("unexpected HTTP address: %s", cfg.HTTP.Address)
	}
	if cfg.Database.MaxConnections != 10 {
		t.Fatalf("unexpected max connections: %d", cfg.Database.MaxConnections)
	}
	if cfg.JWT.Expiration != 24*time.Hour {
		t.Fatalf("unexpected JWT expiration: %s", cfg.JWT.Expiration)
	}
}

func TestLoadParsesEnvironmentValues(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("APP_HTTP_ADDRESS", ":9090")
	t.Setenv("APP_DATABASE_URL", "postgres://localhost/ecommerce")
	t.Setenv("APP_DATABASE_MAX_CONNECTIONS", "20")
	t.Setenv("APP_JWT_SECRET", strings.Repeat("b", minimumJWTSecretLength))
	t.Setenv("APP_JWT_ISSUER", "test-issuer")
	t.Setenv("APP_JWT_EXPIRATION", "30m")
	t.Setenv("APP_LOG_LEVEL", "warn")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Environment != "test" ||
		cfg.HTTP.Address != ":9090" ||
		cfg.Database.MaxConnections != 20 ||
		cfg.JWT.Issuer != "test-issuer" ||
		cfg.JWT.Expiration != 30*time.Minute ||
		cfg.Log.Level != "warn" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
}

func TestLoadRejectsShortJWTSecret(t *testing.T) {
	t.Setenv("APP_DATABASE_URL", "postgres://localhost/ecommerce")
	t.Setenv("APP_JWT_SECRET", "short")

	_, err := Load()
	if err == nil {
		t.Fatal("expected validation error")
	}
}
