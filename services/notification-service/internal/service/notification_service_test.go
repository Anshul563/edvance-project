package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/ntype"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/provider"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/repository"
)

// fakeNotificationStore is an in-memory NotificationStore. It mirrors
// created channel sets into the delivery fake so dispatch assertions
// observe what the transaction persisted.
type fakeNotificationStore struct {
	mu         sync.Mutex
	byID       map[uuid.UUID]*model.Notification
	byEvent    map[string]*model.Notification
	deliveries *fakeDeliveryStore
}

func newFakeNotificationStore() *fakeNotificationStore {
	return &fakeNotificationStore{
		byID:    make(map[uuid.UUID]*model.Notification),
		byEvent: make(map[string]*model.Notification),
	}
}

func (f *fakeNotificationStore) CreateWithDeliveries(
	_ context.Context,
	notification *model.Notification,
	channels []model.Channel,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if notification.EventID != nil {
		if _, exists := f.byEvent[*notification.EventID]; exists {
			return repository.ErrDuplicateEvent
		}
	}

	notification.ID = uuid.New()
	notification.CreatedAt = time.Now()

	stored := *notification
	f.byID[notification.ID] = &stored

	if notification.EventID != nil {
		f.byEvent[*notification.EventID] = &stored
	}

	if f.deliveries != nil {
		f.deliveries.mu.Lock()
		f.deliveries.created = append(
			f.deliveries.created,
			append([]model.Channel{}, channels...),
		)
		f.deliveries.mu.Unlock()
	}

	return nil
}

func (f *fakeNotificationStore) FindByID(
	_ context.Context,
	id uuid.UUID,
) (*model.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	notification, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrNotificationNotFound
	}

	cp := *notification

	return &cp, nil
}

func (f *fakeNotificationStore) FindByEventID(
	_ context.Context,
	eventID string,
) (*model.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	notification, ok := f.byEvent[eventID]
	if !ok {
		return nil, repository.ErrNotificationNotFound
	}

	cp := *notification

	return &cp, nil
}

func (f *fakeNotificationStore) ListByUser(
	_ context.Context,
	userID uuid.UUID,
	unreadOnly bool,
	limit int,
	offset int,
) ([]*model.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var all []*model.Notification

	for _, notification := range f.byID {
		if notification.UserID != userID {
			continue
		}

		if unreadOnly && notification.IsRead() {
			continue
		}

		cp := *notification
		all = append(all, &cp)
	}

	if offset >= len(all) {
		return []*model.Notification{}, nil
	}

	end := offset + limit
	if end > len(all) {
		end = len(all)
	}

	return all[offset:end], nil
}

func (f *fakeNotificationStore) CountByUser(
	_ context.Context,
	userID uuid.UUID,
	unreadOnly bool,
) (int64, error) {
	items, _ := f.ListByUser(context.Background(), userID, unreadOnly, 1<<30, 0)

	return int64(len(items)), nil
}

func (f *fakeNotificationStore) MarkRead(
	_ context.Context,
	id uuid.UUID,
	userID uuid.UUID,
) (*model.Notification, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	notification, ok := f.byID[id]
	if !ok || notification.UserID != userID {
		return nil, repository.ErrNotificationNotFound
	}

	now := time.Now()
	notification.ReadAt = &now

	cp := *notification

	return &cp, nil
}

func (f *fakeNotificationStore) MarkAllRead(
	_ context.Context,
	userID uuid.UUID,
) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var count int64

	for _, notification := range f.byID {
		if notification.UserID == userID && !notification.IsRead() {
			now := time.Now()
			notification.ReadAt = &now
			count++
		}
	}

	return count, nil
}

func (f *fakeNotificationStore) Delete(
	_ context.Context,
	id uuid.UUID,
	userID uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	notification, ok := f.byID[id]
	if !ok || notification.UserID != userID {
		return repository.ErrNotificationNotFound
	}

	delete(f.byID, id)

	if notification.EventID != nil {
		delete(f.byEvent, *notification.EventID)
	}

	return nil
}

// fakePreferenceStore serves one programmable row (nil = never customized).
type fakePreferenceStore struct {
	mu  sync.Mutex
	row *model.Preference
}

func (f *fakePreferenceStore) GetPreferences(
	_ context.Context,
	_ uuid.UUID,
) (*model.Preference, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.row == nil {
		return nil, nil
	}

	cp := *f.row

	return &cp, nil
}

func (f *fakePreferenceStore) UpsertPreferences(
	_ context.Context,
	preference *model.Preference,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	cp := *preference
	f.row = &cp

	return nil
}

