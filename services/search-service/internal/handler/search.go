package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/search-service/internal/config"
	"github.com/Anshul563/edvance-project/services/search-service/internal/model"
	"github.com/Anshul563/edvance-project/services/search-service/internal/repository"
)

// SearchHandler serves the public and internal search endpoints.
type SearchHandler struct {
	repo repository.SearchRepository
	cfg  config.Config
}

// NewSearchHandler builds a SearchHandler.
func NewSearchHandler(repo repository.SearchRepository, cfg config.Config) *SearchHandler {
	return &SearchHandler{repo: repo, cfg: cfg}
}

// Health reports liveness.
func (h *SearchHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "search-service"})
}

// Ready reports readiness, including database reachability.
func (h *SearchHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	if _, err := h.repo.Count(ctx, ""); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "service": "search-service"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": "search-service"})
}

// UnifiedSearch handles GET /api/v1/search.
func (h *SearchHandler) UnifiedSearch(w http.ResponseWriter, r *http.Request) {
	params, err := parseSearchParams(r, h.cfg)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	items, total, err := h.repo.Search(r.Context(), params.Query, params.opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}

	// Best-effort event tracking. A tracking failure must never
	// fail the search response.
	go func() {
		bgCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = h.repo.RecordSearchEvent(bgCtx, normalizeQuery(params.Query), len(items))
	}()

	page := params.opts.Offset/params.opts.Limit + 1
	hasNext := params.opts.Offset+len(items) < int(total)

	response := model.SearchResponse{
		Items:   toSearchItems(items),
		Page:    page,
		Limit:   params.opts.Limit,
		Total:   total,
		HasNext: hasNext,
	}

	writeJSON(w, http.StatusOK, response)
}

// SuggestionSearch handles GET /api/v1/search/suggestions.
func (h *SearchHandler) SuggestionSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeError(w, http.StatusBadRequest, "q is required")
		return
	}
	if len(q) > h.cfg.Search.MaxSuggestLength {
		writeError(w, http.StatusBadRequest, "q exceeds maximum length")
		return
	}

	docs, err := h.repo.Suggest(r.Context(), q, h.cfg.Search.MaxSuggestResults)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "suggestions failed")
		return
	}

	suggestions := make([]model.SuggestionItem, 0, len(docs))
	for _, doc := range docs {
		suggestions = append(suggestions, model.SuggestionItem{
			ID:    doc.ID.String(),
			Type:  doc.SourceType,
			Title: doc.Title,
		})
	}

	writeJSON(w, http.StatusOK, model.SuggestionResponse{Suggestions: suggestions})
}

// TrendingSearches handles GET /api/v1/search/trending.
func (h *SearchHandler) TrendingSearches(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.Trending(r.Context(), h.cfg.Search.TrendingLimit)
	if err != nil {
		// Trending is derived from recorded events; if none exist
		// or the query fails, return an empty result rather than
		// fabricating data.
		writeJSON(w, http.StatusOK, model.TrendingResponse{Queries: []model.TrendingItem{}})
		return
	}

	writeJSON(w, http.StatusOK, model.TrendingResponse{Queries: items})
}

