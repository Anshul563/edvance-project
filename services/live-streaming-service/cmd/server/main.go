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
	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/mediamtx"
	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/repository"
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

	startupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := repository.NewPostgres(startupCtx, cfg.Database.URL)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	mediaClient := mediamtx.NewClient(cfg.Media.APIURL, cfg.Media.Timeout)

	streamService := service.NewLiveStreamService(
		repository.NewStreamRepository(db),
		mediaClient,
		service.PublicAddresses{
			RTMPURL:    cfg.Public.RTMPURL,
			HLSBaseURL: cfg.Public.HLSBaseURL,
			PathPrefix: cfg.Public.StreamKeyID,
		},
		logger,
	)

	// Background reconciliation covers missed media-server hook events.
	reconcilerCtx, reconcilerCancel := context.WithCancel(context.Background())
	defer reconcilerCancel()
	go streamService.StartReconciler(reconcilerCtx, 15*time.Second)

	srv := server.New(cfg, router.Handlers{
		Health: handler.NewHealthHandler(db),
		Stream: handler.NewStreamHandler(streamService),
		Hook:   handler.NewHookHandler(streamService),
	})

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("live-streaming service starting",
			"port", cfg.Port,
			"mediamtx_api", cfg.Media.APIURL,
			"rtmp_public_url", cfg.Public.RTMPURL,
			"hls_public_base_url", cfg.Public.HLSBaseURL,
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
	case received := <-sig:
		slog.Info("shutdown signal received", "signal", received)
	}

	reconcilerCancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("live-streaming service stopped")
}
