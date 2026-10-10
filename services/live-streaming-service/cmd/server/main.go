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

	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/config"
	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/router"
	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/server"
	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/service"
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

	streamService := service.NewLiveStreamService(nil, nil)
	srv := server.New(cfg, router.Handlers{
		Health: handler.NewHealthHandler(),
		Stream: handler.NewStreamHandler(streamService),
	})

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("live-streaming service starting", "port", cfg.Port, "provider", cfg.Provider.Kind)
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

	slog.Info("live-streaming service stopped")
}
