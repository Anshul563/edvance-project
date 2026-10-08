package model

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// AssetStatus is the upload/processing state machine for a media asset:
//
//	created -> uploading -> uploaded -> processing -> ready
//	                              processing -> failed
//	                              failed     -> processing (retry)
//	*        -> deleted
//
// Transitions are explicit; nothing changes status without going
// through CanTransition first, and the repository re-checks the
// expected FROM status so concurrent writers cannot race.
type AssetStatus string

const (
	AssetStatusCreated    AssetStatus = "created"
	AssetStatusUploading  AssetStatus = "uploading"
	AssetStatusUploaded   AssetStatus = "uploaded"
	AssetStatusProcessing AssetStatus = "processing"
	AssetStatusReady      AssetStatus = "ready"
	AssetStatusFailed     AssetStatus = "failed"
	AssetStatusDeleted    AssetStatus = "deleted"
)

// AssetType classifies the object. Video is the primary focus; the
// column exists so audio/image uploads do not need a schema change.
type AssetType string

const (
	AssetTypeVideo AssetType = "video"
	AssetTypeAudio AssetType = "audio"
	AssetTypeImage AssetType = "image"
)

// ErrInvalidTransition is returned for any move the state machine does
// not allow (for example created -> ready).
var ErrInvalidTransition = errors.New("invalid status transition")

// assetTransitions is the single source of truth for legal moves.
var assetTransitions = map[AssetStatus][]AssetStatus{
	AssetStatusCreated:    {AssetStatusUploading, AssetStatusDeleted},
	AssetStatusUploading:  {AssetStatusUploaded, AssetStatusDeleted},
	AssetStatusUploaded:   {AssetStatusProcessing, AssetStatusDeleted},
	AssetStatusProcessing: {AssetStatusReady, AssetStatusFailed, AssetStatusDeleted},
	AssetStatusReady:      {AssetStatusDeleted},
	AssetStatusFailed:     {AssetStatusProcessing, AssetStatusDeleted},
	AssetStatusDeleted:    {},
}

// CanTransition reports whether from -> to is a legal state change.
func CanTransition(from AssetStatus, to AssetStatus) bool {
	for _, allowed := range assetTransitions[from] {
		if allowed == to {
			return true
		}
	}

	return false
}

// MediaAsset is one uploaded object and its processing outcome.
type MediaAsset struct {
	ID               uuid.UUID
	OwnerID          uuid.UUID
	Type             AssetType
	OriginalFilename string
	MIMEType         string
	FileSizeBytes    int64
	StorageKey       string
	Status           AssetStatus
	DurationSeconds  *int64
	Width            *int
	Height           *int
	FrameRate        *float64
	Codec            *string
	Bitrate          *int64
	Container        *string
	PlaybackURL      *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ProcessedAt      *time.Time
}

// Playable reports whether the asset is ready for playback.
func (a *MediaAsset) Playable() bool {
	return a.Status == AssetStatusReady
}

// Deleted reports whether the asset has been removed.
func (a *MediaAsset) Deleted() bool {
	return a.Status == AssetStatusDeleted
}

// Transition validates and applies a state change in memory. Callers
// persist it through the repository, which re-checks the FROM status.
func (a *MediaAsset) Transition(to AssetStatus) error {
	if !CanTransition(a.Status, to) {
		return ErrInvalidTransition
	}

	a.Status = to

	return nil
}

// ReadyMetadata is the engine-provided description persisted when
// processing completes. None of it ever comes from a client request.
type ReadyMetadata struct {
	DurationSeconds *int64
	Width           *int
	Height          *int
	FrameRate       *float64
	Codec           *string
	Bitrate         *int64
	Container       *string
	PlaybackURL     *string
}
