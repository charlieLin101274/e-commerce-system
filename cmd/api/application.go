package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/linenxing/e-commerce-system/base/auth"
	"github.com/linenxing/e-commerce-system/base/config"
	"github.com/linenxing/e-commerce-system/base/logger"
	"github.com/linenxing/e-commerce-system/base/postgres"
	"github.com/linenxing/e-commerce-system/cmd/api/apis"
	_ "github.com/linenxing/e-commerce-system/docs/swagger"
	"github.com/linenxing/e-commerce-system/middlewares"
	authservice "github.com/linenxing/e-commerce-system/services/auth"
	campaignservice "github.com/linenxing/e-commerce-system/services/campaign"
	cartservice "github.com/linenxing/e-commerce-system/services/cart"
	orderservice "github.com/linenxing/e-commerce-system/services/order"
	productservice "github.com/linenxing/e-commerce-system/services/product"
	campaignstore "github.com/linenxing/e-commerce-system/stores/campaign"
	cartstore "github.com/linenxing/e-commerce-system/stores/cart"
	orderstore "github.com/linenxing/e-commerce-system/stores/order"
	productstore "github.com/linenxing/e-commerce-system/stores/product"
	userstore "github.com/linenxing/e-commerce-system/stores/user"
	"github.com/rs/zerolog"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

const (
	readHeaderTimeout = 5 * time.Second
	shutdownTimeout   = 10 * time.Second
)

type Application struct {
	logger zerolog.Logger
	db     *pgxpool.Pool
	server *http.Server
}

func NewApplication(ctx context.Context, cfg config.Config, log zerolog.Logger) (*Application, error) {
	ctx = logger.WithContext(ctx, log)

	db, err := postgres.NewPool(ctx, cfg.Database.URL, cfg.Database.MaxConnections)
	if err != nil {
		return nil, fmt.Errorf("initialize postgres: %w", err)
	}

	tokenManager := auth.NewJWTManager(
		cfg.JWT.Secret,
		cfg.JWT.Issuer,
		cfg.JWT.Expiration,
	)
	passwordManager := auth.NewBcryptPasswordManager()
	userStore := userstore.NewPostgresStore(db)
	productStore := productstore.NewPostgresStore(db)
	cartStore := cartstore.NewPostgresStore(db)
	orderStore := orderstore.NewPostgresStore(db)
	campaignStore := campaignstore.NewPostgresStore(db)
	authService := authservice.New(userStore, tokenManager, passwordManager)
	productService := productservice.New(productStore)
	cartService := cartservice.New(cartStore, productStore)
	orderService := orderservice.New(db, orderStore)
	campaignService := campaignservice.New(campaignStore)
	authAPI := apis.NewAuthAPI(authService)
	productAPI := apis.NewProductAPI(productService)
	cartAPI := apis.NewCartAPI(cartService)
	orderAPI := apis.NewOrderAPI(orderService)
	campaignAPI := apis.NewCampaignAPI(campaignService)

	router := gin.New()
	router.Use(middlewares.RequestLogger(log), middlewares.Recovery())
	apis.RegisterHealthRoute(router)
	authMiddleware := middlewares.Authentication(tokenManager)
	authAPI.RegisterRoutes(router, authMiddleware)
	productAPI.RegisterRoutes(router, authMiddleware, middlewares.RequireRole("admin"))
	cartAPI.RegisterRoutes(router, authMiddleware)
	orderAPI.RegisterRoutes(router, authMiddleware)
	campaignAPI.RegisterRoutes(router, middlewares.OptionalAuthentication(tokenManager), authMiddleware, middlewares.RequireRole("admin"))
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return &Application{
		logger: log,
		db:     db,
		server: &http.Server{
			Addr:              cfg.HTTP.Address,
			Handler:           router,
			ReadHeaderTimeout: readHeaderTimeout,
		},
	}, nil
}

func (a *Application) Run(ctx context.Context) error {
	serverErrors := make(chan error, 1)
	go func() {
		a.logger.Info().
			Str("address", a.server.Addr).
			Msg("starting HTTP server")
		serverErrors <- a.server.ListenAndServe()
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)

	select {
	case signal := <-signals:
		a.logger.Info().
			Str("signal", signal.String()).
			Msg("shutdown signal received")
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
	defer cancel()
	if err := a.server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}
	return nil
}

func (a *Application) Close() {
	a.db.Close()
}
