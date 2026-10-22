package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"

	"github.com/Anshul563/edvance-project/packages/events"
	"github.com/Anshul563/edvance-project/services/search-service/internal/config"
	"github.com/Anshul563/edvance-project/services/search-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/search-service/internal/model"
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

	// NATS + JetStream: durable consumer that keeps the search index in
	// sync with content lifecycle events. Upserts are idempotent
	// (same source_type+source_id updates the existing row), and
	// deletes remove unpublished/deleted content. Only public
	// documents are indexed; the index is a denormalized copy, never
	// the authority. A connection failure is not fatal: public search
	// keeps working over the existing index.
	natsConn, natsErr := events.Connect(cfg.NATS, logger)

	var natsCancel context.CancelFunc

	if natsErr == nil {
		defer natsConn.Close()

		if js := natsConn.JetStream(); js != nil {
			if _, serr := events.EnsureStream(
				js,
				events.DefaultEventStream(cfg.NATS),
			); serr != nil {
				logger.Warn(
					"could not ensure jetstream stream",
					"error", serr,
				)
			} else {
				indexEvent := func(ctx context.Context, evt *events.Event) error {
					var data map[string]string
					if err := json.Unmarshal(evt.Data, &data); err != nil {
						data = map[string]string{}
					}

					switch evt.Type {
					case events.CoursePublished, events.CourseUpdated, events.VideoPublished:
						return indexUpsert(ctx, searchRepo, evt, data)
					case events.CourseDeleted, events.VideoUnpublished, events.VideoDeleted:
						return indexDelete(ctx, searchRepo, evt, data)
					default:
						// Unrelated event family: acknowledge and skip.
						return nil
					}
				}

				consumer := events.NewJetStreamConsumer(
					js,
					"edvance.>",
					"search-service",
					cfg.NATS.AckTimeout,
					cfg.NATS.MaxDeliveries,
					logger,
				)

				natsCtx, cancel := context.WithCancel(context.Background())
				natsCancel = cancel

				go func() {
					if err := consumer.Consume(natsCtx, indexEvent); err != nil {
						logger.Error(
							"nats consumer stopped",
							"error", err,
						)
					}
				}()

				logger.Info(
					"search service consuming nats events",
					"stream", cfg.NATS.Stream,
					"subject", "edvance.>",
					"durable", "search-service",
				)
			}
		}
	}

	if natsErr != nil {
		logger.Warn(
			"NATS unavailable; search index sync from the bus is disabled",
			"error", natsErr,
		)
	}

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

	if natsCancel != nil {
		natsCancel()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	slog.Info("search service stopped")
}

// indexUpsert indexes one public document. Only public content is
// eligible for discovery; anything else is skipped (the index mirrors
// visibility, it does not override it).
func indexUpsert(
	ctx context.Context,
	repo repository.SearchRepository,
	evt *events.Event,
	data map[string]string,
) error {
	sourceType := data["contentType"]
	sourceIDStr := data["contentId"]

	if sourceType == "" || sourceIDStr == "" {
		return nil
	}

	if !model.ValidSourceType(sourceType) {
		return nil
	}

	sourceID, err := uuid.Parse(sourceIDStr)
	if err != nil {
		return err
	}

	if data["visibility"] != model.VisibilityPublic {
		return nil
	}

	doc := &model.SearchDocument{
		SourceType:  sourceType,
		SourceID:    sourceID,
		Title:       data["title"],
		Description: data["description"],
		Visibility:  model.VisibilityPublic,
		Language:    data["language"],
		Category:    data["category"],
		Tags:        map[string]any{},
		Metadata:    map[string]any{},
		IndexedAt:   time.Now().UTC(),
		CreatedAt:   evt.OccurredAt,
		UpdatedAt:   evt.OccurredAt,
	}

	if evt.OccurredAt.IsZero() {
		doc.PublishedAt = nil
	} else {
		doc.PublishedAt = &evt.OccurredAt
	}

	return repo.Upsert(ctx, doc)
}

// indexDelete removes a document that is no longer eligible for
// discovery (deleted or unpublished).
func indexDelete(
	ctx context.Context,
	repo repository.SearchRepository,
	evt *events.Event,
	data map[string]string,
) error {
	sourceType := data["contentType"]
	sourceIDStr := data["contentId"]

	if sourceType == "" || sourceIDStr == "" {
		return nil
	}

	if !model.ValidSourceType(sourceType) {
		return nil
	}

	sourceID, err := uuid.Parse(sourceIDStr)
	if err != nil {
		return err
	}

	if err := repo.Delete(ctx, sourceType, sourceID); err != nil {
		// A missing document is not an error: the index may already
		// reflect the deletion.
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return err
	}

	return nil
}
