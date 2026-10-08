// Package notifier publishes internal business events to
// notification-service over its internal HTTP event endpoint. Delivery
// is best-effort: content publishing must never fail because a
// notification could not be queued.
package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EventContentPublished is the content-service publish event. It is a
// RESERVED future type: notification-service does not yet map it to a
// template, so delivery currently answers "unknown event type" and we
// log it without failing the publish. When notification-service grows a
// content.published mapping this starts rendering with zero changes
// here.
//
// Deliberately NOT course.published: courses belong to course-service,
// and inventing business semantics for another domain is not this
// service's job.
const EventContentPublished = "content.published"

// Event is the transport-independent payload. Today it goes over HTTP;
// a future NATS publisher can send the same struct.
type Event struct {
	EventID string
	Type    string
	UserID  uuid.UUID
	Data    map[string]string
}

// Notifier is the publishing seam the service depends on. Tests inject
// fakes; production wires either HTTPNotifier or Noop.
type Notifier interface {
	Notify(ctx context.Context, event Event) error
}

// Noop is the disabled notifier used when no notification service is
// configured. It accepts every event and does nothing.
func Noop() Notifier {
	return noopNotifier{}
}

type noopNotifier struct{}

func (noopNotifier) Notify(context.Context, Event) error {
	return nil
}

// HTTPNotifier posts events to notification-service's internal
// endpoint, which is guarded by a shared service token (never a user
// JWT) and is never proxied by the API gateway.
type HTTPNotifier struct {
	endpoint string
	token    string
	client   *http.Client
}

// NewHTTP builds a notifier for notification-service. baseURL is the
// service root, e.g. http://localhost:8092.
func NewHTTP(baseURL string, token string) *HTTPNotifier {
	return &HTTPNotifier{
		endpoint: strings.TrimRight(baseURL, "/") +
			"/internal/v1/notifications/events",
		token: token,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type eventPayload struct {
	EventID string            `json:"eventId"`
	Type    string            `json:"type"`
	UserID  string            `json:"userId"`
	Data    map[string]string `json:"data"`
}

// Notify sends one event. The context is detached from the caller so a
// client that disconnects mid-request cannot cancel a delivery that has
// already been committed to the database.
func (n *HTTPNotifier) Notify(ctx context.Context, event Event) error {
	if event.UserID == uuid.Nil {
		return fmt.Errorf("notifier: user id is required")
	}

	body, err := json.Marshal(eventPayload{
		EventID: event.EventID,
		Type:    event.Type,
		UserID:  event.UserID.String(),
		Data:    event.Data,
	})
	if err != nil {
		return fmt.Errorf("notifier: encode event: %w", err)
	}

	sendCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		5*time.Second,
	)
	defer cancel()

	req, err := http.NewRequestWithContext(
		sendCtx,
		http.MethodPost,
		n.endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("notifier: build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+n.token)

	response, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("notifier: post event: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf(
			"notifier: notification service returned status %d",
			response.StatusCode,
		)
	}

	return nil
}
