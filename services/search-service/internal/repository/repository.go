package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/search-service/internal/model"
)

// ErrNotFound is returned when a document to delete does not
// exist in the index.
var ErrNotFound = errors.New("search document not found")

// SearchRepository is the port the search service uses to read and
// write its own search index. The implementation is PostgreSQL
// full-text search today; the interface keeps an OpenSearch or
// other backend swappable without touching handlers.
type SearchRepository interface {
	// Indexer: idempotent upsert and delete by source identity.
	Upsert(ctx context.Context, doc *model.SearchDocument) error
	Delete(ctx context.Context, sourceType string, sourceID uuid.UUID) error

	// Search performs relevance-ranked full-text search with
	// filters, sorting, and pagination.
	Search(ctx context.Context, query string, opts SearchOptions) ([]*model.SearchResultItem, int64, error)

	// Suggest returns a small bounded set of titles or creator
	// names matching a prefix-style query.
	Suggest(ctx context.Context, query string, limit int) ([]*model.SearchDocument, error)

	// RecordSearchEvent stores a normalized query for trending.
	// Best effort: a failure must never fail the search.
	RecordSearchEvent(ctx context.Context, normalizedQuery string, resultCount int) error

	// Trending returns the most frequent recent normalized queries.
	Trending(ctx context.Context, limit int) ([]model.TrendingItem, error)

	// Count returns the number of indexed documents, optionally
	// filtered by source type.
	Count(ctx context.Context, sourceType string) (int64, error)
}

// SearchOptions carries the filters, sort order, and pagination for
// a search. Filters that are empty strings are treated as
// "no filter".
type SearchOptions struct {
	Type     string
	Category string
	Language string
	Sort     string
	Limit    int
	Offset   int
}
