package model

import (
	"time"

	"github.com/google/uuid"
)

type WebhookStatus string

const (
	WebhookReceived  WebhookStatus = "received"
	WebhookProcessed WebhookStatus = "processed"
	WebhookFailed    WebhookStatus = "failed"
)

// WebhookEvent is the idempotent inbound event store. (Provider,
// EventID) is the replay key: a retried delivery resolves to the stored
// record instead of re-applying business effects.
type WebhookEvent struct {
	ID           uuid.UUID
	Provider     string
	EventID      *string
	EventType    string
	Payload      string
	Signature    *string
	Status       WebhookStatus
	ErrorMessage *string
	ReceivedAt   time.Time
	ProcessedAt  *time.Time
}
