package service

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
)

// StatusEvent is the pipeline progress announcement sent to
// content-service, which owns the user-facing video row. It carries
// only facts video-service already knows; content-service never trusts
// it for ownership or publication.
type StatusEvent struct {
	MediaAssetID    uuid.UUID
	Status          string
	DurationSeconds *int64
	ThumbnailURL    *string
	ErrorCode       string
	ErrorMessage    string
}

// StatusNotifier delivers pipeline progress to content-service. The
// HTTP implementation lives in internal/contentclient; tests inject a
// recorder. Delivery is best-effort by design: a notification outage
// must never fail an upload or a processing callback.
type StatusNotifier interface {
	Notify(ctx context.Context, event StatusEvent) error
}

// SetStatusNotifier attaches the best-effort progress notifier. It is
// optional so tests and tools can run the pipeline standalone.
func (s *MediaService) SetStatusNotifier(notifier StatusNotifier) {
	s.notifier = notifier
}

// notifyStatus emits one event, swallowing (and logging) failures.
func (s *MediaService) notifyStatus(
	ctx context.Context,
	event StatusEvent,
) {
	if s.notifier == nil {
		return
	}

	if err := s.notifier.Notify(ctx, event); err != nil {
		s.logger.Warn(
			"media status notification failed",
			"asset_id", event.MediaAssetID,
			"status", event.Status,
			"error", err,
		)
	}
}

// logDuration converts the engine's optional duration into the pointer
// shape the event carries. Zero is valid; negative is clamped away.
func logDuration(value *int64) *int64 {
	if value == nil || *value < 0 {
		return nil
	}

	return value
}

// slogStatus is a tiny helper for the worker, which has its own logger
// and no MediaService receiver.
func slogStatus(
	logger *slog.Logger,
	notifier StatusNotifier,
	event StatusEvent,
) {
	if notifier == nil {
		return
	}

	if err := notifier.Notify(context.Background(), event); err != nil {
		logger.Warn(
			"media status notification failed",
			"asset_id", event.MediaAssetID,
			"status", event.Status,
			"error", err,
		)
	}
}
