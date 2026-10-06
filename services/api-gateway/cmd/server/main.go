package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Anshul563/edvance-project/services/api-gateway/internal/config"
	"github.com/Anshul563/edvance-project/services/api-gateway/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()
	srv := server.New(cfg)

	serverErr := make(chan error, 1)

	go func() {
		slog.Info("API Gateway starting",
			"port", cfg.Port,
			"environment", cfg.AppEnv,
		)

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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("API Gateway stopped")
}
