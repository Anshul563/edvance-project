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

	"github.com/Anshul563/edvance-project/services/creator-service/internal/config"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/router"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/server"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/service"
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

	creatorService := service.NewCreatorService(
		repository.NewCreatorRepository(db),
		repository.NewChannelRepository(db),
	)

	srv := server.New(
		cfg,
		router.Handlers{
			Health:  handler.NewHealthHandler(db),
			Creator: handler.NewCreatorHandler(creatorService),
		},
	)

	serverErr := make(chan error, 1)

	go func() {
		slog.Info(
			"creator service starting",
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

	slog.Info("creator service stopped")
}