// fakeTemplateStore serves scripted templates.
type fakeTemplateStore struct {
	mu        sync.Mutex
	templates map[string]*model.Template
}

func templateKey(notificationType string, channel model.Channel) string {
	return notificationType + ":" + string(channel)
}

func (f *fakeTemplateStore) GetTemplate(
	_ context.Context,
	notificationType string,
	channel model.Channel,
) (*model.Template, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	template, ok := f.templates[templateKey(notificationType, channel)]
	if !ok {
		return nil, nil
	}

	cp := *template

	return &cp, nil
}

// fakeDeliveryStore records dispatch outcomes.
type fakeDeliveryStore struct {
	mu      sync.Mutex
	created [][]model.Channel
	sent    []uuid.UUID
	failed  []uuid.UUID
}

func (f *fakeDeliveryStore) ListByNotification(
	_ context.Context,
	_ uuid.UUID,
) ([]*model.Delivery, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// The last created channel set drives one delivery row each.
	if len(f.created) == 0 {
		return []*model.Delivery{}, nil
	}

	out := []*model.Delivery{}

	for _, channel := range f.created[len(f.created)-1] {
		out = append(out, &model.Delivery{
			ID:      uuid.New(),
			Channel: channel,
			Status:  model.DeliveryPending,
		})
	}

	return out, nil
}

func (f *fakeDeliveryStore) MarkSent(
	_ context.Context,
	id uuid.UUID,
	_ string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.sent = append(f.sent, id)

	return nil
}

func (f *fakeDeliveryStore) MarkFailed(
	_ context.Context,
	id uuid.UUID,
	_ int32,
	_ string,
	_ string,
	_ *time.Time,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.failed = append(f.failed, id)

	return nil
}

func (f *fakeDeliveryStore) ClaimDue(
	_ context.Context,
	_ time.Time,
) (*model.Delivery, error) {
	return nil, repository.ErrDeliveryNotFound
}

func (f *fakeDeliveryStore) CancelPending(
	_ context.Context,
	_ uuid.UUID,
	_ model.Channel,
) error {
	return nil
}

// fakeEmailProvider records sends with scripted failures.
type fakeEmailProvider struct {
	mu   sync.Mutex
	sent []provider.EmailRequest
	err  error
}

func (f *fakeEmailProvider) Send(
	_ context.Context,
	request provider.EmailRequest,
) (provider.EmailResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return provider.EmailResponse{}, f.err
	}

	f.sent = append(f.sent, request)

	return provider.EmailResponse{MessageID: "test-msg"}, nil
}

type fixture struct {
	notifications *NotificationService
	delivery      *DeliveryService
	preferences   *PreferenceService

	notificationStore *fakeNotificationStore
	deliveryStore     *fakeDeliveryStore
	preferenceStore   *fakePreferenceStore
	templateStore     *fakeTemplateStore
	email             *fakeEmailProvider
}

func newFixture() *fixture {
	notificationStore := newFakeNotificationStore()
	deliveryStore := &fakeDeliveryStore{}
	notificationStore.deliveries = deliveryStore
	preferenceStore := &fakePreferenceStore{}
	templateStore := &fakeTemplateStore{templates: make(map[string]*model.Template)}
	email := &fakeEmailProvider{}

	delivery, err := NewDeliveryService(
		deliveryStore,
		templateStore,
		email,
		notificationStore,
		4,
	)
	if err != nil {
		panic(err)
	}

	notifications, err := NewNotificationService(
		notificationStore,
		preferenceStore,
		templateStore,
		delivery,
	)
	if err != nil {
		panic(err)
	}

	preferences, err := NewPreferenceService(preferenceStore)
	if err != nil {
		panic(err)
	}

	return &fixture{
		notifications:     notifications,
		delivery:          delivery,
		preferences:       preferences,
		notificationStore: notificationStore,
		deliveryStore:     deliveryStore,
		preferenceStore:   preferenceStore,
		templateStore:     templateStore,
		email:             email,
	}
}

