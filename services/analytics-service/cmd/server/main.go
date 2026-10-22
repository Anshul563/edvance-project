package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/Anshul563/edvance-project/packages/events"
	"github.com/Anshul563/edvance-project/services/analytics-service/internal/config"
	"github.com/Anshul563/edvance-project/services/analytics-service/internal/handlers"
	"github.com/Anshul563/edvance-project/services/analytics-service/internal/models"
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

	// NATS + JetStream: durable consumer that ingests business events
	// into analytics. Idempotent by event id (repo.Save uses ON CONFLICT
	// DO NOTHING), so redeliveries never double-count. A connection
	// failure is not fatal: the HTTP ingestion API keeps working.
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
				ingestEvent := func(ctx context.Context, evt *events.Event) error {
					properties := map[string]any{}
					if err := json.Unmarshal(evt.Data, &properties); err != nil {
						properties = map[string]any{}
					}

					entityType, _ := properties["entity_type"].(string)
					entityID, _ := properties["entity_id"].(string)

					analyticsEvent := models.Event{
						EventID:      evt.ID,
						EventType:    evt.Type,
						EventVersion: evt.Version,
						OccurredAt:   evt.OccurredAt,
						ActorID:      strOrEmpty(properties, "actor_id"),
						AnonymousID:  strOrEmpty(properties, "anonymous_id"),
						SessionID:    strOrEmpty(properties, "session_id"),
						Source:       evt.Source,
						EntityType:   entityType,
						EntityID:     entityID,
						Properties:   properties,
					}

					if err := analyticsService.IngestEvent(ctx, analyticsEvent); err != nil {
						return err
					}

					logger.Debug(
						"analytics event ingested",
						"event_id", evt.ID,
						"type", evt.Type,
					)
					return nil
				}

				consumer := events.NewJetStreamConsumer(
					js,
					"edvance.>",
					"analytics-service",
					cfg.NATS.AckTimeout,
					cfg.NATS.MaxDeliveries,
					logger,
				)

				natsCtx, cancel := context.WithCancel(context.Background())
				natsCancel = cancel

				go func() {
					if err := consumer.Consume(natsCtx, ingestEvent); err != nil {
						logger.Error(
							"nats consumer stopped",
							"error", err,
						)
					}
				}()

				logger.Info(
					"analytics service consuming nats events",
					"stream", cfg.NATS.Stream,
					"subject", "edvance.>",
					"durable", "analytics-service",
				)
			}
		}
	}

	if natsErr != nil {
		logger.Warn(
			"NATS unavailable; event ingestion from the bus is disabled",
			"error", natsErr,
		)
	}

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

	if natsCancel != nil {
		natsCancel()
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}

// strOrEmpty returns a map value as a string, or "" when absent or not a
// string. Keeps analytics ingestion robust to loosely typed payloads.
func strOrEmpty(properties map[string]any, key string) string {
	if value, ok := properties[key]; ok {
		if str, isStr := value.(string); isStr {
			return str
		}
	}
	return ""
}
