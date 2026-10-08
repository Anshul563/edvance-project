package model

import (
	"time"

	"github.com/google/uuid"
)

type Channel string

const (
	ChannelInApp Channel = "in_app"
	ChannelEmail Channel = "email"
	ChannelPush  Channel = "push"
	ChannelSMS   Channel = "sms"
)

type DeliveryStatus string

const (
	DeliveryPending    DeliveryStatus = "pending"
	DeliveryProcessing DeliveryStatus = "processing"
	DeliverySent       DeliveryStatus = "sent"
	DeliveryFailed     DeliveryStatus = "failed"
	DeliveryCancelled  DeliveryStatus = "cancelled"
)

// Delivery tracks one channel's send state for a notification. Email can
// fail and retry independently while the in-app row stays sent; push/sms
// exist in the domain for future providers without schema changes.
type Delivery struct {
	ID                uuid.UUID
	NotificationID    uuid.UUID
	Channel           Channel
	Status            DeliveryStatus
	AttemptCount      int32
	ProviderMessageID *string
	ErrorCode         *string
	ErrorMessage      *string
	NextRetryAt       *time.Time
	SentAt            *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Terminal reports whether the delivery will never change again
// (success, exhaustion, or explicit cancellation).
func (d *Delivery) Terminal() bool {
	return d.Status == DeliverySent ||
		d.Status == DeliveryFailed ||
		d.Status == DeliveryCancelled
}
