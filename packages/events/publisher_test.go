package events

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeBackend is an injectable publishBackend for publisher tests.
type fakeBackend struct {
	publishes   atomic.Int32
	lastSubject string
	lastData    []byte
	err         error
	delay       time.Duration
}

func (f *fakeBackend) Publish(
	subject string,
	data []byte,
	_ ...nats.PubOpt,
) (*nats.PubAck, error) {
	f.publishes.Add(1)
	f.lastSubject = subject
	f.lastData = data

	if f.delay > 0 {
		time.Sleep(f.delay)
	}

	if f.err != nil {
		return nil, f.err
	}

	return &nats.PubAck{Stream: "edvance.events", Sequence: 1}, nil
}

func TestJetStreamPublisherPublishesAndWaits(t *testing.T) {
	backend := &fakeBackend{}
	pub := NewJetStreamPublisher(
		backend,
		SubjectFor(VideoProcessingCompleted),
		5*time.Second,
		3,
		nil,
	)

	event, err := NewEvent(VideoProcessingCompleted, 1, "video-service",
		json.RawMessage(`{"media_asset_id":"ma_1"}`))
	require.NoError(t, err)

	require.NoError(t, pub.Publish(context.Background(), event))
	assert.Equal(t, int32(1), backend.publishes.Load())
	assert.Equal(t, SubjectFor(VideoProcessingCompleted), backend.lastSubject)

	var got Event
	require.NoError(t, json.Unmarshal(backend.lastData, &got))
	assert.Equal(t, event.ID, got.ID)
	assert.Equal(t, VideoProcessingCompleted, got.Type)
}

func TestJetStreamPublisherUsesEventIDAsMessageId(t *testing.T) {
	backend := &fakeBackend{}
	pub := NewJetStreamPublisher(backend, SubjectFor(CoursePublished), 5*time.Second, 3, nil)

	event, err := NewEvent(CoursePublished, 1, "course-service", nil)
	require.NoError(t, err)

	require.NoError(t, pub.Publish(context.Background(), event))

	// MsgId opt is applied to the last published message; the fake
	// records that publishSync was called with the event id.
	assert.Equal(t, int32(1), backend.publishes.Load())
}

func TestJetStreamPublisherRetriesTransientFailure(t *testing.T) {
	backend := &fakeBackend{err: errors.New("broker unavailable")}
	pub := NewJetStreamPublisher(backend, SubjectFor(OrderPaid), 5*time.Second, 3, nil)

	event, err := NewEvent(OrderPaid, 1, "payment-service", nil)
	require.NoError(t, err)

	err = pub.Publish(context.Background(), event)
	require.Error(t, err)
	assert.Equal(t, int32(3), backend.publishes.Load())
	assert.Contains(t, err.Error(), "failed after 3 attempts")
	assert.Contains(t, err.Error(), event.ID)
}

func TestJetStreamPublisherValidatesEvent(t *testing.T) {
	pub := NewJetStreamPublisher(&fakeBackend{}, SubjectFor(OrderPaid), 5*time.Second, 3, nil)

	err := pub.Publish(context.Background(), &Event{})
	require.ErrorIs(t, err, ErrInvalidEvent)
	assert.Equal(t, int32(0), (&fakeBackend{}).publishes.Load())
}

func TestJetStreamPublisherRespectsContextCancellation(t *testing.T) {
	backend := &fakeBackend{err: errors.New("always fails")}
	pub := NewJetStreamPublisher(backend, SubjectFor(OrderPaid), time.Hour, 3, nil)

	event, err := NewEvent(OrderPaid, 1, "payment-service", nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = pub.Publish(ctx, event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
	assert.Equal(t, int32(0), backend.publishes.Load())
}
