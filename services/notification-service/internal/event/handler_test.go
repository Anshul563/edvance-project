package event

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/service"
)

type stubCreator struct {
	request service.CreateNotificationRequest
	created *service.CreatedNotification
	err     error
}

func (s *stubCreator) Create(
	_ context.Context,
	request service.CreateNotificationRequest,
) (*service.CreatedNotification, error) {
	s.request = request

	if s.err != nil {
		return nil, s.err
	}

	return s.created, nil
}

func testCreated() *service.CreatedNotification {
	return &service.CreatedNotification{
		Notification: &model.Notification{ID: uuid.New()},
	}
}

func TestHandleKnownEvent(t *testing.T) {
	stub := &stubCreator{created: testCreated()}
	h := NewHandler(stub)
	userID := uuid.New()

	created, err := h.Handle(context.Background(), Event{
		EventID: "pay-event-123",
		Type:    "payment.captured",
		UserID:  userID,
		Data:    map[string]string{"email": "user@example.com", "amount": "₹999", "courseTitle": "Go"},
	})
	if err != nil {
		t.Fatalf("handle: %v", err)
	}

	_ = created

	if stub.request.UserID != userID {
		t.Fatal("user must come from the event")
	}

	if stub.request.Type != "payment.captured" {
		t.Fatal("type must pass through")
	}

	if stub.request.EventID != "pay-event-123" {
		t.Fatal("event id must pass through for idempotency")
	}

	if len(stub.request.Channels) != 2 {
		t.Fatalf("expected default channels, got %v", stub.request.Channels)
	}

	if stub.request.Email != "user@example.com" {
		t.Fatal("email must come from event data")
	}

	if stub.request.Priority != model.PriorityNormal {
		t.Fatalf("unexpected priority %s", stub.request.Priority)
	}
}

func TestHandleSecurityPriority(t *testing.T) {
	stub := &stubCreator{created: testCreated()}
	h := NewHandler(stub)

	if _, err := h.Handle(context.Background(), Event{
		EventID: "sec-1",
		Type:    "auth.security_alert",
		UserID:  uuid.New(),
		Data:    map[string]string{"email": "a@b.c"},
	}); err != nil {
		t.Fatalf("handle: %v", err)
	}

	if stub.request.Priority != model.PriorityCritical {
		t.Fatalf("expected critical, got %s", stub.request.Priority)
	}
}

func TestHandleUnknownEvent(t *testing.T) {
	stub := &stubCreator{created: testCreated()}
	h := NewHandler(stub)

	if _, err := h.Handle(context.Background(), Event{
		EventID: "x-1",
		Type:    "bogus.event",
		UserID:  uuid.New(),
	}); !errors.Is(err, ErrUnknownEvent) {
		t.Fatalf("expected unknown event, got %v", err)
	}
}

func TestHandleMissingUser(t *testing.T) {
	stub := &stubCreator{created: testCreated()}
	h := NewHandler(stub)

	if _, err := h.Handle(context.Background(), Event{
		EventID: "x-2",
		Type:    "payment.captured",
	}); err == nil {
		t.Fatal("expected user-required error")
	}
}
