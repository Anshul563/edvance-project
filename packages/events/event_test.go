package events

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEventValidatesAndRoundTrips(t *testing.T) {
	data := json.RawMessage(`{"video_id":"vid_1"}`)

	event, err := NewEvent(
		VideoProcessingCompleted,
		DefaultSchemaVersion,
		"video-service",
		data,
	)
	require.NoError(t, err)
	require.NotEmpty(t, event.ID)
	assert.Equal(t, VideoProcessingCompleted, event.Type)
	assert.Equal(t, DefaultSchemaVersion, event.Version)
	assert.Equal(t, "video-service", event.Source)
	assert.True(t, event.OccurredAt.Before(time.Now().Add(time.Second)))
	assert.Equal(t, data, event.Data)

	// JSON round trip preserves the envelope.
	raw, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded Event
	require.NoError(t, json.Unmarshal(raw, &decoded))
	assert.True(t, event.Equal(&decoded))
}

func TestNewEventRejectsInvalidInput(t *testing.T) {
	_, err := NewEvent("", 1, "video-service", nil)
	assert.ErrorIs(t, err, ErrInvalidEvent)

	_, err = NewEvent(VideoProcessingCompleted, 0, "video-service", nil)
	assert.ErrorIs(t, err, ErrInvalidEvent)

	_, err = NewEvent(VideoProcessingCompleted, 1, "", nil)
	assert.ErrorIs(t, err, ErrInvalidEvent)
}

func TestEventValidate(t *testing.T) {
	event := &Event{ID: "x", Type: "video.published", Version: 1, Source: "content-service", OccurredAt: time.Now(), Data: json.RawMessage(`{}`)}
	require.NoError(t, event.Validate())

	empty := &Event{}
	assert.ErrorIs(t, empty.Validate(), ErrInvalidEvent)

	zeroVersion := &Event{ID: "x", Type: "video.published", Version: 0, Source: "content-service", OccurredAt: time.Now()}
	assert.ErrorIs(t, zeroVersion.Validate(), ErrInvalidEvent)

	badJSON := &Event{ID: "x", Type: "video.published", Version: 1, Source: "content-service", OccurredAt: time.Now(), Data: json.RawMessage(`{not-json`)}
	assert.ErrorIs(t, badJSON.Validate(), ErrInvalidEvent)
}

func TestEventWithCorrelationAndOccurredAt(t *testing.T) {
	occurred := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	event, err := NewEvent(CoursePublished, 1, "course-service", nil)
	require.NoError(t, err)

	event.WithID("fixed-id").WithOccurredAt(occurred).WithCorrelationID("corr-1")

	assert.Equal(t, "fixed-id", event.ID)
	assert.Equal(t, "corr-1", event.CorrelationID)
	assert.Equal(t, occurred, event.OccurredAt)

	// Payload decodes an empty object into a map.
	var m map[string]any
	require.NoError(t, event.Payload(&m))
	assert.NotNil(t, m)
}

func TestEventPayloadDecodesTypedStruct(t *testing.T) {
	type shape struct {
		VideoID string `json:"video_id"`
		Status  string `json:"status"`
	}

	event, err := NewEvent(VideoProcessingCompleted, 1, "video-service",
		json.RawMessage(`{"video_id":"vid_1","status":"ready"}`))
	require.NoError(t, err)

	var got shape
	require.NoError(t, event.Payload(&got))
	assert.Equal(t, "vid_1", got.VideoID)
	assert.Equal(t, "ready", got.Status)
}

func TestEventNilDataDefaultsToObject(t *testing.T) {
	event, err := NewEvent(VideoUploaded, 1, "video-service", nil)
	require.NoError(t, err)

	assert.Equal(t, json.RawMessage(`{}`), event.Data)
}

func TestSubjectForMapsEventTypes(t *testing.T) {
	assert.Equal(t, "edvance.video.processing.completed", SubjectFor(VideoProcessingCompleted))
	assert.Equal(t, "edvance.course.published", SubjectFor(CoursePublished))
	assert.Equal(t, "edvance.order.paid", SubjectFor(OrderPaid))
}

func TestSubjectForInvalidType(t *testing.T) {
	assert.Equal(t, "edvance.invalid.bad", SubjectFor("bad"))
}
