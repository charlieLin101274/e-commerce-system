package config

import (
	"errors"
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

const minimumJWTSecretLength = 32

type Config struct {
	Environment string `env:"APP_ENV" envDefault:"development"`
	HTTP        HTTPConfig
	Database    DatabaseConfig
	JWT         JWTConfig
	Log         LogConfig
}

type HTTPConfig struct {
	Address string `env:"APP_HTTP_ADDRESS" envDefault:":8080"`
}

type DatabaseConfig struct {
	URL            string `env:"APP_DATABASE_URL,required,notEmpty"`
	MaxConnections int32  `env:"APP_DATABASE_MAX_CONNECTIONS" envDefault:"10"`
}

type JWTConfig struct {
	Secret     string        `env:"APP_JWT_SECRET,required,notEmpty"`
	Issuer     string        `env:"APP_JWT_ISSUER" envDefault:"e-commerce-system"`
	Expiration time.Duration `env:"APP_JWT_EXPIRATION" envDefault:"24h"`
}

type LogConfig struct {
	Level string `env:"APP_LOG_LEVEL" envDefault:"info"`
}

func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("parse environment config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Validate() error {
	if c.Database.URL == "" {
		return errors.New("APP_DATABASE_URL is required")
	}
	if len(c.JWT.Secret) < minimumJWTSecretLength {
		return fmt.Errorf("APP_JWT_SECRET must contain at least %d characters", minimumJWTSecretLength)
	}
	if c.Database.MaxConnections <= 0 {
		return errors.New("APP_DATABASE_MAX_CONNECTIONS must be greater than zero")
	}
	if c.JWT.Expiration <= 0 {
		return errors.New("APP_JWT_EXPIRATION must be greater than zero")
	}
	return nil
}
