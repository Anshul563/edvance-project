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

	"github.com/Anshul563/edvance-project/services/content-service/internal/config"
	"github.com/Anshul563/edvance-project/services/content-service/internal/creatorclient"
	"github.com/Anshul563/edvance-project/services/content-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/content-service/internal/notifier"
	"github.com/Anshul563/edvance-project/services/content-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/content-service/internal/router"
	"github.com/Anshul563/edvance-project/services/content-service/internal/server"
	"github.com/Anshul563/edvance-project/services/content-service/internal/service"
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

	limits := service.Limits{
		DefaultPage:             cfg.Content.DefaultPageSize,
		MaxPage:                 cfg.Content.MaxPageSize,
		MaxShortDurationSeconds: cfg.Content.MaxShortDurationSeconds,
		MaxTitleLength:          cfg.Content.MaxTitleLength,
		MaxDescriptionLength:    cfg.Content.MaxDescriptionLength,
		MaxPostContentLength:    cfg.Content.MaxPostContentLength,
		MaxTagsPerItem:          cfg.Content.MaxTagsPerItem,
	}

	// Ownership is derived by asking creator-service about the caller's
	// own token. The content service never trusts a client-sent
	// creator_id and never reads creator-service's database.
	creators := creatorclient.New(cfg.CreatorServiceURL)

	// Publishing is best effort: a notification outage must never roll
	// back a publish. Noop is used when no URL is configured.
	var publisher notifier.Notifier = notifier.Noop()

	if cfg.Notification.URL != "" {
		publisher = notifier.NewHTTP(
			cfg.Notification.URL,
			cfg.Notification.InternalToken,
		)
	}

	videoService := service.NewVideoService(
		repository.NewVideoRepository(db),
		creators,
		publisher,
		limits,
	)

	shortService := service.NewShortService(
		repository.NewShortRepository(db),
		creators,
		publisher,
		limits,
	)

	postService := service.NewPostService(
		repository.NewPostRepository(db),
		creators,
		publisher,
		limits,
	)

	tagService := service.NewTagService(
		repository.NewTagRepository(db),
		limits,
	)

	categoryService := service.NewCategoryService(
		repository.NewCategoryRepository(db),
		limits,
	)

	srv := server.New(
		cfg,
		router.Handlers{
			Health:   handler.NewHealthHandler(db),
			Video:    handler.NewVideoHandler(videoService),
			Short:    handler.NewShortHandler(shortService),
			Post:     handler.NewPostHandler(postService),
			Tag:      handler.NewTagHandler(tagService),
			Category: handler.NewCategoryHandler(categoryService),
		},
	)

	serverErr := make(chan error, 1)

	go func() {
		slog.Info(
			"content service starting",
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
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error(
				"server stopped",
				"error",
				err,
			)
			os.Exit(1)
		}

	case received := <-sig:
		slog.Info(
			"shutdown signal received",
			"signal",
			received,
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

	slog.Info("content service stopped")
}
