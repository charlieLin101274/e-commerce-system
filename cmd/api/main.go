package main

import (
	"context"

	"github.com/linenxing/e-commerce-system/base/config"
	"github.com/linenxing/e-commerce-system/base/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	log := logger.New("e-commerce-api", cfg.Environment, cfg.Log.Level)
	app, err := NewApplication(context.Background(), cfg, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize application")
	}
	defer app.Close()

	if err := app.Run(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("application stopped")
	}
}
