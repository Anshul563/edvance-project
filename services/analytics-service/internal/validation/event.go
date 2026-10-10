package validation

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/analytics-service/internal/models"
)

var ErrInvalidEvent = errors.New("invalid event")

func ValidateEvent(event models.Event) error {
	if strings.TrimSpace(event.EventID) == "" {
		return fmt.Errorf("%w: event_id is required", ErrInvalidEvent)
	}
	if _, err := uuid.Parse(event.EventID); err != nil {
		return fmt.Errorf("%w: event_id must be a valid UUID", ErrInvalidEvent)
	}
	if strings.TrimSpace(event.EventType) == "" {
		return fmt.Errorf("%w: event_type is required", ErrInvalidEvent)
	}
	if _, ok := models.SupportedEventTypes[event.EventType]; !ok {
		return fmt.Errorf("%w: unsupported event type %q", ErrInvalidEvent, event.EventType)
	}
	if event.EventVersion <= 0 {
		return fmt.Errorf("%w: event_version must be positive", ErrInvalidEvent)
	}
	if event.OccurredAt.IsZero() {
		return fmt.Errorf("%w: occurred_at is required", ErrInvalidEvent)
	}
	if event.OccurredAt.After(time.Now().Add(5 * time.Minute)) {
		return fmt.Errorf("%w: occurred_at cannot be in the future", ErrInvalidEvent)
	}
	if strings.TrimSpace(event.Source) == "" {
		return fmt.Errorf("%w: source_service is required", ErrInvalidEvent)
	}
	if strings.TrimSpace(event.EntityType) == "" {
		return fmt.Errorf("%w: entity_type is required", ErrInvalidEvent)
	}
	if strings.TrimSpace(event.EntityID) == "" {
		return fmt.Errorf("%w: entity_id is required", ErrInvalidEvent)
	}
	if strings.TrimSpace(event.SchemaVersion) == "" {
		return fmt.Errorf("%w: schema_version is required", ErrInvalidEvent)
	}
	if len(event.Properties) > 25 {
		return fmt.Errorf("%w: properties exceeds supported limit", ErrInvalidEvent)
	}
	for key, value := range event.Properties {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("%w: property names are required", ErrInvalidEvent)
		}
		if utf8.RuneCountInString(key) > 64 {
			return fmt.Errorf("%w: property name too long", ErrInvalidEvent)
		}
		if isSensitiveKey(key) {
			return fmt.Errorf("%w: sensitive data is not allowed in properties", ErrInvalidEvent)
		}
		if !isSafePropertyValue(value) {
			return fmt.Errorf("%w: unsupported property value type", ErrInvalidEvent)
		}
	}
	if strings.TrimSpace(event.ActorID) == "" && strings.TrimSpace(event.AnonymousID) == "" {
		return fmt.Errorf("%w: actor_id or anonymous_id is required", ErrInvalidEvent)
	}
	return nil
}

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, blocked := range []string{"password", "token", "secret", "card", "cvv", "ssn", "private_message", "email", "ip_address"} {
		if strings.Contains(lower, blocked) {
			return true
		}
	}
	return false
}

func isSafePropertyValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return utf8.RuneCountInString(v) <= 256
	case bool, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case []any:
		if len(v) > 20 {
			return false
		}
		for _, item := range v {
			if !isSafePropertyValue(item) {
				return false
			}
		}
		return true
	case map[string]any:
		if len(v) > 10 {
			return false
		}
		for key, item := range v {
			if strings.TrimSpace(key) == "" || utf8.RuneCountInString(key) > 64 {
				return false
			}
			if !isSafePropertyValue(item) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
