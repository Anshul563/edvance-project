//go:build integration

package router

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/event"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/provider"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/service"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
	testInternal = "test-internal-token"
)

func issueFlowToken(t *testing.T, userID uuid.UUID) string {
	t.Helper()

	now := time.Now()

	claims := token.AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			ID:        uuid.NewString(),
			Issuer:    testIssuer,
			Audience:  jwt.ClaimStrings{testAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		},
	}

	signed, err := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return signed
}

// Full notification flow against real PostgreSQL:
//
//	internal event -> stored + email sent (console) -> list ->
//	unread count -> mark read -> mark all read -> duplicate event ->
//	preferences gate -> delete, plus cross-user isolation.
//
// DATABASE_URL must point at edvance_notification.
func TestNotificationFlowIntegration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := repository.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer pool.Close()

	notificationRepo := repository.NewNotificationRepository(pool)
	deliveryRepo := repository.NewDeliveryRepository(pool)
	preferenceRepo := repository.NewPreferenceRepository(pool)
	templateRepo := repository.NewTemplateRepository(pool)

	deliveryService, err := service.NewDeliveryService(
		deliveryRepo,
		templateRepo,
		provider.NewConsoleProvider(),
		notificationRepo,
	)
	if err != nil {
		t.Fatalf("delivery service: %v", err)
	}

	notificationService, err := service.NewNotificationService(
		notificationRepo,
		preferenceRepo,
		templateRepo,
		deliveryService,
	)
	if err != nil {
		t.Fatalf("notification service: %v", err)
	}

	preferenceService, err := service.NewPreferenceService(preferenceRepo)
	if err != nil {
		t.Fatalf("preference service: %v", err)
	}

	r := New(
		Handlers{
			Health:       handler.NewHealthHandler(pool),
			Notification: handler.NewNotificationHandler(notificationService),
			Preference:   handler.NewPreferenceHandler(preferenceService),
			Event:        handler.NewEventHandler(event.NewHandler(notificationService)),
		},
		middleware.Authenticate(middleware.AuthConfig{
			AccessSecret: testSecret,
			Issuer:       testIssuer,
			Audience:     testAudience,
		}),
		middleware.InternalOnly(testInternal),
	)

	userID := uuid.New()
	otherID := uuid.New()
	userToken := issueFlowToken(t, userID)
	otherToken := issueFlowToken(t, otherID)

	defer func() {
		for _, id := range []uuid.UUID{userID, otherID} {
			_, _ = pool.Exec(
				context.Background(),
				`DELETE FROM notification_deliveries WHERE notification_id IN (
					SELECT id FROM notifications WHERE user_id = $1
				)`,
				id,
			)
			_, _ = pool.Exec(
				context.Background(),
				`DELETE FROM notifications WHERE user_id = $1`,
				id,
			)
			_, _ = pool.Exec(
				context.Background(),
				`DELETE FROM notification_preferences WHERE user_id = $1`,
				id,
			)
		}
	}()

	serve := func(
		method string,
		path string,
		token string,
		internalToken string,
		body string,
	) *httptest.ResponseRecorder {
		t.Helper()

		var req *http.Request

		if body == "" {
			req = httptest.NewRequest(method, path, nil)
		} else {
			req = httptest.NewRequest(method, path, strings.NewReader(body))
		}

		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		if internalToken != "" {
			req.Header.Set("X-Internal-Token", internalToken)
		}

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		return rec
	}

	decode := func(rec *httptest.ResponseRecorder) map[string]any {
		t.Helper()

		var body map[string]any

		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode: %v", err)
		}

		return body
	}

	eventBody := func(eventID string) string {
		return `{"eventId":"` + eventID + `","type":"payment.captured",` +
			`"userId":"` + userID.String() + `",` +
			`"data":{"email":"user@example.com","amount":"₹999"}}`
	}

	// Internal event without token: rejected.
	denied := serve(
		http.MethodPost,
		"/internal/v1/notifications/events",
		"",
		"",
		eventBody("evt-flow-1"),
	)
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", denied.Code)
	}

	// Internal event creates notification + sends email via console.
	ingested := serve(
		http.MethodPost,
		"/internal/v1/notifications/events",
		"",
		testInternal,
		eventBody("evt-flow-1"),
	)
	if ingested.Code != http.StatusCreated {
		t.Fatalf("ingest: %d %s", ingested.Code, ingested.Body.String())
	}

	notificationID, _ := decode(ingested)["id"].(string)

	// Duplicate event replays idempotently.
	replay := serve(
		http.MethodPost,
		"/internal/v1/notifications/events",
		"",
		testInternal,
		eventBody("evt-flow-1"),
	)
	if replay.Code != http.StatusOK {
		t.Fatalf("expected 200 replay, got %d", replay.Code)
	}

	replayBody := decode(replay)

	if replayBody["duplicate"] != true || replayBody["id"] != notificationID {
		t.Fatalf("expected idempotent replay, got %v", replayBody)
	}

	// List shows it; unread count is 1.
	listed := serve(http.MethodGet, "/", userToken, "", "")
	if listed.Code != http.StatusOK {
		t.Fatalf("list: %d", listed.Code)
	}

	unread := serve(http.MethodGet, "/unread-count", userToken, "", "")
	if unread.Code != http.StatusOK {
		t.Fatalf("unread: %d", unread.Code)
	}

	if decode(unread)["count"] != float64(1) {
		t.Fatalf("expected 1 unread, got %s", unread.Body.String())
	}

	// Mark read.
	read := serve(
		http.MethodPost,
		"/"+notificationID+"/read",
		userToken,
		"",
		"",
	)
	if read.Code != http.StatusOK {
		t.Fatalf("mark read: %d", read.Code)
	}

	// Other user cannot read it.
	foreign := serve(
		http.MethodPost,
		"/"+notificationID+"/read",
		otherToken,
		"",
		"",
	)
	if foreign.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", foreign.Code)
	}

	// Disable email, ingest again: no email row this time.
	disabled := serve(
		http.MethodPatch,
		"/preferences",
		userToken,
		"",
		`{"emailEnabled":false}`,
	)
	if disabled.Code != http.StatusOK {
		t.Fatalf("preferences: %d", disabled.Code)
	}

	second := serve(
		http.MethodPost,
		"/internal/v1/notifications/events",
		"",
		testInternal,
		eventBody("evt-flow-2"),
	)
	if second.Code != http.StatusCreated {
		t.Fatalf("second ingest: %d", second.Code)
	}

	secondID, _ := decode(second)["id"].(string)

	var emailRows int

	err = pool.QueryRow(
		context.Background(),
		`SELECT COUNT(*) FROM notification_deliveries
		 WHERE notification_id = $1 AND channel = 'email'`,
		secondID,
	).Scan(&emailRows)
	if err != nil {
		t.Fatalf("count email rows: %v", err)
	}

	if emailRows != 0 {
		t.Fatal("disabled email must create no email delivery")
	}

	// Delete both; cross-user delete fails.
	if rec := serve(
		http.MethodDelete,
		"/"+notificationID,
		otherToken,
		"",
		"",
	); rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	for _, id := range []string{notificationID, secondID} {
		if rec := serve(
			http.MethodDelete,
			"/"+id,
			userToken,
			"",
			"",
		); rec.Code != http.StatusOK {
			t.Fatalf("delete %s: %d", id, rec.Code)
		}
	}

	empty := serve(http.MethodGet, "/", userToken, "", "")
	if decode(empty)["pagination"].(map[string]any)["total"] != float64(0) {
		t.Fatalf("expected empty list, got %s", empty.Body.String())
	}
}
