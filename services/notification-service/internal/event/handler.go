package event

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/service"
)

// ErrUnknownEvent means no default mapping exists for the type. Trusted
// callers get a 400, not a silent drop: unknown types indicate a sender
// bug or version skew worth surfacing.
var ErrUnknownEvent = errors.New("unknown event type")

// Creator is the notification creation contract. It keeps this package
// independent of the concrete service for tests and the future NATS
// consumer, which will call Handle with the same struct.
type Creator interface {
	Create(
		ctx context.Context,
		request service.CreateNotificationRequest,
	) (*service.CreatedNotification, error)
}

// Defaults maps event types to their out-of-the-box content, channels,
// and priority. DB templates override title/subject/body at send time;
// these defaults guarantee every known event renders even with an empty
// template table. Per-domain mappings live in auth.go, course.go,
// learning.go, payment.go, and video.go; this aggregator only merges
// them so Handle stays transport logic.
type Defaults struct {
	Title    string
	Body     string
	Channels []model.Channel
	Priority model.Priority
}

func inAppEmail() []model.Channel {
	return []model.Channel{model.ChannelInApp, model.ChannelEmail}
}

func inAppOnly() []model.Channel {
	return []model.Channel{model.ChannelInApp}
}

func defaultsFor(notificationType string) (Defaults, bool) {
	for _, table := range []map[string]Defaults{
		authDefaults(),
		courseDefaults(),
		learningDefaults(),
		paymentDefaults(),
		videoDefaults(),
	} {
		if def, ok := table[notificationType]; ok {
			return def, true
		}
	}

	return Defaults{}, false
}

// Handler maps business events to notifications. Transport-independent:
// HTTP calls Handle today, NATS will call it tomorrow with the same
// Event struct.
type Handler struct {
	creator Creator
}

func NewHandler(creator Creator) *Handler {
	return &Handler{
		creator: creator,
	}
}

// Handle renders defaults with event data and creates the notification
// (idempotent by event id).
func (h *Handler) Handle(
	ctx context.Context,
	event Event,
) (*service.CreatedNotification, error) {
	if event.UserID == uuid.Nil {
		return nil, errors.New("event user id is required")
	}

	def, ok := defaultsFor(event.Type)
	if !ok {
		return nil, ErrUnknownEvent
	}

	data := event.Data
	if data == nil {
		data = map[string]string{}
	}

	title, err := service.RenderTemplate(def.Title, data)
	if err != nil {
		return nil, err
	}

	body, err := service.RenderTemplate(def.Body, data)
	if err != nil {
		return nil, err
	}

	return h.creator.Create(ctx, service.CreateNotificationRequest{
		UserID:   event.UserID,
		Type:     event.Type,
		Title:    title,
		Body:     body,
		Data:     data,
		Priority: def.Priority,
		EventID:  event.EventID,
		Channels: def.Channels,
		Email:    data["email"],
	})
}
