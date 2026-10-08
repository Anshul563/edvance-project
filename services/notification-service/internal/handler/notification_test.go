package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/notification-service/internal/event"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/model"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/service"
	"github.com/Anshul563/edvance-project/services/notification-service/internal/token"
)

const (
	testSecret   = "test-access-secret-0123456789abcdef"
	testIssuer   = "edvance-auth"
	testAudience = "edvance-api"
	testInternal = "test-internal-token"
)

// stubNotifications implements the notification handler interface.
type stubNotifications struct {
	page *service.NotificationPage
	item *model.Notification
	err  error

	marked int64

	gotUserID uuid.UUID
}

func testNotification(userID uuid.UUID) *model.Notification {
	return &model.Notification{
		ID:        uuid.New(),
		UserID:    userID,
		Type:      "payment.captured",
		Title:     "Paid",
		Body:      "Done",
		Priority:  model.PriorityNormal,
		CreatedAt: time.Now(),
	}
}

func (s *stubNotifications) List(
	_ context.Context,
	userID uuid.UUID,
	_ bool,
	_ int,
	_ int,
) (*service.NotificationPage, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.page, nil
}

func (s *stubNotifications) UnreadCount(
	_ context.Context,
	userID uuid.UUID,
) (int64, error) {
	s.gotUserID = userID

	if s.err != nil {
		return 0, s.err
	}

	return 7, nil
}

func (s *stubNotifications) MarkRead(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) (*model.Notification, error) {
	s.gotUserID = userID

	if s.err != nil {
		return nil, s.err
	}

	return s.item, nil
}

func (s *stubNotifications) MarkAllRead(
	_ context.Context,
	userID uuid.UUID,
) (int64, error) {
	s.gotUserID = userID

	if s.err != nil {
		return 0, s.err
	}

	return s.marked, nil
}

func (s *stubNotifications) DeleteNotification(
	_ context.Context,
	userID uuid.UUID,
	_ uuid.UUID,
) error {
	s.gotUserID = userID

	return s.err
}

// stubPreferences implements the preference handler interface.
type stubPreferences struct {
	preference *model.Preference
	err        error
}

func (s *stubPreferences) Get(
	_ context.Context,
	_ uuid.UUID,
) (*model.Preference, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.preference, nil
}

func (s *stubPreferences) Update(
	_ context.Context,
	_ uuid.UUID,
	_ service.UpdatePreferencesInput,
) (*model.Preference, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.preference, nil
}

// stubEvents implements the event handler interface.
type stubEvents struct {
	created *service.CreatedNotification
	err     error

	gotEvent event.Event
}

func (s *stubEvents) Handle(
	_ context.Context,
	event event.Event,
) (*service.CreatedNotification, error) {
	s.gotEvent = event

	if s.err != nil {
		return nil, s.err
	}

	return s.created, nil
}

func issueTokenFor(userID uuid.UUID) string {
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

	signed, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(testSecret))

	return signed
}

func testMiddleware() (
	auth func(http.Handler) http.Handler,
	internal func(http.Handler) http.Handler,
) {
	cfg := middleware.AuthConfig{
		AccessSecret: testSecret,
		Issuer:       testIssuer,
		Audience:     testAudience,
	}

	return middleware.Authenticate(cfg), middleware.InternalOnly(testInternal)
}

func testRouter(
	notifications *stubNotifications,
	preferences *stubPreferences,
	events *stubEvents,
) http.Handler {
	auth, internal := testMiddleware()

	r := chi.NewRouter()
	r.With(auth).Get("/", NewNotificationHandler(notifications).List)
	r.With(auth).Get("/unread-count", NewNotificationHandler(notifications).UnreadCount)
	r.With(auth).Post("/read-all", NewNotificationHandler(notifications).MarkAllRead)
	r.With(auth).Post("/{notificationID}/read", NewNotificationHandler(notifications).MarkRead)
	r.With(auth).Delete("/{notificationID}", NewNotificationHandler(notifications).Delete)
	r.With(auth).Get("/preferences", NewPreferenceHandler(preferences).Get)
	r.With(auth).Patch("/preferences", NewPreferenceHandler(preferences).Update)
	r.With(internal).Post(
		"/internal/v1/notifications/events",
		NewEventHandler(events).Ingest,
	)

	return r
}

