package main

import (
	"context"
	"errors"
	"os/signal"
	"syscall"
	"time"

	"github.com/linenxing/e-commerce-system/base/config"
	"github.com/linenxing/e-commerce-system/base/logger"
	"github.com/linenxing/e-commerce-system/base/postgres"
	campaignservice "github.com/linenxing/e-commerce-system/services/campaign"
	cartrecallservice "github.com/linenxing/e-commerce-system/services/cartrecall"
	notificationservice "github.com/linenxing/e-commerce-system/services/notification"
	campaignstore "github.com/linenxing/e-commerce-system/stores/campaign"
	cartrecallstore "github.com/linenxing/e-commerce-system/stores/cartrecall"
	notificationstore "github.com/linenxing/e-commerce-system/stores/notification"
)

const pollInterval = time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New("cart-recall-worker", cfg.Environment, cfg.Log.Level)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx = logger.WithContext(ctx, log)
	db, err := postgres.NewPool(ctx, cfg.Database.URL, cfg.Database.MaxConnections)
	if err != nil {
		log.Fatal().Err(err).Msg("initialize postgres")
	}
	defer db.Close()
	journeys := cartrecallstore.NewPostgresStore(db)
	worker := cartrecallservice.NewWorker(journeys, campaignservice.New(campaignstore.NewPostgresStore(db)), notificationservice.New(notificationstore.NewPostgresStore(db)), cfg.CartRecall.Delay)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if !errors.Is(ctx.Err(), context.Canceled) {
				log.Error().Err(ctx.Err()).Msg("worker stopped")
			}
			return
		case <-ticker.C:
			processed, processErr := worker.ProcessBatch(ctx)
			if processErr != nil {
				log.Error().Err(processErr).Msg("process cart recall batch")
				continue
			}
			if processed > 0 {
				log.Info().Int("item_count", processed).Msg("cart recall batch processed")
			}
		}
	}
}
