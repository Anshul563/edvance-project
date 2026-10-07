package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/Anshul563/edvance-project/services/commerce-service/internal/config"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/course"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/learning"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/router"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/server"
	"github.com/Anshul563/edvance-project/services/commerce-service/internal/service"
)

func main() {
	logger := slog.New(
		slog.NewJSONHandler(os.Stdout, nil),
	)

	slog.SetDefault(logger)

	// Development convenience: load .env from the service directory.
	// Real environment variables always take precedence, so this is a
	// no-op in production where .env does not exist.
	if err := godotenv.Load(); err != nil {
		slog.Debug(
			"no .env file loaded, using process environment",
		)
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error(
			"failed to load config",
			"error",
			err,
		)
		os.Exit(1)
	}

	startupCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	db, err := repository.NewPostgres(
		startupCtx,
		cfg.Database.URL,
	)
	if err != nil {
		slog.Error(
			"failed to connect to postgres",
			"error",
			err,
		)
		os.Exit(1)
	}
	defer db.Close()

	courseClient := course.NewHTTPClient(
		cfg.Course.BaseURL,
		cfg.Course.RequestTimeout,
	)

	cartRepository := repository.NewCartRepository(db)
	orderRepository := repository.NewOrderRepository(db)
	couponRepository := repository.NewCouponRepository(db)
	purchaseRepository := repository.NewPurchaseRepository(db)

	provisioner := learning.NewHTTPProvisioner(
		cfg.Learning.BaseURL,
		cfg.Learning.RequestTimeout,
	)

	cartService, err := service.NewCartService(
		cartRepository,
		courseClient,
		purchaseRepository,
	)
	if err != nil {
		slog.Error(
			"failed to create cart service",
			"error",
			err,
		)
		os.Exit(1)
	}

	couponService, err := service.NewCouponService(
		couponRepository,
		courseClient,
	)
	if err != nil {
		slog.Error(
			"failed to create coupon service",
			"error",
			err,
		)
		os.Exit(1)
	}

	orderService, err := service.NewOrderService(
		orderRepository,
		courseClient,
		couponService,
		couponRepository,
		purchaseRepository,
	)
	if err != nil {
		slog.Error(
			"failed to create order service",
			"error",
			err,
		)
		os.Exit(1)
	}

	purchaseService, err := service.NewPurchaseService(
		purchaseRepository,
		provisioner,
	)
	if err != nil {
		slog.Error(
			"failed to create purchase service",
			"error",
			err,
		)
		os.Exit(1)
	}

	srv := server.New(
		cfg,
		router.Handlers{
			Health:   handler.NewHealthHandler(db),
			Cart:     handler.NewCartHandler(cartService),
			Order:    handler.NewOrderHandler(orderService),
			Coupon:   handler.NewCouponHandler(couponService),
			Purchase: handler.NewPurchaseHandler(purchaseService),
		},
	)

	serverErr := make(chan error, 1)

	go func() {
		slog.Info(
			"commerce service starting",
			"port",
			cfg.Port,
			"environment",
			cfg.AppEnv,
		)

		serverErr <- srv.Start()
	}()

	sig := make(chan os.Signal, 1)

	signal.Notify(
		sig,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	select {
	case err := <-serverErr:
		if !errors.Is(err, os.ErrClosed) {
			slog.Error(
				"server stopped",
				"error",
				err,
			)
			os.Exit(1)
		}

	case signal := <-sig:
		slog.Info(
			"shutdown signal received",
			"signal",
			signal,
		)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error(
			"graceful shutdown failed",
			"error",
			err,
		)
		os.Exit(1)
	}

	slog.Info("commerce service stopped")
}