func authedRequest(
	method string,
	path string,
	body string,
	userID uuid.UUID,
) *http.Request {
	var req *http.Request

	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	}

	req.Header.Set("Authorization", "Bearer "+issueTokenFor(userID))

	return req
}

func emptyStubs(userID uuid.UUID) (*stubNotifications, *stubPreferences, *stubEvents) {
	notification := testNotification(userID)

	return &stubNotifications{
			page: &service.NotificationPage{
				Items:      []*model.Notification{notification},
				Total:      1,
				Page:       1,
				Limit:      20,
				TotalPages: 1,
			},
			item:   notification,
			marked: 3,
		},
		&stubPreferences{preference: model.Defaults(userID)},
		&stubEvents{
			created: &service.CreatedNotification{Notification: notification},
		}
}

func TestProtectedEndpointsRejectAnonymous(t *testing.T) {
	notifications, preferences, events := emptyStubs(uuid.New())
	r := testRouter(notifications, preferences, events)

	paths := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodGet, "/", ""},
		{http.MethodGet, "/unread-count", ""},
		{http.MethodPost, "/read-all", ""},
		{http.MethodPost, "/" + uuid.NewString() + "/read", ""},
		{http.MethodDelete, "/" + uuid.NewString(), ""},
		{http.MethodGet, "/preferences", ""},
		{http.MethodPatch, "/preferences", `{}`},
	}

	for _, tc := range paths {
		var req *http.Request

		if tc.body == "" {
			req = httptest.NewRequest(tc.method, tc.path, nil)
		} else {
			req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		}

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 for %s %s, got %d", tc.method, tc.path, rec.Code)
		}
	}
}

func TestListAndUnreadCount(t *testing.T) {
	userID := uuid.New()
	notifications, preferences, events := emptyStubs(userID)
	r := testRouter(notifications, preferences, events)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(http.MethodGet, "/?unread=true", "", userID))

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if notifications.gotUserID != userID {
		t.Fatal("identity must come from the JWT")
	}

	count := httptest.NewRecorder()
	r.ServeHTTP(count, authedRequest(http.MethodGet, "/unread-count", "", userID))

	if count.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", count.Code)
	}

	if !strings.Contains(count.Body.String(), `"count":7`) {
		t.Fatalf("unexpected body: %s", count.Body.String())
	}
}

func TestMarkReadNotFound(t *testing.T) {
	userID := uuid.New()
	notifications := &stubNotifications{err: service.ErrNotificationNotFound}
	_, preferences, events := emptyStubs(userID)
	_ = preferences

	rec := httptest.NewRecorder()
	testRouter(notifications, &stubPreferences{}, events).ServeHTTP(
		rec,
		authedRequest(http.MethodPost, "/"+uuid.NewString()+"/read", "", userID),
	)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), `"code":"NOTIFICATION_NOT_FOUND"`) {
		t.Fatalf("expected code, got %s", rec.Body.String())
	}
}

