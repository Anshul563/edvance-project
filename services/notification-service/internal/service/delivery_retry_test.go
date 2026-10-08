package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/ntype"
)

// recordingDeliveryStore is a scriptable DeliveryStore for retry-path
// tests: ListByNotification synthesizes one pending row per channel
// recorded at creation, and every mutation is captured for assertions.
type recordingDeliveryStore struct {
	mu       sync.Mutex
	created  [][]model.Channel
	byID     map[uuid.UUID]*model.Delivery
	sent     []uuid.UUID
	failures []failedCall
}

type failedCall struct {
	id      uuid.UUID
	attempt int32
	retryAt *time.Time
}

func newRecordingDeliveryStore() *recordingDeliveryStore {
	return &recordingDeliveryStore{
		byID: make(map[uuid.UUID]*model.Delivery),
	}
}

func (f *recordingDeliveryStore) recordCreate(channels []model.Channel) []uuid.UUID {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.created = append(f.created, append([]model.Channel{}, channels...))

	var ids []uuid.UUID

	for _, channel := range channels {
		id := uuid.New()
		f.byID[id] = &model.Delivery{
			ID:      id,
			Channel: channel,
			Status:  model.DeliveryPending,
		}
		ids = append(ids, id)
	}

	return ids
}

func (f *recordingDeliveryStore) ListByNotification(
	_ context.Context,
	_ uuid.UUID,
) ([]*model.Delivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := []*model.Delivery{}

	for _, row := range f.byID {
		cp := *row
		out = append(out, &cp)
	}

	return out, nil
}

func (f *recordingDeliveryStore) MarkSent(
	_ context.Context,
	id uuid.UUID,
	_ string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	row, ok := f.byID[id]
	if !ok {
		return errors.New("missing delivery")
	}

	row.Status = model.DeliverySent
	f.sent = append(f.sent, id)

	return nil
}

func (f *recordingDeliveryStore) MarkFailed(
	_ context.Context,
	id uuid.UUID,
	attempt int32,
	_ string,
	_ string,
	retryAt *time.Time,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	row, ok := f.byID[id]
	if !ok {
		return errors.New("missing delivery")
	}

	row.AttemptCount = attempt
	row.NextRetryAt = retryAt

	if retryAt == nil {
		row.Status = model.DeliveryFailed
	}

	f.failures = append(f.failures, failedCall{id: id, attempt: attempt, retryAt: retryAt})

	return nil
}

func (f *recordingDeliveryStore) ClaimDue(
	_ context.Context,
	now time.Time,
) (*model.Delivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, row := range f.byID {
		if row.Status != model.DeliveryPending {
			continue
		}

		if row.NextRetryAt != nil && row.NextRetryAt.After(now) {
			continue
		}

		row.Status = model.DeliveryProcessing
		cp := *row

		return &cp, nil
	}

	return nil, errors.New("nothing due")
}

func (f *recordingDeliveryStore) CancelPending(
	_ context.Context,
	_ uuid.UUID,
	_ model.Channel,
) error {
	return nil
}

func (f *fakeNotificationStore) ListByNotification(
	_ context.Context,
	_ uuid.UUID,
) ([]*model.Delivery, error) {
	return []*model.Delivery{}, nil
}

