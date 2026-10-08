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

	"github.com/Anshul563/edvance-project/services/notification-service/internal/config"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/event"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/provider"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/router"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/server"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/service"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/worker"
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

	emailProvider, err := newEmailProvider(cfg)
	if err != nil {
		slog.Error(
			"failed to create email provider",
			"error",
			err,
		)
		os.Exit(1)
	}

	notificationRepository := repository.NewNotificationRepository(db)
	deliveryRepository := repository.NewDeliveryRepository(db)
	preferenceRepository := repository.NewPreferenceRepository(db)
	templateRepository := repository.NewTemplateRepository(db)

	deliveryService, err := service.NewDeliveryService(
		deliveryRepository,
		templateRepository,
		emailProvider,
		notificationRepository,
		cfg.Worker.MaxAttempts,
	)
	if err != nil {
		slog.Error(
			"failed to create delivery service",
			"error",
			err,
		)
		os.Exit(1)
	}

	notificationService, err := service.NewNotificationService(
		notificationRepository,
		preferenceRepository,
		templateRepository,
		deliveryService,
	)
	if err != nil {
		slog.Error(
			"failed to create notification service",
			"error",
			err,
		)
		os.Exit(1)
	}

	preferenceService, err := service.NewPreferenceService(
		preferenceRepository,
	)
	if err != nil {
		slog.Error(
			"failed to create preference service",
			"error",
			err,
		)
		os.Exit(1)
	}

	srv := server.New(
		cfg,
		router.Handlers{
			Health:       handler.NewHealthHandler(db),
			Notification: handler.NewNotificationHandler(notificationService),
			Preference:   handler.NewPreferenceHandler(preferenceService),
			Event:        handler.NewEventHandler(event.NewHandler(notificationService)),
		},
	)

	serverErr := make(chan error, 1)

	// Delivery retry worker: polls due email deliveries on an interval.
	// It stops via workerCancel before HTTP shutdown completes so no
	// send outlives the process.
	workerCtx, workerCancel := context.WithCancel(context.Background())
	workerDone := make(chan struct{})

	go func() {
		defer close(workerDone)

		worker.Run(
			workerCtx,
			deliveryService,
			cfg.Worker.Interval,
			cfg.Worker.Count,
		)
	}()

	go func() {
		slog.Info(
			"notification service starting",
			"port",
			cfg.Port,
			"environment",
			cfg.AppEnv,
			"email_provider",
			cfg.Email.Provider,
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

	workerCancel()

	select {
	case <-workerDone:
	case <-shutdownCtx.Done():
		slog.Error("worker shutdown timed out")
	}

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error(
			"graceful shutdown failed",
			"error",
			err,
		)
		os.Exit(1)
	}

	slog.Info("notification service stopped")
}

// newEmailProvider builds the configured email provider. Config
// validation already guarantees console mode never runs in production.
func newEmailProvider(cfg config.Config) (provider.EmailProvider, error) {
	switch cfg.Email.Provider {
	case "smtp":
		return provider.NewSMTPProvider(provider.SMTPConfig{
			Host:     cfg.Email.SMTPHost,
			Port:     cfg.Email.SMTPPort,
			Username: cfg.Email.SMTPUsername,
			Password: cfg.Email.SMTPPassword,
			From:     cfg.Email.SMTPFrom,
		})

	default:
		return provider.NewConsoleProvider(), nil
	}
}
