package logger

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

func New(service, environment, level string) zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	parsedLevel, err := zerolog.ParseLevel(strings.ToLower(level))
	if err != nil {
		parsedLevel = zerolog.InfoLevel
	}

	return zerolog.New(os.Stdout).
		Level(parsedLevel).
		With().
		Timestamp().
		Str("service", service).
		Str("environment", environment).
		Logger()
}

func WithContext(ctx context.Context, log zerolog.Logger) context.Context {
	return log.WithContext(ctx)
}

func FromContext(ctx context.Context) *zerolog.Logger {
	return zerolog.Ctx(ctx)
}
