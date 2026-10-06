package model

import (
	"time"

	"github.com/google/uuid"
)

type MediaJobType string

const (
	MediaJobVideoTranscode    MediaJobType = "video_transcode"
	MediaJobThumbnailGenerate MediaJobType = "thumbnail_generate"
	MediaJobVideoProbe        MediaJobType = "video_probe"
)

type MediaJobStatus string

const (
	MediaJobQueued    MediaJobStatus = "queued"
	MediaJobRunning   MediaJobStatus = "running"
	MediaJobCompleted MediaJobStatus = "completed"
	MediaJobFailed    MediaJobStatus = "failed"
	MediaJobCancelled MediaJobStatus = "cancelled"
)

// MediaJob tracks one unit of media-engine work. VideoID is a
// cross-service identifier (plain UUID, never a foreign key). This
// service orchestrates; FFmpeg execution lives in the external
// media engine.
type MediaJob struct {
	ID                 uuid.UUID
	VideoID            uuid.UUID
	JobType            MediaJobType
	Status             MediaJobStatus
	SourceObjectKey    *string
	OutputManifestURL  *string
	OutputThumbnailURL *string
	DurationSeconds    *int64
	Width              *int32
	Height             *int32
	Progress           int32
	AttemptCount       int32
	IdempotencyKey     *string
	EngineJobID        *string
	ErrorCode          *string
	ErrorMessage       *string
	QueuedAt           time.Time
	StartedAt          *time.Time
	CompletedAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// Terminal reports whether the job will never change state again
// (except via an explicit retry, which creates a NEW job).
func (j *MediaJob) Terminal() bool {
	return j.Status == MediaJobCompleted ||
		j.Status == MediaJobFailed ||
		j.Status == MediaJobCancelled
}
