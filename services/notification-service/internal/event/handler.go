package event

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/ntype"
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
// template table.
type Defaults struct {
	Title    string
	Body     string
	Channels []model.Channel
	Priority model.Priority
}

func defaultsFor(notificationType string) (Defaults, bool) {
	inAppEmail := []model.Channel{model.ChannelInApp, model.ChannelEmail}
	inApp := []model.Channel{model.ChannelInApp}

	defaults := map[string]Defaults{
		ntype.UserEmailVerified: {"Email verified", "Your email address is verified.", inAppEmail, model.PriorityNormal},
		ntype.AuthPasswordReset: {"Password reset", "Use the link we emailed you to choose a new password.", inAppEmail, model.PriorityHigh},
		ntype.AuthSecurityAlert: {"Security alert", "We noticed unusual activity on your account.", inAppEmail, model.PriorityCritical},

		ntype.CoursePublished: {"Course published", "Your course is now live.", inAppEmail, model.PriorityNormal},
		ntype.CourseUpdated:   {"Course updated", "A course you follow has new content.", inApp, model.PriorityLow},

		ntype.LearningEnrolled:        {"You're enrolled!", "Your learning journey starts now.", inAppEmail, model.PriorityNormal},
		ntype.LearningLessonCompleted: {"Lesson complete", "Nice progress — keep going.", inApp, model.PriorityLow},
		ntype.LearningCourseCompleted: {"Course completed!", "You finished the course. Congratulations!", inAppEmail, model.PriorityHigh},

		ntype.PaymentCreated:  {"Payment started", "Complete your payment to get access.", inApp, model.PriorityNormal},
		ntype.PaymentCaptured: {"Payment successful", "Your payment was completed successfully.", inAppEmail, model.PriorityNormal},
		ntype.PaymentFailed:   {"Payment failed", "Your payment could not be completed.", inAppEmail, model.PriorityHigh},
		ntype.PaymentRefunded: {"Refund issued", "A refund was issued to your account.", inAppEmail, model.PriorityNormal},

		ntype.OrderPaid:   {"Order confirmed", "Your order is confirmed.", inAppEmail, model.PriorityNormal},
		ntype.OrderFailed: {"Order failed", "Your order could not be completed.", inAppEmail, model.PriorityHigh},

		ntype.VideoProcessingCompleted: {"Video ready", "Your video finished processing.", inApp, model.PriorityNormal},
		ntype.VideoProcessingFailed:    {"Video processing failed", "Your video could not be processed.", inApp, model.PriorityHigh},
	}

	def, ok := defaults[notificationType]

	return def, ok
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
