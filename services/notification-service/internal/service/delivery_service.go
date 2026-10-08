package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/provider"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/repository"
)

// DeliveryStore is the persistence contract for deliveries.
// *repository.DeliveryRepository satisfies it.
type DeliveryStore interface {
	ListByNotification(
		ctx context.Context,
		notificationID uuid.UUID,
	) ([]*model.Delivery, error)
	MarkSent(ctx context.Context, id uuid.UUID, providerMessageID string) error
	MarkFailed(
		ctx context.Context,
		id uuid.UUID,
		attemptCount int32,
		code string,
		message string,
		nextRetryAt *time.Time,
	) error
	ClaimDue(ctx context.Context, now time.Time) (*model.Delivery, error)
	CancelPending(
		ctx context.Context,
		notificationID uuid.UUID,
		channel model.Channel,
	) error
}

// NotificationLookup reads notification rows for retry rebuilds.
// Satisfied by the notification repository (or a narrow adapter).
type NotificationLookup interface {
	FindByID(ctx context.Context, id uuid.UUID) (*model.Notification, error)
}

// DeliveryService sends email deliveries and tracks per-channel state.
// In-app rows are storage, not sends: persisting them IS delivery, so
// they flip to sent immediately. Email goes through the provider
// synchronously in this step (dev implementation); ClaimDue + RetryDue
// expose the same path to a future background worker without changes.
type DeliveryService struct {
	deliveries    DeliveryStore
	templates     TemplateLookup
	email         provider.EmailProvider
	notifications NotificationLookup
}

func NewDeliveryService(
	deliveries DeliveryStore,
	templates TemplateLookup,
	email provider.EmailProvider,
	notifications NotificationLookup,
) (*DeliveryService, error) {
	if deliveries == nil || templates == nil || email == nil || notifications == nil {
		return nil, errors.New("delivery dependencies are required")
	}

	return &DeliveryService{
		deliveries:    deliveries,
		templates:     templates,
		email:         email,
		notifications: notifications,
	}, nil
}

// DispatchNew processes a freshly created notification's deliveries:
// in-app rows flip to sent, email rows attempt their first send. Email
// failures schedule retries (or go terminally failed); nothing here
// unwinds the stored notification.
func (s *DeliveryService) DispatchNew(
	ctx context.Context,
	notification *model.Notification,
	email *EmailContent,
) error {
	deliveries, err := s.deliveries.ListByNotification(ctx, notification.ID)
	if err != nil {
		return fmt.Errorf("list deliveries: %w", err)
	}

	for _, delivery := range deliveries {
		switch delivery.Channel {
		case model.ChannelInApp:
			if err := s.deliveries.MarkSent(ctx, delivery.ID, ""); err != nil {
				return fmt.Errorf("mark in-app sent: %w", err)
			}

		case model.ChannelEmail:
			if email == nil {
				continue
			}

			s.sendEmailAttempt(ctx, delivery, email.To, email.Subject, email.Body)

		default:
			// push/sms: no provider yet; rows stay pending for the
			// future worker rather than failing spuriously.
			continue
		}
	}

	return nil
}

// RetryDue claims one due delivery and attempts it. In-app rows found
// pending (crash between commit and marking) flip straight to sent.
// Email rows rebuild recipient + content from the notification row and
// templates, then attempt a real send.
func (s *DeliveryService) RetryDue(ctx context.Context, now time.Time) error {
	delivery, err := s.deliveries.ClaimDue(ctx, now)
	if err != nil {
		if errors.Is(err, repository.ErrDeliveryNotFound) {
			return nil
		}

		return fmt.Errorf("claim delivery: %w", err)
	}

	if delivery.Channel == model.ChannelInApp {
		return s.deliveries.MarkSent(ctx, delivery.ID, "")
	}

	if delivery.Channel != model.ChannelEmail {
		// Unreachable in v1 (creation only allows in_app/email):
		// fail terminally rather than looping forever.
		return s.deliveries.MarkFailed(
			ctx,
			delivery.ID,
			delivery.AttemptCount+1,
			"no_provider",
			"channel has no provider",
			nil,
		)
	}

	notification, err := s.notifications.FindByID(ctx, delivery.NotificationID)
	if err != nil {
		return fmt.Errorf("load notification: %w", err)
	}

	data := map[string]string{}

	if notification.Data != "" {
		_ = json.Unmarshal([]byte(notification.Data), &data)
	}

	to := data["email"]

	subject, body, err := s.emailContent(ctx, notification, data)
	if err != nil {
		return fmt.Errorf("rebuild content: %w", err)
	}

	s.sendEmailAttempt(ctx, delivery, to, subject, body)

	return nil
}

// emailContent renders subject/body for a notification, preferring the
// active template with request-data fallback.
func (s *DeliveryService) emailContent(
	ctx context.Context,
	notification *model.Notification,
	data map[string]string,
) (string, string, error) {
	template, err := s.templates.GetTemplate(ctx, notification.Type, model.ChannelEmail)
	if err != nil {
		return "", "", fmt.Errorf("load template: %w", err)
	}

	subject := notification.Title
	body := notification.Body

	if template != nil {
		if template.SubjectTemplate != nil && *template.SubjectTemplate != "" {
			subject, err = RenderTemplate(*template.SubjectTemplate, data)
			if err != nil {
				return "", "", fmt.Errorf("render subject: %w", err)
			}
		} else if template.TitleTemplate != nil && *template.TitleTemplate != "" {
			subject, err = RenderTemplate(*template.TitleTemplate, data)
			if err != nil {
				return "", "", fmt.Errorf("render title: %w", err)
			}
		}

		if template.BodyTemplate != "" {
			body, err = RenderTemplate(template.BodyTemplate, data)
			if err != nil {
				return "", "", fmt.Errorf("render body: %w", err)
			}
		}
	}

	return subject, body, nil
}

// sendEmailAttempt performs one send: address validation first (invalid
// recipients fail terminally without provider calls), then provider
// errors schedule retries until attempts run out.
func (s *DeliveryService) sendEmailAttempt(
	ctx context.Context,
	delivery *model.Delivery,
	to string,
	subject string,
	body string,
) {
	if _, err := mail.ParseAddress(to); err != nil {
		_ = s.deliveries.MarkFailed(
			ctx,
			delivery.ID,
			delivery.AttemptCount+1,
			"invalid_recipient",
			"invalid email address",
			nil,
		)

		return
	}

	response, err := s.email.Send(ctx, provider.EmailRequest{
		To:      to,
		Subject: subject,
		Body:    body,
	})
	if err != nil {
		next, ok := NextRetry(delivery.AttemptCount+1, time.Now())

		var retryAt *time.Time

		if ok {
			retryAt = &next
		}

		_ = s.deliveries.MarkFailed(
			ctx,
			delivery.ID,
			delivery.AttemptCount+1,
			"send_failed",
			"email provider error",
			retryAt,
		)

		return
	}

	_ = s.deliveries.MarkSent(ctx, delivery.ID, response.MessageID)
}
