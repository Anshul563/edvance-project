package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/Anshul563/edvance-project/services/admin-api/internal/config"
	"github.com/Anshul563/edvance-project/services/admin-api/internal/router"
	"github.com/Anshul563/edvance-project/services/admin-api/internal/server"
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

	h := router.New(cfg)
	srv := server.New(fmt.Sprintf(":%d", cfg.Port), h)

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("admin api starting", "port", cfg.Port)
		serverErr <- srv.Start()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) && err != nil {
			slog.Error("server stopped", "error", err)
			os.Exit(1)
		}
	case signal := <-sig:
		slog.Info("shutdown signal received", "signal", signal)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}
