//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
)

// Needs PostgreSQL with migration 001 applied to edvance_notification:
//
//	DATABASE_URL=postgres://... go test -tags integration ./internal/repository/
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

func cleanupNotification(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM notification_deliveries WHERE notification_id = $1`,
			id,
		)
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM notifications WHERE id = $1`,
			id,
		)
	})
}

func seedNotification(
	t *testing.T,
	pool *pgxpool.Pool,
	userID uuid.UUID,
	eventID string,
) *model.Notification {
	t.Helper()

	repo := NewNotificationRepository(pool)

	notification := &model.Notification{
		UserID:   userID,
		Type:     "payment.captured",
		Title:    "Paid",
		Body:     "Done.",
		Priority: model.PriorityNormal,
	}

	if eventID != "" {
		notification.EventID = &eventID
	}

	if err := repo.CreateWithDeliveries(
		context.Background(),
		notification,
		[]model.Channel{model.ChannelInApp, model.ChannelEmail},
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	cleanupNotification(t, pool, notification.ID)

	return notification
}

func TestNotificationRepositoryCRUD(t *testing.T) {
	pool := newTestPool(t)
	repo := NewNotificationRepository(pool)
	ctx := context.Background()
	userID := uuid.New()

	notification := seedNotification(t, pool, userID, "evt-crud-1")

	if notification.ID == uuid.Nil {
		t.Fatal("expected id to be set")
	}

	// Deliveries created atomically with the notification.
	var deliveries int

	err := pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM notification_deliveries WHERE notification_id = $1`,
		notification.ID,
	).Scan(&deliveries)
	if err != nil {
		t.Fatalf("count deliveries: %v", err)
	}

	if deliveries != 2 {
		t.Fatalf("expected 2 deliveries, got %d", deliveries)
	}

	byID, err := repo.FindByID(ctx, notification.ID)
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if byID.Title != "Paid" {
		t.Fatal("wrong notification")
	}

	byEvent, err := repo.FindByEventID(ctx, "evt-crud-1")
	if err != nil {
		t.Fatalf("find by event: %v", err)
	}

	if byEvent.ID != notification.ID {
		t.Fatal("wrong notification by event")
	}

	if _, err := repo.FindByID(ctx, uuid.New()); !errors.Is(
		err,
		ErrNotificationNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestNotificationDuplicateEvent(t *testing.T) {
	pool := newTestPool(t)
	repo := NewNotificationRepository(pool)
	ctx := context.Background()
	userID := uuid.New()

	seedNotification(t, pool, userID, "evt-dup-1")

	dup := &model.Notification{
		UserID:   userID,
		Type:     "payment.captured",
		Title:    "Other",
		Body:     "Other.",
		Priority: model.PriorityNormal,
		EventID:  strPtr("evt-dup-1"),
	}

	if err := repo.CreateWithDeliveries(
		ctx,
		dup,
		[]model.Channel{model.ChannelInApp},
	); !errors.Is(err, ErrDuplicateEvent) {
		t.Fatalf("expected duplicate, got %v", err)
	}

	// The duplicate adopted the winner's row: no second notification.
	var count int

	err := pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM notifications WHERE event_id = $1`,
		"evt-dup-1",
	).Scan(&count)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}
}