// InternalUpsertDocument handles PUT /internal/v1/search/documents/:type/:id.
func (h *SearchHandler) InternalUpsertDocument(w http.ResponseWriter, r *http.Request) {
	sourceType := r.PathValue("type")
	sourceIDStr := r.PathValue("id")

	if !model.ValidSourceType(sourceType) {
		writeError(w, http.StatusBadRequest, "invalid source type")
		return
	}

	sourceID, err := uuid.Parse(sourceIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid source id")
		return
	}

	var req model.UpsertDocumentRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := req.Validate(sourceType); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	doc := req.ToDocument(sourceType, sourceID)
	doc.ID = uuid.New()

	if err := h.repo.Upsert(r.Context(), doc); err != nil {
		writeError(w, http.StatusInternalServerError, "indexing failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "indexed", "sourceType": sourceType, "sourceId": sourceID.String()})
}

// InternalDeleteDocument handles DELETE /internal/v1/search/documents/:type/:id.
func (h *SearchHandler) InternalDeleteDocument(w http.ResponseWriter, r *http.Request) {
	sourceType := r.PathValue("type")
	sourceIDStr := r.PathValue("id")

	if !model.ValidSourceType(sourceType) {
		writeError(w, http.StatusBadRequest, "invalid source type")
		return
	}

	sourceID, err := uuid.Parse(sourceIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid source id")
		return
	}

	if err := h.repo.Delete(r.Context(), sourceType, sourceID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "document not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "deletion failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted", "sourceType": sourceType, "sourceId": sourceID.String()})
}

// InternalReindex handles POST /internal/v1/search/reindex.
// It is a bounded bulk upsert: source services that do not emit
// indexing events can rebuild a slice of their index by posting
// up to MaxReindexBatch documents.
func (h *SearchHandler) InternalReindex(w http.ResponseWriter, r *http.Request) {
	var req model.ReindexRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len(req.Documents) == 0 {
		writeError(w, http.StatusBadRequest, "documents is required")
		return
	}
	if len(req.Documents) > h.cfg.Search.MaxReindexBatch {
		writeError(w, http.StatusBadRequest, "document count exceeds maximum batch size")
		return
	}

	indexed := 0
	for i := range req.Documents {
		entry := &req.Documents[i]

		if !model.ValidSourceType(entry.SourceType) {
			writeError(w, http.StatusBadRequest, "invalid source type at index "+strconv.Itoa(i))
			return
		}

		sourceID, err := uuid.Parse(entry.SourceID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid source id at index "+strconv.Itoa(i))
			return
		}

		if err := entry.UpsertDocumentRequest.Validate(entry.SourceType); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		doc := entry.UpsertDocumentRequest.ToDocument(entry.SourceType, sourceID)
		doc.ID = uuid.New()

		if err := h.repo.Upsert(r.Context(), doc); err != nil {
			writeError(w, http.StatusInternalServerError, "reindex failed at index "+strconv.Itoa(i))
			return
		}
		indexed++
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "reindexed", "count": indexed})
}

// searchParams holds parsed and validated search parameters.
type searchParams struct {
	Query string
	opts  repository.SearchOptions
}

// parseSearchParams validates query parameters against the
// configured limits and returns the search options.
func parseSearchParams(r *http.Request, cfg config.Config) (*searchParams, error) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		return nil, errors.New("q is required")
	}
	if len(q) > cfg.Search.MaxQueryLength {
		return nil, errors.New("q exceeds maximum length")
	}

	sourceType := strings.TrimSpace(r.URL.Query().Get("type"))
	if sourceType == "" {
		sourceType = "all"
	}
	if sourceType != "all" && !model.ValidSourceType(sourceType) {
		return nil, errors.New("invalid type")
	}

	sort := strings.TrimSpace(r.URL.Query().Get("sort"))
	if sort == "" {
		sort = "relevance"
	}
	switch sort {
	case "relevance", "newest":
	default:
		return nil, errors.New("invalid sort")
	}

	page := 1
	if v := r.URL.Query().Get("page"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 1 {
			return nil, errors.New("invalid page")
		}
		page = parsed
	}

	limit := cfg.Pagination.DefaultPageSize
	if v := r.URL.Query().Get("limit"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 1 {
			return nil, errors.New("invalid limit")
		}
		limit = parsed
	}
	if limit > cfg.Pagination.MaxPageSize {
		limit = cfg.Pagination.MaxPageSize
	}

	opts := repository.SearchOptions{
		Type:     sourceType,
		Category: strings.TrimSpace(r.URL.Query().Get("category")),
		Language: strings.TrimSpace(r.URL.Query().Get("language")),
		Sort:     sort,
		Limit:    limit,
		Offset:   (page - 1) * limit,
	}

	return &searchParams{Query: q, opts: opts}, nil
}

// toSearchItems maps repository results to the public DTO,
// excluding owner identity and internal fields.
func toSearchItems(items []*model.SearchResultItem) []model.SearchItem {
	result := make([]model.SearchItem, 0, len(items))
	for _, item := range items {
		doc := item.Document
		result = append(result, model.SearchItem{
			ID:           doc.ID.String(),
			Type:         doc.SourceType,
			Title:        doc.Title,
			Description:  doc.Description,
			ThumbnailURL: model.StringPointer(doc.ThumbnailURL),
			CanonicalURL: model.StringPointer(doc.CanonicalURL),
			Category:     model.StringPointer(doc.Category),
			PublishedAt:  model.FormatTimeRFC3339(doc.PublishedAt),
			Score:        item.Score,
		})
	}
	return result
}

// normalizeQuery lowercases and collapses whitespace for event
// storage so trending groups equivalent queries without storing
// raw user input.
func normalizeQuery(q string) string {
	return strings.ToLower(strings.Join(strings.Fields(q), " "))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