func TestUserIsolation(t *testing.T) {
	userID := uuid.New()
	otherNotification := testNotification(uuid.New())
	notifications := &stubNotifications{
		page: &service.NotificationPage{Items: []*model.Notification{otherNotification}},
		item: otherNotification,
	}
	_, preferences, events := emptyStubs(userID)

	rec := httptest.NewRecorder()
	testRouter(notifications, preferences, events).ServeHTTP(
		rec,
		authedRequest(http.MethodGet, "/", "", userID),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if notifications.gotUserID != userID {
		t.Fatal("listing must be scoped to the JWT identity")
	}
}

func TestInternalEventAuth(t *testing.T) {
	notifications, preferences, events := emptyStubs(uuid.New())
	r := testRouter(notifications, preferences, events)

	body := `{"eventId":"evt-1","type":"payment.captured",` +
		`"userId":"` + uuid.NewString() + `","data":{"email":"a@b.c"}}`

	// Missing token rejected.
	denied := httptest.NewRecorder()
	r.ServeHTTP(
		denied,
		httptest.NewRequest(
			http.MethodPost,
			"/internal/v1/notifications/events",
			strings.NewReader(body),
		),
	)

	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", denied.Code)
	}

	// Wrong token rejected.
	wrong := httptest.NewRequest(
		http.MethodPost,
		"/internal/v1/notifications/events",
		strings.NewReader(body),
	)
	wrong.Header.Set("Authorization", "Bearer wrong")

	wrongRec := httptest.NewRecorder()
	r.ServeHTTP(wrongRec, wrong)

	if wrongRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", wrongRec.Code)
	}

	// Wrong scheme rejected even with the right secret.
	schemed := httptest.NewRequest(
		http.MethodPost,
		"/internal/v1/notifications/events",
		strings.NewReader(body),
	)
	schemed.Header.Set("Authorization", "Token "+testInternal)

	schemedRec := httptest.NewRecorder()
	r.ServeHTTP(schemedRec, schemed)

	if schemedRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", schemedRec.Code)
	}

	// JWT (not internal token) rejected on the internal route.
	jwtReq := httptest.NewRequest(
		http.MethodPost,
		"/internal/v1/notifications/events",
		strings.NewReader(body),
	)
	jwtReq.Header.Set("Authorization", "Bearer "+issueTokenFor(uuid.New()))

	jwtRec := httptest.NewRecorder()
	r.ServeHTTP(jwtRec, jwtReq)

	if jwtRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", jwtRec.Code)
	}

	// Correct token accepted.
	okReq := httptest.NewRequest(
		http.MethodPost,
		"/internal/v1/notifications/events",
		strings.NewReader(body),
	)
	okReq.Header.Set("Authorization", "Bearer "+testInternal)

	okRec := httptest.NewRecorder()
	r.ServeHTTP(okRec, okReq)

	if okRec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", okRec.Code, okRec.Body.String())
	}

	if events.gotEvent.Type != "payment.captured" {
		t.Fatal("event must reach the handler")
	}

	if events.gotEvent.UserID == uuid.Nil {
		t.Fatal("target user comes from the trusted event, never the JWT")
	}
}

func TestInternalUnknownEvent(t *testing.T) {
	notifications, preferences, _ := emptyStubs(uuid.New())
	events := &stubEvents{err: event.ErrUnknownEvent}
	r := testRouter(notifications, preferences, events)

	req := httptest.NewRequest(
		http.MethodPost,
		"/internal/v1/notifications/events",
		strings.NewReader(`{"type":"bogus","userId":"`+uuid.NewString()+`"}`),
	)
	req.Header.Set("Authorization", "Bearer "+testInternal)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestInternalOversizedBody(t *testing.T) {
	notifications, preferences, events := emptyStubs(uuid.New())
	r := testRouter(notifications, preferences, events)

	// 1MB cap: oversized payloads fail closed before decode.
	big := `{"eventId":"x","type":"payment.captured","userId":"` +
		uuid.NewString() + `","data":{"pad":"` + strings.Repeat("x", 2<<20) + `"}}`

	req := httptest.NewRequest(
		http.MethodPost,
		"/internal/v1/notifications/events",
		strings.NewReader(big),
	)
	req.Header.Set("Authorization", "Bearer "+testInternal)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestPreferenceSecurityCoercion(t *testing.T) {
	userID := uuid.New()
	notifications, _, events := emptyStubs(userID)
	preferences := &stubPreferences{preference: model.Defaults(userID)}

	rec := httptest.NewRecorder()
	testRouter(notifications, preferences, events).ServeHTTP(
		rec,
		authedRequest(
			http.MethodPatch,
			"/preferences",
			`{"securityEnabled":false,"emailEnabled":false}`,
			userID,
		),
	)

	// The stub echoes defaults; the coercion itself is unit-tested at
	// the service layer. Here we assert the endpoint accepts the shape.
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
