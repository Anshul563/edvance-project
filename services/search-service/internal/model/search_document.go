package model

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Source types that may be indexed for search.
const (
	SourceCourse = "course"
	SourceVideo  = "video"
	SourceShort  = "short"
	SourcePost   = "post"
	SourceCreator = "creator"
)

// Visibility values stored in the index. Only public documents are
// eligible for public search results.
const (
	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
	VisibilityDraft   = "draft"
	VisibilityUnlisted = "unlisted"
)

// Limits for indexed document fields. These bound the internal
// indexing API so a misbehaving producer cannot bloat the index.
const (
	MaxTitleLength       = 300
	MaxDescriptionLength = 2000
	MaxBodyLength        = 100000
	MaxURLLength         = 2048
)

// SearchDocument is the unified search index entry. The source
// service remains the authoritative owner of the content; this
// document is a denormalized copy used only for discovery.
type SearchDocument struct {
	ID           uuid.UUID
	SourceType   string
	SourceID     uuid.UUID
	OwnerID      uuid.NullUUID
	Title        string
	Description  string
	Body         string
	ThumbnailURL string
	CanonicalURL string
	Visibility   string
	Language     string
	Category     string
	Tags         map[string]any
	Metadata     map[string]any
	PublishedAt  *time.Time
	IndexedAt    time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SearchResultItem pairs a document with its relevance score for a
// single search. Score is populated only by search queries.
type SearchResultItem struct {
	Document *SearchDocument
	Score    float32
}

// UpsertDocumentRequest is the request body for the internal
// indexing endpoint. SourceType and SourceID come from the URL
// path, not the body, so a producer cannot forge another
// service's documents.
type UpsertDocumentRequest struct {
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	Body         string         `json:"body"`
	ThumbnailURL string         `json:"thumbnailUrl"`
	CanonicalURL string         `json:"canonicalUrl"`
	Visibility   string         `json:"visibility"`
	Language     string         `json:"language"`
	Category     string         `json:"category"`
	Tags         map[string]any `json:"tags"`
	Metadata     map[string]any `json:"metadata"`
	PublishedAt  *time.Time     `json:"publishedAt"`
}

// Validate checks the document fields for the internal indexing
// endpoint. It never trusts caller-supplied visibility beyond the
// allowed set and enforces length bounds.
func (r *UpsertDocumentRequest) Validate(sourceType string) error {
	if !ValidSourceType(sourceType) {
		return errors.New("invalid source type")
	}

	title := strings.TrimSpace(r.Title)
	if title == "" {
		return errors.New("title is required")
	}
	if len(title) > MaxTitleLength {
		return errors.New("title exceeds maximum length")
	}

	if len(r.Description) > MaxDescriptionLength {
		return errors.New("description exceeds maximum length")
	}
	if len(r.Body) > MaxBodyLength {
		return errors.New("body exceeds maximum length")
	}

	if r.Visibility != "" && !ValidVisibility(r.Visibility) {
		return errors.New("invalid visibility")
	}

	if len(r.ThumbnailURL) > MaxURLLength {
		return errors.New("thumbnailUrl exceeds maximum length")
	}
	if len(r.CanonicalURL) > MaxURLLength {
		return errors.New("canonicalUrl exceeds maximum length")
	}

	if len(r.Language) > 8 {
		return errors.New("language exceeds maximum length")
	}
	if len(r.Category) > 64 {
		return errors.New("category exceeds maximum length")
	}

	return nil
}

// ToDocument converts a validated request into a SearchDocument for
// the given source type and id. Visibility defaults to public.
func (r *UpsertDocumentRequest) ToDocument(sourceType string, sourceID uuid.UUID) *SearchDocument {
	visibility := r.Visibility
	if visibility == "" {
		visibility = VisibilityPublic
	}

	title := strings.TrimSpace(r.Title)

	return &SearchDocument{
		SourceType:   sourceType,
		SourceID:     sourceID,
		Title:        title,
		Description:  r.Description,
		Body:         r.Body,
		ThumbnailURL: r.ThumbnailURL,
		CanonicalURL: r.CanonicalURL,
		Visibility:   visibility,
		Language:     r.Language,
		Category:     r.Category,
		Tags:         r.Tags,
		Metadata:     r.Metadata,
		PublishedAt:  r.PublishedAt,
	}
}

// ValidSourceType reports whether t is an indexable source type.
func ValidSourceType(t string) bool {
	switch t {
	case SourceCourse, SourceVideo, SourceShort, SourcePost, SourceCreator:
		return true
	default:
		return false
	}
}

// ValidVisibility reports whether v is a stored visibility value.
func ValidVisibility(v string) bool {
	switch v {
	case VisibilityPublic, VisibilityPrivate, VisibilityDraft, VisibilityUnlisted:
		return true
	default:
		return false
	}
}

// IsPubliclySearchable reports whether a document may appear in
// public search results. Only public documents are eligible;
// private, draft, unlisted, and any other value are excluded.
func (d *SearchDocument) IsPubliclySearchable() bool {
	return d.Visibility == VisibilityPublic
}

// SearchItem is the public API representation of a search result.
// It deliberately excludes owner identity, body, tags, metadata,
// visibility, language, and the search vector.
type SearchItem struct {
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	ThumbnailURL *string `json:"thumbnailUrl"`
	CanonicalURL *string `json:"canonicalUrl"`
	Category     *string `json:"category"`
	PublishedAt  *string `json:"publishedAt"`
	Score        float32 `json:"score"`
}

// SearchResponse is the public search response envelope.
type SearchResponse struct {
	Items   []SearchItem `json:"items"`
	Page    int          `json:"page"`
	Limit   int          `json:"limit"`
	Total   int64        `json:"total"`
	HasNext bool         `json:"hasNext"`
}

// SuggestionItem is a single search suggestion.
type SuggestionItem struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

// SuggestionResponse is the suggestions response envelope.
type SuggestionResponse struct {
	Suggestions []SuggestionItem `json:"suggestions"`
}

// TrendingItem is a trending search query with its event count.
type TrendingItem struct {
	Query string `json:"query"`
	Count int64  `json:"count"`
}

// TrendingResponse is the trending response envelope.
type TrendingResponse struct {
	Queries []TrendingItem `json:"queries"`
}

// FormatTimeRFC3339 renders t as an RFC3339 string, or nil when t
// is nil. Used to keep nullable timestamps nullable in responses.
func FormatTimeRFC3339(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(time.RFC3339)
	return &s
}

// StringPointer returns nil for an empty string, otherwise a
// pointer to the trimmed value. Used to keep optional string
// fields null rather than empty in responses.
func StringPointer(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// ReindexRequest is the bounded bulk indexing request used by
// the internal reindex endpoint. It lets a source service
// rebuild a slice of the index when it does not emit
// indexing events.
type ReindexRequest struct {
	Documents []ReindexDocument `json:"documents"`
}

// ReindexDocument is a single document within a reindex batch.
// SourceType and SourceID identify the source object; the
// remaining fields are the searchable copy.
type ReindexDocument struct {
	SourceType string `json:"sourceType"`
	SourceID   string `json:"sourceId"`

	UpsertDocumentRequest
}
