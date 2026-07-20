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
	"github.com/linenxing/e-commerce-system/models"
	notificationservice "github.com/linenxing/e-commerce-system/services/notification"
	notificationstore "github.com/linenxing/e-commerce-system/stores/notification"
)

const pollInterval = time.Second

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New("notification-worker", cfg.Environment, cfg.Log.Level)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx = logger.WithContext(ctx, log)
	db, err := postgres.NewPool(ctx, cfg.Database.URL, cfg.Database.MaxConnections)
	if err != nil {
		log.Fatal().Err(err).Msg("initialize postgres")
	}
	defer db.Close()
	store := notificationstore.NewPostgresStore(db)
	mock := notificationservice.NewMockProvider(store)
	worker := notificationservice.NewWorker(store, map[models.NotificationChannel]notificationservice.DeliveryProvider{
		models.NotificationChannelInApp: mock,
		models.NotificationChannelPush:  mock,
	})
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
				log.Error().Err(processErr).Msg("process notification batch")
				continue
			}
			if processed > 0 {
				log.Info().Int("task_count", processed).Msg("notification batch processed")
			}
		}
	}
}
