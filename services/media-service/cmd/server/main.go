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

	"github.com/Anshul563/edvance-project/services/media-service/internal/config"
	"github.com/Anshul563/edvance-project/services/media-service/internal/engine"
	"github.com/Anshul563/edvance-project/services/media-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/media-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/media-service/internal/router"
	"github.com/Anshul563/edvance-project/services/media-service/internal/server"
	"github.com/Anshul563/edvance-project/services/media-service/internal/service"
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

	mediaService := service.NewMediaService(
		repository.NewMediaJobRepository(db),
		service.TrustingVideoAuthorization{},
		engine.NewHTTPClient(
			cfg.Engine.BaseURL,
			cfg.Engine.InternalToken,
			cfg.Engine.RequestTimeout,
		),
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
			"media service starting",
			"port",
			cfg.Port,
			"environment",
			cfg.AppEnv,
			"engine_url",
			cfg.Engine.BaseURL,
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

	slog.Info("media service stopped")
}
