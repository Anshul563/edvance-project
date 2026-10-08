package service

import (
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
)

// AssetView is the JSON shape of a media asset plus its renditions. It
// is assembled from trusted server state and engine callbacks only —
// never from client-supplied metadata.
type AssetView struct {
	ID               uuid.UUID         `json:"id"`
	OwnerID          uuid.UUID         `json:"ownerId"`
	Type             model.AssetType   `json:"type"`
	OriginalFilename string            `json:"originalFilename"`
	MIMEType         string            `json:"mimeType"`
	FileSizeBytes    int64             `json:"fileSizeBytes"`
	StorageKey       string            `json:"storageKey"`
	Status           model.AssetStatus `json:"status"`
	DurationSeconds  *int64            `json:"durationSeconds,omitempty"`
	Width            *int              `json:"width,omitempty"`
	Height           *int              `json:"height,omitempty"`
	FrameRate        *float64          `json:"frameRate,omitempty"`
	Codec            *string           `json:"codec,omitempty"`
	Bitrate          *int64            `json:"bitrate,omitempty"`
	Container        *string           `json:"container,omitempty"`
	PlaybackURL      *string           `json:"playbackUrl,omitempty"`
	CreatedAt        time.Time         `json:"createdAt"`
	UpdatedAt        time.Time         `json:"updatedAt"`
	ProcessedAt      *time.Time        `json:"processedAt,omitempty"`
	Variants         []VariantView     `json:"variants,omitempty"`
	Thumbnails       []ThumbnailView   `json:"thumbnails,omitempty"`
	Captions         []CaptionView     `json:"captions,omitempty"`
}

// VariantView is a rendition.
type VariantView struct {
	ID            uuid.UUID            `json:"id"`
	Quality       model.VariantQuality `json:"quality"`
	Width         int                  `json:"width"`
	Height        int                  `json:"height"`
	Bitrate       *int64               `json:"bitrate,omitempty"`
	Codec         *string              `json:"codec,omitempty"`
	Container     *string              `json:"container,omitempty"`
	StorageKey    string               `json:"storageKey"`
	PlaybackURL   *string              `json:"playbackUrl,omitempty"`
	FileSizeBytes *int64               `json:"fileSizeBytes,omitempty"`
	CreatedAt     time.Time            `json:"createdAt"`
}

// ThumbnailView is a still frame.
type ThumbnailView struct {
	ID               uuid.UUID `json:"id"`
	StorageKey       string    `json:"storageKey"`
	URL              *string   `json:"url,omitempty"`
	Width            *int      `json:"width,omitempty"`
	Height           *int      `json:"height,omitempty"`
	TimestampSeconds *float64  `json:"timestampSeconds,omitempty"`
	IsPrimary        bool      `json:"isPrimary"`
}

// CaptionView is a subtitle track.
type CaptionView struct {
	ID         uuid.UUID           `json:"id"`
	Language   string              `json:"language"`
	Label      string              `json:"label"`
	Format     model.CaptionFormat `json:"format"`
	StorageKey string              `json:"storageKey"`
	URL        *string             `json:"url,omitempty"`
	IsDefault  bool                `json:"isDefault"`
}

func (s *MediaService) assetView(
	asset *model.MediaAsset,
	variants []*model.MediaVariant,
	thumbnails []*model.Thumbnail,
	captions []*model.Caption,
) AssetView {
	view := AssetView{
		ID:               asset.ID,
		OwnerID:          asset.OwnerID,
		Type:             asset.Type,
		OriginalFilename: asset.OriginalFilename,
		MIMEType:         asset.MIMEType,
		FileSizeBytes:    asset.FileSizeBytes,
		StorageKey:       asset.StorageKey,
		Status:           asset.Status,
		DurationSeconds:  asset.DurationSeconds,
		Width:            asset.Width,
		Height:           asset.Height,
		FrameRate:        asset.FrameRate,
		Codec:            asset.Codec,
		Bitrate:          asset.Bitrate,
		Container:        asset.Container,
		PlaybackURL:      asset.PlaybackURL,
		CreatedAt:        asset.CreatedAt,
		UpdatedAt:        asset.UpdatedAt,
		ProcessedAt:      asset.ProcessedAt,
		Variants:         make([]VariantView, 0, len(variants)),
		Thumbnails:       make([]ThumbnailView, 0, len(thumbnails)),
		Captions:         make([]CaptionView, 0, len(captions)),
	}

	for _, variant := range variants {
		view.Variants = append(view.Variants, VariantView{
			ID:            variant.ID,
			Quality:       variant.Quality,
			Width:         variant.Width,
			Height:        variant.Height,
			Bitrate:       variant.Bitrate,
			Codec:         variant.Codec,
			Container:     variant.Container,
			StorageKey:    variant.StorageKey,
			PlaybackURL:   variant.PlaybackURL,
			FileSizeBytes: variant.FileSizeBytes,
			CreatedAt:     variant.CreatedAt,
		})
	}

	for _, thumbnail := range thumbnails {
		view.Thumbnails = append(view.Thumbnails, ThumbnailView{
			ID:               thumbnail.ID,
			StorageKey:       thumbnail.StorageKey,
			URL:              thumbnail.URL,
			Width:            thumbnail.Width,
			Height:           thumbnail.Height,
			TimestampSeconds: thumbnail.TimestampSeconds,
			IsPrimary:        thumbnail.IsPrimary,
		})
	}

	for _, caption := range captions {
		view.Captions = append(view.Captions, CaptionView{
			ID:         caption.ID,
			Language:   caption.Language,
			Label:      caption.Label,
			Format:     caption.Format,
			StorageKey: caption.StorageKey,
			URL:        caption.URL,
			IsDefault:  caption.IsDefault,
		})
	}

	return view
}
