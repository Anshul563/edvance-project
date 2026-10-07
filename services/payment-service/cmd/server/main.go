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

	"github.com/Anshul563/edvance-project/services/payment-service/internal/commerce"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/config"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/razorpay"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/router"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/server"
	"github.com/Anshul563/edvance-project/services/payment-service/internal/service"
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

	razorpayClient := razorpay.NewHTTPClient(
		cfg.Razorpay.BaseURL,
		cfg.Razorpay.KeyID,
		cfg.Razorpay.KeySecret,
		cfg.Razorpay.RequestTimeout,
	)

	commerceClient := commerce.NewHTTPClient(
		cfg.Commerce.BaseURL,
		cfg.Commerce.InternalToken,
		cfg.Commerce.RequestTimeout,
	)

	paymentRepository := repository.NewPaymentRepository(db)
	webhookRepository := repository.NewWebhookRepository(db)
	refundRepository := repository.NewRefundRepository(db)

	paymentService, err := service.NewPaymentService(
		paymentRepository,
		commerceClient,
		razorpayClient,
		cfg.Razorpay.KeyID,
		cfg.Razorpay.KeySecret,
	)
	if err != nil {
		slog.Error(
			"failed to create payment service",
			"error",
			err,
		)
		os.Exit(1)
	}

	refundService, err := service.NewRefundService(
		refundRepository,
		paymentRepository,
		razorpayClient,
	)
	if err != nil {
		slog.Error(
			"failed to create refund service",
			"error",
			err,
		)
		os.Exit(1)
	}

	webhookService, err := service.NewWebhookService(
		webhookRepository,
		paymentRepository,
		refundService,
		commerceClient,
		cfg.Razorpay.WebhookSecret,
	)
	if err != nil {
		slog.Error(
			"failed to create webhook service",
			"error",
			err,
		)
		os.Exit(1)
	}

	srv := server.New(
		cfg,
		router.Handlers{
			Health:  handler.NewHealthHandler(db),
			Payment: handler.NewPaymentHandler(paymentService),
			Webhook: handler.NewWebhookHandler(webhookService),
			Refund:  handler.NewRefundHandler(refundService),
		},
	)

	serverErr := make(chan error, 1)

	go func() {
		slog.Info(
			"payment service starting",
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

	slog.Info("payment service stopped")
}