func TestNotificationConcurrentDuplicates(t *testing.T) {
	pool := newTestPool(t)
	repo := NewNotificationRepository(pool)
	ctx := context.Background()
	userID := uuid.New()

	const racers = 8

	var wg sync.WaitGroup
	var created atomic.Int32
	var dups atomic.Int32

	for i := 0; i < racers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			notification := &model.Notification{
				UserID:   userID,
				Type:     "payment.captured",
				Title:    "Paid",
				Body:     "Done.",
				Priority: model.PriorityNormal,
				EventID:  strPtr("evt-race-1"),
			}

			err := repo.CreateWithDeliveries(
				ctx,
				notification,
				[]model.Channel{model.ChannelInApp},
			)

			switch {
			case err == nil:
				created.Add(1)
				cleanupNotification(t, pool, notification.ID)

			case errors.Is(err, ErrDuplicateEvent):
				dups.Add(1)

			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	if created.Load() != 1 {
		t.Fatalf("expected exactly 1 creation, got %d", created.Load())
	}

	if dups.Load() != racers-1 {
		t.Fatalf("expected %d duplicates, got %d", racers-1, dups.Load())
	}
}

func TestNotificationReadDelete(t *testing.T) {
	pool := newTestPool(t)
	repo := NewNotificationRepository(pool)
	ctx := context.Background()
	userID := uuid.New()
	otherID := uuid.New()

	a := seedNotification(t, pool, userID, "")
	b := seedNotification(t, pool, userID, "")

	// Foreign reads fail identically to missing.
	if _, err := repo.MarkRead(ctx, a.ID, otherID); !errors.Is(
		err,
		ErrNotificationNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}

	read, err := repo.MarkRead(ctx, a.ID, userID)
	if err != nil {
		t.Fatalf("mark read: %v", err)
	}

	if !read.IsRead() {
		t.Fatal("expected read")
	}

	// Idempotent repeat.
	if _, err := repo.MarkRead(ctx, a.ID, userID); err != nil {
		t.Fatalf("repeat: %v", err)
	}

	unread, err := repo.CountByUser(ctx, userID, true)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if unread != 1 {
		t.Fatalf("expected 1 unread, got %d", unread)
	}

	marked, err := repo.MarkAllRead(ctx, userID)
	if err != nil {
		t.Fatalf("mark all: %v", err)
	}

	if marked != 1 {
		t.Fatalf("expected 1 marked, got %d", marked)
	}

	// Foreign deletion fails; own deletion cascades deliveries.
	if err := repo.Delete(ctx, b.ID, otherID); !errors.Is(
		err,
		ErrNotificationNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}

	if err := repo.Delete(ctx, b.ID, userID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	var deliveries int

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM notification_deliveries WHERE notification_id = $1`,
		b.ID,
	).Scan(&deliveries)
	if err != nil {
		t.Fatalf("count: %v", err)
	}

	if deliveries != 0 {
		t.Fatal("deliveries must cascade")
	}

	items, err := repo.ListByUser(ctx, userID, false, 10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(items))
	}
}

func TestDeliveryLifecycle(t *testing.T) {
	pool := newTestPool(t)
	repo := NewDeliveryRepository(pool)
	ctx := context.Background()
	userID := uuid.New()

	notification := seedNotification(t, pool, userID, "")

	deliveries, err := repo.ListByNotification(ctx, notification.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(deliveries) != 2 {
		t.Fatalf("expected 2 deliveries, got %d", len(deliveries))
	}

	var emailID uuid.UUID

	for _, delivery := range deliveries {
		if delivery.Channel == model.ChannelEmail {
			emailID = delivery.ID
		}
	}

	// Retry schedule: pending email is claimable immediately.
	claimed, err := repo.ClaimDue(ctx, time.Now())
	if err != nil {
		t.Fatalf("claim: %v", err)
	}

	if claimed.Status != model.DeliveryProcessing {
		t.Fatal("expected processing claim")
	}

	if err := repo.MarkSent(ctx, claimed.ID, "msg-1"); err != nil {
		t.Fatalf("mark sent: %v", err)
	}

	// Failed attempt with retry scheduled.
	retryAt := time.Now().Add(time.Minute)

	if err := repo.MarkFailed(ctx, emailID, 1, "send_failed", "boom", &retryAt); err != nil {
		t.Fatalf("mark failed: %v", err)
	}

	// Not yet due: nothing claimable (the other row is sent).
	if _, err := repo.ClaimDue(ctx, time.Now()); !errors.Is(
		err,
		ErrDeliveryNotFound,
	) {
		t.Fatalf("expected nothing due, got %v", err)
	}

	// Due retry claims again.
	claimed, err = repo.ClaimDue(ctx, time.Now().Add(2*time.Minute))
	if err != nil {
		t.Fatalf("claim retry: %v", err)
	}

	if claimed.ID != emailID {
		t.Fatal("expected the retry row")
	}

	// Terminal failure: no instant, no further claims.
	if err := repo.MarkFailed(ctx, emailID, 4, "send_failed", "boom", nil); err != nil {
		t.Fatalf("terminal fail: %v", err)
	}

	if _, err := repo.ClaimDue(ctx, time.Now().Add(24*time.Hour)); !errors.Is(
		err,
		ErrDeliveryNotFound,
	) {
		t.Fatalf("expected nothing due, got %v", err)
	}
}

func TestPreferencesRepository(t *testing.T) {
	pool := newTestPool(t)
	repo := NewPreferenceRepository(pool)
	ctx := context.Background()
	userID := uuid.New()

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM notification_preferences WHERE user_id = $1`,
			userID,
		)
	})

	// No row yet: nil, not an error.
	row, err := repo.GetPreferences(ctx, userID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if row != nil {
		t.Fatal("expected nil for uncustomized user")
	}

	preference := model.Defaults(userID)
	preference.EmailEnabled = false

	if err := repo.UpsertPreferences(ctx, preference); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	stored, err := repo.GetPreferences(ctx, userID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if stored.EmailEnabled || !stored.SecurityEnabled {
		t.Fatal("stored preferences mismatch")
	}

	preference.EmailEnabled = true

	if err := repo.UpsertPreferences(ctx, preference); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	again, err := repo.GetPreferences(ctx, userID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if !again.EmailEnabled {
		t.Fatal("expected updated row")
	}
}

func TestTemplatesRepository(t *testing.T) {
	pool := newTestPool(t)
	repo := NewTemplateRepository(pool)
	ctx := context.Background()

	missing, err := repo.GetTemplate(ctx, "payment.captured", model.ChannelEmail)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if missing != nil {
		t.Fatal("expected nil for unconfigured template")
	}

	subject := "Pay {{.amount}}"

	template := &model.Template{
		Type:            "payment.captured",
		Channel:         model.ChannelEmail,
		SubjectTemplate: &subject,
		BodyTemplate:    "Done {{.amount}}",
		Version:         1,
		Active:          true,
	}

	if err := repo.CreateTemplate(ctx, template); err != nil {
		t.Fatalf("create: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM notification_templates WHERE id = $1`,
			template.ID,
		)
	})

	found, err := repo.GetTemplate(ctx, "payment.captured", model.ChannelEmail)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if found == nil || found.BodyTemplate != "Done {{.amount}}" {
		t.Fatalf("unexpected template: %+v", found)
	}

	found.Active = false

	if err := repo.UpdateTemplate(ctx, found); err != nil {
		t.Fatalf("update: %v", err)
	}

	gone, err := repo.GetTemplate(ctx, "payment.captured", model.ChannelEmail)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if gone != nil {
		t.Fatal("inactive templates must not resolve")
	}

	if err := repo.UpdateTemplate(ctx, &model.Template{ID: uuid.New()}); !errors.Is(
		err,
		ErrTemplateNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func strPtr(s string) *string { return &s }
