package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// JobType identifies the unit of work. probe/transcode/thumbnail/hls/
// caption are dispatched to the external media engine; cleanup is
// storage work this service performs itself.
type JobType string

const (
	JobTypeProbe     JobType = "probe"
	JobTypeTranscode JobType = "transcode"
	JobTypeThumbnail JobType = "thumbnail"
	JobTypeHLS       JobType = "hls"
	JobTypeCaption   JobType = "caption"
	JobTypeCleanup   JobType = "cleanup"
)

// ValidJobTypes is the accepted set (matches the database CHECK).
var ValidJobTypes = map[JobType]bool{
	JobTypeProbe:     true,
	JobTypeTranscode: true,
	JobTypeThumbnail: true,
	JobTypeHLS:       true,
	JobTypeCaption:   true,
	JobTypeCleanup:   true,
}

// JobStatus is the queue lifecycle:
//
//	queued -> processing -> completed
//	                     -> failed   (after max attempts, or a
//	                                 terminal engine error)
//	queued -> cancelled
type JobStatus string

const (
	JobStatusQueued     JobStatus = "queued"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCancelled  JobStatus = "cancelled"
)

// ProcessingJob is one unit of work in the queue.
type ProcessingJob struct {
	ID           uuid.UUID
	MediaAssetID uuid.UUID
	JobType      JobType
	Status       JobStatus
	Attempts     int
	Priority     int
	ErrorMessage *string
	Payload      map[string]any
	StartedAt    *time.Time
	CompletedAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Terminal reports whether the job can no longer change on its own.
func (j *ProcessingJob) Terminal() bool {
	return j.Status == JobStatusCompleted ||
		j.Status == JobStatusFailed ||
		j.Status == JobStatusCancelled
}

// PayloadString reads a string field from the job payload.
func (j *ProcessingJob) PayloadString(key string) string {
	value, _ := j.Payload[key].(string)

	return value
}

// PayloadNumber reads a numeric field from the job payload.
func (j *ProcessingJob) PayloadNumber(key string) float64 {
	value, ok := j.Payload[key].(float64)
	if !ok {
		return 0
	}

	return value
}

// DecodePayload converts a raw JSONB value into a payload map.
func DecodePayload(raw []byte) map[string]any {
	payload := map[string]any{}

	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &payload)
	}

	return payload
}
