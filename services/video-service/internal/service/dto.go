package service

// InitUploadParams is the only client-controlled input in the upload
// pipeline. Everything else (keys, statuses, URLs) is derived server-side.
type InitUploadParams struct {
	Filename  string `json:"filename"`
	MIMEType  string `json:"mimeType"`
	SizeBytes int64  `json:"size"`
}

// InitUploadResult hands the client a presigned PUT URL. The signed
// content type binds MIME type enforcement at the store itself.
type InitUploadResult struct {
	UploadURL   string    `json:"uploadUrl"`
	ContentType string    `json:"contentType"`
	Asset       AssetView `json:"asset"`
}

// CallbackRequest is the media engine's report for one job. The engine
// repeats the jobId and mediaAssetId it was given; the payload beyond
// that is metadata and rendered objects — none of it is trusted for
// ownership or state, only for rendition details.
type CallbackRequest struct {
	JobID        string            `json:"jobId"`
	MediaAssetID string            `json:"mediaAssetId"`
	Status       string            `json:"status"` // "succeeded" | "failed"
	Error        *EngineError      `json:"error,omitempty"`
	Metadata     *EngineMetadata   `json:"metadata,omitempty"`
	Variants     []VariantOutput   `json:"variants,omitempty"`
	Thumbnails   []ThumbnailOutput `json:"thumbnails,omitempty"`
	Captions     []CaptionOutput   `json:"captions,omitempty"`
}

// EngineError carries a machine code, a human message and whether a
// retried run could plausibly succeed.
type EngineError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retriable bool   `json:"retriable"`
}

// EngineMetadata is the probed description of the source object.
type EngineMetadata struct {
	DurationSeconds *int64   `json:"durationSeconds,omitempty"`
	Width           *int     `json:"width,omitempty"`
	Height          *int     `json:"height,omitempty"`
	FrameRate       *float64 `json:"frameRate,omitempty"`
	Codec           *string  `json:"codec,omitempty"`
	Bitrate         *int64   `json:"bitrate,omitempty"`
	Container       *string  `json:"container,omitempty"`
}

// VariantOutput is one rendition produced by the engine.
type VariantOutput struct {
	Quality       string  `json:"quality"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	Bitrate       *int64  `json:"bitrate,omitempty"`
	Codec         *string `json:"codec,omitempty"`
	Container     *string `json:"container,omitempty"`
	StorageKey    string  `json:"storageKey"`
	PlaybackURL   *string `json:"playbackUrl,omitempty"`
	FileSizeBytes *int64  `json:"fileSizeBytes,omitempty"`
}

// ThumbnailOutput is one still frame extracted by the engine. At most
// one entry may declare IsPrimary; the service enforces that.
type ThumbnailOutput struct {
	StorageKey       string   `json:"storageKey"`
	URL              *string  `json:"url,omitempty"`
	Width            *int     `json:"width,omitempty"`
	Height           *int     `json:"height,omitempty"`
	TimestampSeconds *float64 `json:"timestampSeconds,omitempty"`
	IsPrimary        bool     `json:"isPrimary"`
}

// CaptionOutput is one subtitle track. At most one entry may declare
// IsDefault.
type CaptionOutput struct {
	Language   string  `json:"language"`
	Label      string  `json:"label"`
	Format     string  `json:"format"`
	StorageKey string  `json:"storageKey"`
	URL        *string `json:"url,omitempty"`
	IsDefault  bool    `json:"isDefault"`
}
