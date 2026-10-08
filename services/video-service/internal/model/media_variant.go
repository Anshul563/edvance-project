package model

import (
	"time"

	"github.com/google/uuid"
)

// VariantQuality is a rendition label. An upload only produces the
// renditions the engine decided to build; every quality is optional.
type VariantQuality string

const (
	Quality144p  VariantQuality = "144p"
	Quality240p  VariantQuality = "240p"
	Quality360p  VariantQuality = "360p"
	Quality480p  VariantQuality = "480p"
	Quality720p  VariantQuality = "720p"
	Quality1080p VariantQuality = "1080p"
	Quality1440p VariantQuality = "1440p"
	Quality2160p VariantQuality = "2160p"
)

// ValidVariantQualities is the accepted set for engine callbacks.
var ValidVariantQualities = map[VariantQuality]bool{
	Quality144p:  true,
	Quality240p:  true,
	Quality360p:  true,
	Quality480p:  true,
	Quality720p:  true,
	Quality1080p: true,
	Quality1440p: true,
	Quality2160p: true,
}

// MediaVariant is one rendition of a media asset.
type MediaVariant struct {
	ID            uuid.UUID
	MediaAssetID  uuid.UUID
	Quality       VariantQuality
	Width         int
	Height        int
	Bitrate       *int64
	Codec         *string
	Container     *string
	StorageKey    string
	PlaybackURL   *string
	FileSizeBytes *int64
	CreatedAt     time.Time
}
