package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/Anshul563/edvance-project/services/search-service/internal/config"
	"github.com/Anshul563/edvance-project/services/search-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/search-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/search-service/internal/router"
	"github.com/Anshul563/edvance-project/services/search-service/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Development convenience: load .env from the service
	// directory. Real environment variables always take
	// precedence, so this is a no-op in production.
	if err := godotenv.Load(); err != nil {
		slog.Debug("no .env file loaded, using process environment")
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer startupCancel()

	db, err := repository.NewPostgres(startupCtx, cfg.Database.URL)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	searchRepo := repository.NewSearchRepository(db)
	searchHandler := handler.NewSearchHandler(searchRepo, cfg)

	srv := server.New(
		cfg,
		router.Handlers{
			Search: searchHandler,
		},
	)

	serverErr := make(chan error, 1)

	go func() {
		slog.Info(
			"search service starting",
			"port", cfg.Port,
			"environment", cfg.AppEnv,
		)
		serverErr <- srv.Start()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server stopped", "error", err)
			os.Exit(1)
		}
	case received := <-sig:
		slog.Info("shutdown signal received", "signal", received)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("search service stopped")
}