func TestCreateNotification(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()

	created, err := fx.notifications.Create(ctx, CreateNotificationRequest{
		UserID:   userID,
		Type:     ntype.PaymentCaptured,
		Title:    "Payment successful",
		Body:     "Done.",
		Priority: model.PriorityNormal,
		EventID:  "evt-1",
		Channels: []model.Channel{model.ChannelInApp, model.ChannelEmail},
		Email:    "user@example.com",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if created.Duplicate {
		t.Fatal("first create is not a duplicate")
	}

	if created.Notification.Title != "Payment successful" {
		t.Fatal("without templates, request content passes through")
	}

	// Duplicate event id resolves idempotently.
	again, err := fx.notifications.Create(ctx, CreateNotificationRequest{
		UserID:   userID,
		Type:     ntype.PaymentCaptured,
		Title:    "Other",
		Body:     "Other.",
		EventID:  "evt-1",
		Channels: []model.Channel{model.ChannelInApp},
	})
	if err != nil {
		t.Fatalf("duplicate: %v", err)
	}

	if !again.Duplicate || again.Notification.ID != created.Notification.ID {
		t.Fatal("expected idempotent replay")
	}

	// Unknown types rejected.
	if _, err := fx.notifications.Create(ctx, CreateNotificationRequest{
		UserID:   userID,
		Type:     "bogus.type",
		Title:    "x",
		Body:     "y",
		Channels: []model.Channel{model.ChannelInApp},
	}); !errors.Is(err, ErrInvalidType) {
		t.Fatalf("expected invalid type, got %v", err)
	}

	// Unknown channels rejected.
	if _, err := fx.notifications.Create(ctx, CreateNotificationRequest{
		UserID:   userID,
		Type:     ntype.PaymentCaptured,
		Title:    "x",
		Body:     "y",
		Channels: []model.Channel{"carrier_pigeon"},
	}); !errors.Is(err, ErrInvalidChannel) {
		t.Fatalf("expected invalid channel, got %v", err)
	}

	// Email channel without address rejected.
	if _, err := fx.notifications.Create(ctx, CreateNotificationRequest{
		UserID:   userID,
		Type:     ntype.PaymentCaptured,
		Title:    "x",
		Body:     "y",
		Channels: []model.Channel{model.ChannelEmail},
	}); !errors.Is(err, ErrEmailRequired) {
		t.Fatalf("expected email required, got %v", err)
	}
}

func TestPreferencesGateChannels(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()

	// User disables email: email rows never created.
	fx.preferenceStore.row = &model.Preference{
		UserID:       userID,
		EmailEnabled: false,
		InAppEnabled: true,
	}

	created, err := fx.notifications.Create(ctx, CreateNotificationRequest{
		UserID:   userID,
		Type:     ntype.PaymentCaptured,
		Title:    "x",
		Body:     "y",
		Channels: []model.Channel{model.ChannelInApp, model.ChannelEmail},
		Email:    "user@example.com",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if len(fx.email.sent) != 0 {
		t.Fatal("disabled email must not send")
	}

	_ = created

	// Security bypasses every toggle, even all-off.
	fx.preferenceStore.row = &model.Preference{
		UserID:       userID,
		EmailEnabled: false,
		InAppEnabled: false,
	}

	security, err := fx.notifications.Create(ctx, CreateNotificationRequest{
		UserID:   userID,
		Type:     ntype.AuthPasswordReset,
		Title:    "Reset",
		Body:     "Reset it.",
		Channels: []model.Channel{model.ChannelInApp, model.ChannelEmail},
		Email:    "user@example.com",
	})
	if err != nil {
		t.Fatalf("security create: %v", err)
	}

	_ = security

	if len(fx.email.sent) != 1 {
		t.Fatalf("security email must send, got %d", len(fx.email.sent))
	}

	// Category toggle: payment disabled skips payment types.
	fx.preferenceStore.row = &model.Preference{
		UserID:         userID,
		EmailEnabled:   true,
		InAppEnabled:   true,
		PaymentEnabled: false,
	}

	_, err = fx.notifications.Create(ctx, CreateNotificationRequest{
		UserID:   userID,
		Type:     ntype.PaymentCaptured,
		Title:    "x",
		Body:     "y",
		Channels: []model.Channel{model.ChannelInApp},
	})
	if err != nil {
		t.Fatalf("category-gated create: %v", err)
	}
}

func TestTemplateRendering(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()

	subject := "Payment of {{.amount}} received"
	body := "Hi {{.name}}, {{.missing}}!"

	fx.templateStore.templates[templateKey(ntype.PaymentCaptured, model.ChannelEmail)] = &model.Template{
		SubjectTemplate: &subject,
		BodyTemplate:    body,
	}

	// Missing variable fails loudly, nothing stored half-rendered.
	if _, err := fx.notifications.Create(ctx, CreateNotificationRequest{
		UserID:   userID,
		Type:     ntype.PaymentCaptured,
		Title:    "fallback",
		Body:     "fallback",
		Data:     map[string]string{"amount": "₹999"},
		Channels: []model.Channel{model.ChannelEmail},
		Email:    "user@example.com",
	}); err == nil {
		t.Fatal("expected render error for missing variable")
	}

	created, err := fx.notifications.Create(ctx, CreateNotificationRequest{
		UserID:   userID,
		Type:     ntype.PaymentCaptured,
		Title:    "fallback",
		Body:     "fallback",
		Data:     map[string]string{"amount": "₹999", "name": "Anshul", "missing": "x"},
		Channels: []model.Channel{model.ChannelEmail},
		Email:    "user@example.com",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if created.Email == nil || created.Email.Subject != "Payment of ₹999 received" {
		t.Fatalf("unexpected subject: %+v", created.Email)
	}

	if !strings.Contains(created.Email.Body, "Hi Anshul") {
		t.Fatalf("unexpected body: %q", created.Email.Body)
	}
}

func TestRetrySchedule(t *testing.T) {
	now := time.Now()

	expected := []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour}

	for i, delay := range expected {
		next, ok := NextRetry(int32(i+1), now)
		if !ok {
			t.Fatalf("attempt %d must schedule", i+1)
		}

		if next.Sub(now) != delay {
			t.Fatalf("attempt %d: got %v, want %v", i+1, next.Sub(now), delay)
		}
	}

	if _, ok := NextRetry(5, now); ok {
		t.Fatal("attempt 5 must be terminal")
	}

	if _, ok := NextRetry(0, now); ok {
		t.Fatal("attempt 0 is invalid")
	}

	if MaxAttempts() != 4 {
		t.Fatalf("expected 4 attempts, got %d", MaxAttempts())
	}
}

func TestRenderTemplate(t *testing.T) {
	out, err := RenderTemplate("Pay {{.amount}} now", map[string]string{"amount": "₹999"})
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	if out != "Pay ₹999 now" {
		t.Fatalf("got %q", out)
	}

	if _, err := RenderTemplate("Hi {{.name}}", map[string]string{}); err == nil {
		t.Fatal("expected missing-variable error")
	}

	if _, err := RenderTemplate("Hi {{.name", map[string]string{}); err == nil {
		t.Fatal("expected syntax error")
	}
}

func TestPreferenceService(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()

	// Defaults when never customized.
	def, err := fx.preferences.Get(ctx, userID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if !def.EmailEnabled || def.MarketingEnabled || !def.SecurityEnabled {
		t.Fatalf("unexpected defaults: %+v", def)
	}

	// Update coerces security on, even when explicitly disabled.
	email := false
	security := false

	updated, err := fx.preferences.Update(ctx, userID, UpdatePreferencesInput{
		EmailEnabled:    &email,
		SecurityEnabled: &security,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.EmailEnabled {
		t.Fatal("expected email off")
	}

	if !updated.SecurityEnabled {
		t.Fatal("security must stay enabled")
	}

	stored, err := fx.preferences.Get(ctx, userID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if stored.EmailEnabled || !stored.SecurityEnabled {
		t.Fatal("stored preferences mismatch")
	}
}

func TestReadAndDelete(t *testing.T) {
	fx := newFixture()
	ctx := context.Background()
	userID := uuid.New()
	otherID := uuid.New()

	created, err := fx.notifications.Create(ctx, CreateNotificationRequest{
		UserID:   userID,
		Type:     ntype.PaymentCaptured,
		Title:    "x",
		Body:     "y",
		Channels: []model.Channel{model.ChannelInApp},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	// Foreign reads fail identically to missing.
	if _, err := fx.notifications.MarkRead(
		ctx,
		otherID,
		created.Notification.ID,
	); !errors.Is(err, ErrNotificationNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}

	read, err := fx.notifications.MarkRead(ctx, userID, created.Notification.ID)
	if err != nil {
		t.Fatalf("mark read: %v", err)
	}

	if !read.IsRead() {
		t.Fatal("expected read")
	}

	// Idempotent repeat.
	if _, err := fx.notifications.MarkRead(
		ctx,
		userID,
		created.Notification.ID,
	); err != nil {
		t.Fatalf("repeat: %v", err)
	}

	count, err := fx.notifications.UnreadCount(ctx, userID)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected 0 unread, got %d", count)
	}

	if err := fx.notifications.DeleteNotification(
		ctx,
		otherID,
		created.Notification.ID,
	); !errors.Is(err, ErrNotificationNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}

	if err := fx.notifications.DeleteNotification(
		ctx,
		userID,
		created.Notification.ID,
	); err != nil {
		t.Fatalf("delete: %v", err)
	}
}