func TestPermanentFailureSkipsProvider(t *testing.T) {
	notifications := newFakeNotificationStore()
	deliveries := newRecordingDeliveryStore()
	notifications.deliveries = nil
	templates := &fakeTemplateStore{templates: make(map[string]*model.Template)}
	email := &fakeEmailProvider{}

	delivery, err := NewDeliveryService(deliveries, templates, email, notifications, 4)
	if err != nil {
		t.Fatalf("delivery service: %v", err)
	}

	notification := &model.Notification{
		ID:     uuid.New(),
		UserID: uuid.New(),
		Type:   ntype.PaymentCaptured,
		Title:  "Paid",
		Body:   "Done.",
	}

	ids := deliveries.recordCreate([]model.Channel{model.ChannelEmail})

	if err := delivery.DispatchNew(context.Background(), notification, &EmailContent{
		To:      "not-an-email",
		Subject: "Paid",
		Body:    "Done.",
	}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	// Invalid recipient: terminal failure, provider never called.
	if len(email.sent) != 0 {
		t.Fatal("provider must not be called for invalid addresses")
	}

	if len(deliveries.failures) != 1 {
		t.Fatalf("expected one failure record, got %d", len(deliveries.failures))
	}

	failure := deliveries.failures[0]

	if failure.id != ids[0] {
		t.Fatal("failure must attach to the email row")
	}

	if failure.retryAt != nil {
		t.Fatal("permanent failures schedule no retry")
	}

	if failure.attempt != 1 {
		t.Fatalf("expected attempt 1, got %d", failure.attempt)
	}
}

func TestTransientFailureSchedulesRetry(t *testing.T) {
	notifications := newFakeNotificationStore()
	deliveries := newRecordingDeliveryStore()
	templates := &fakeTemplateStore{templates: make(map[string]*model.Template)}
	email := &fakeEmailProvider{err: errors.New("smtp down")}

	delivery, err := NewDeliveryService(deliveries, templates, email, notifications, 4)
	if err != nil {
		t.Fatalf("delivery service: %v", err)
	}

	notification := &model.Notification{
		ID:     uuid.New(),
		UserID: uuid.New(),
		Type:   ntype.PaymentCaptured,
		Title:  "Paid",
		Body:   "Done.",
	}

	ids := deliveries.recordCreate([]model.Channel{model.ChannelEmail})

	before := time.Now()

	if err := delivery.DispatchNew(context.Background(), notification, &EmailContent{
		To:      "user@example.com",
		Subject: "Paid",
		Body:    "Done.",
	}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if len(deliveries.failures) != 1 {
		t.Fatalf("expected one failure record, got %d", len(deliveries.failures))
	}

	failure := deliveries.failures[0]

	if failure.id != ids[0] || failure.attempt != 1 {
		t.Fatalf("unexpected failure record: %+v", failure)
	}

	if failure.retryAt == nil {
		t.Fatal("transient failures must schedule a retry")
	}

	if failure.retryAt.Sub(before) < time.Minute || failure.retryAt.Sub(before) > 2*time.Minute {
		t.Fatalf("expected ~1m retry, got %v", failure.retryAt.Sub(before))
	}
}

func TestAttemptsExhaustGoesTerminal(t *testing.T) {
	notifications := newFakeNotificationStore()
	deliveries := newRecordingDeliveryStore()
	templates := &fakeTemplateStore{templates: make(map[string]*model.Template)}
	email := &fakeEmailProvider{err: errors.New("smtp down")}

	delivery, err := NewDeliveryService(deliveries, templates, email, notifications, 2)
	if err != nil {
		t.Fatalf("delivery service: %v", err)
	}

	notification := &model.Notification{
		ID:     uuid.New(),
		UserID: uuid.New(),
		Type:   ntype.PaymentCaptured,
		Title:  "Paid",
		Body:   "Done.",
	}

	ids := deliveries.recordCreate([]model.Channel{model.ChannelEmail})

	// Simulate a row already at attempt 2 of max 2: next failure is terminal.
	deliveries.mu.Lock()
	deliveries.byID[ids[0]].AttemptCount = 2
	deliveries.mu.Unlock()

	if err := delivery.DispatchNew(context.Background(), notification, &EmailContent{
		To:      "user@example.com",
		Subject: "Paid",
		Body:    "Done.",
	}); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if len(deliveries.failures) != 1 {
		t.Fatalf("expected one failure record, got %d", len(deliveries.failures))
	}

	if deliveries.failures[0].retryAt != nil {
		t.Fatal("exhausted attempts must not schedule")
	}
}

func TestRetryDueRecoversInApp(t *testing.T) {
	notifications := newFakeNotificationStore()
	deliveries := newRecordingDeliveryStore()
	templates := &fakeTemplateStore{templates: make(map[string]*model.Template)}
	email := &fakeEmailProvider{}

	delivery, err := NewDeliveryService(deliveries, templates, email, notifications, 4)
	if err != nil {
		t.Fatalf("delivery service: %v", err)
	}

	ids := deliveries.recordCreate([]model.Channel{model.ChannelInApp})

	if err := delivery.RetryDue(context.Background(), time.Now()); err != nil {
		t.Fatalf("retry: %v", err)
	}

	if len(deliveries.sent) != 1 || deliveries.sent[0] != ids[0] {
		t.Fatal("pending in-app rows must flip to sent")
	}
}
