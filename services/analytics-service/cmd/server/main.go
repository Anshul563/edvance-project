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

	"github.com/Anshul563/edvance-project/services/analytics-service/internal/config"
	"github.com/Anshul563/edvance-project/services/analytics-service/internal/handlers"
	"github.com/Anshul563/edvance-project/services/analytics-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/analytics-service/internal/router"
	"github.com/Anshul563/edvance-project/services/analytics-service/internal/server"
	"github.com/Anshul563/edvance-project/services/analytics-service/internal/services"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if err := godotenv.Load(); err != nil {
		slog.Debug("no .env file loaded, using process environment")
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := repository.NewPostgres(startupCtx, cfg.Database.URL)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	analyticsService := services.NewAnalyticsService(repository.NewEventRepository(db))
	srv := server.New(cfg, router.Handlers{
		Health:    handlers.NewHealthHandler(),
		Analytics: handlers.NewAnalyticsHandler(analyticsService, cfg.Limit.MaxBatchSize),
	})

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("analytics service starting", "port", cfg.Port)
		serverErr <- srv.Start()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if !errors.Is(err, os.ErrClosed) {
			slog.Error("server stopped", "error", err)
			os.Exit(1)
		}
	case signal := <-sig:
		slog.Info("shutdown signal received", "signal", signal)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}
