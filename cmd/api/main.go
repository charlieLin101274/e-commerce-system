package main

import (
	"context"

	"github.com/linenxing/e-commerce-system/base/config"
	"github.com/linenxing/e-commerce-system/base/logger"
)

// @title Simple E-Commerce API
// @version 1.0
// @description Backend APIs for the simple e-commerce MVP.
// @BasePath /
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter the token with the Bearer prefix, for example: Bearer <token>
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
