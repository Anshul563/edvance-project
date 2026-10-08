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

	"github.com/Anshul563/edvance-project/services/video-service/internal/client"
	"github.com/Anshul563/edvance-project/services/video-service/internal/config"
	"github.com/Anshul563/edvance-project/services/video-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/video-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/video-service/internal/router"
	"github.com/Anshul563/edvance-project/services/video-service/internal/server"
	"github.com/Anshul563/edvance-project/services/video-service/internal/service"
	"github.com/Anshul563/edvance-project/services/video-service/internal/storage"
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

	objectStore, err := storage.NewS3(
		cfg.Storage,
		cfg.Upload.URLTTL,
	)
	if err != nil {
		slog.Error(
			"failed to init object storage",
			"error",
			err,
		)
		os.Exit(1)
	}

	engineClient := client.NewHTTPMediaEngineClient(
		cfg.Engine.BaseURL,
		cfg.Engine.Token,
		10*time.Second,
	)

	mediaRepo := repository.NewMediaRepository(db)
	variantRepo := repository.NewVariantRepository(db)
	jobRepo := repository.NewProcessingRepository(db)
	thumbnailRepo := repository.NewThumbnailRepository(db)
	captionRepo := repository.NewCaptionRepository(db)

	mediaService := service.NewMediaService(
		mediaRepo,
		variantRepo,
		jobRepo,
		thumbnailRepo,
		captionRepo,
		objectStore,
		cfg.Upload,
		cfg.Worker.MaxAttempts,
		logger,
	)

	worker := service.NewProcessingWorker(
		jobRepo,
		mediaRepo,
		objectStore,
		engineClient,
		cfg.Engine.CallbackURL,
		cfg.Worker.WorkerCount,
		cfg.Worker.MaxAttempts,
		cfg.Worker.RetryBaseSeconds,
		cfg.Worker.PollInterval,
		logger,
	)

	srv := server.New(
		cfg,
		router.Handlers{
			Health: handler.NewHealthHandler(db),
			Media:  handler.NewMediaHandler(mediaService),
		},
	)

	serverErr := make(chan error, 1)

	go func() {
		slog.Info(
			"video service starting",
			"port",
			cfg.Port,
			"environment",
			cfg.AppEnv,
			"workers",
			cfg.Worker.WorkerCount,
		)

		serverErr <- srv.Start()
	}()

	workerCtx, workerCancel := context.WithCancel(context.Background())
	workerErr := make(chan error, 1)

	go func() {
		slog.Info("processing worker starting")

		worker.Run(workerCtx)

		// Run returns only after the context is cancelled, so this
		// channel exists to say "worker drained".
		workerErr <- nil
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
	}

	workerCancel()

	select {
	case <-workerErr:
	case <-time.After(10 * time.Second):
		slog.Warn("processing worker did not stop in time")
	}

	slog.Info("video service stopped")
}
