package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/search-service/internal/config"
	"github.com/Anshul563/edvance-project/services/search-service/internal/handler"
	"github.com/Anshul563/edvance-project/services/search-service/internal/model"
	"github.com/Anshul563/edvance-project/services/search-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/search-service/internal/router"
)

// stubRepo implements repository.SearchRepository with canned
// results and records what the handler passed through.
type stubRepo struct {
	searchItems []*model.SearchResultItem
	searchTotal int64
	searchErr   error

	suggestDocs []*model.SearchDocument
	suggestErr  error

	trendingItems []model.TrendingItem
	trendingErr   error

	upsertErr error
	deleteErr error
	countVal  int64
	countErr  error

	gotSearchQuery string
	gotSearchOpts  repository.SearchOptions
	gotUpsertDoc   *model.SearchDocument
	gotDeleteType  string
	gotDeleteID    uuid.UUID
	recordedEvents []string
}

func (s *stubRepo) Upsert(_ context.Context, doc *model.SearchDocument) error {
	s.gotUpsertDoc = doc
	return s.upsertErr
}

func (s *stubRepo) Delete(_ context.Context, sourceType string, sourceID uuid.UUID) error {
	s.gotDeleteType = sourceType
	s.gotDeleteID = sourceID
	return s.deleteErr
}

func (s *stubRepo) Search(
	_ context.Context,
	query string,
	opts repository.SearchOptions,
) ([]*model.SearchResultItem, int64, error) {
	s.gotSearchQuery = query
	s.gotSearchOpts = opts
	if s.searchErr != nil {
		return nil, 0, s.searchErr
	}
	return s.searchItems, s.searchTotal, nil
}

func (s *stubRepo) Suggest(
	_ context.Context,
	query string,
	limit int,
) ([]*model.SearchDocument, error) {
	if s.suggestErr != nil {
		return nil, s.suggestErr
	}
	return s.suggestDocs, nil
}

func (s *stubRepo) RecordSearchEvent(
	_ context.Context,
	normalizedQuery string,
	resultCount int,
) error {
	s.recordedEvents = append(s.recordedEvents, normalizedQuery)
	return nil
}

func (s *stubRepo) Trending(_ context.Context, limit int) ([]model.TrendingItem, error) {
	if s.trendingErr != nil {
		return nil, s.trendingErr
	}
	return s.trendingItems, nil
}

func (s *stubRepo) Count(_ context.Context, sourceType string) (int64, error) {
	return s.countVal, s.countErr
}

func testConfig() config.Config {
	return config.Config{
		AppEnv: "test",
		Port:   8091,
		Pagination: config.PaginationConfig{
			DefaultPageSize: 20,
			MaxPageSize:     50,
		},
		Search: config.SearchConfig{
			MaxQueryLength:    200,
			MaxSuggestLength:  50,
			MaxSuggestResults: 10,
			MaxReindexBatch:   500,
			TrendingWindow:    24 * time.Hour,
			TrendingLimit:     10,
		},
	}
}

// buildServer wires the stub repository into the real router
// so middleware, validation, and routing are all exercised.
func buildServer(t *testing.T, repo *stubRepo, cfg config.Config) http.Handler {
	t.Helper()

	handler := handler.NewSearchHandler(repo, cfg)
	return router.New(router.Handlers{Search: handler}, cfg)
}

func doRequest(
	t *testing.T,
	h http.Handler,
	method, target, body, internalToken string,
) *httptest.ResponseRecorder {
	t.Helper()

	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	if internalToken != "" {
		req.Header.Set("Authorization", "Bearer "+internalToken)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func TestUnifiedSearchValid(t *testing.T) {
	repo := &stubRepo{
		searchItems: []*model.SearchResultItem{
			{
				Document: &model.SearchDocument{
					ID:          uuid.New(),
					SourceType:  "course",
					Title:       "Go Backend Development",
					Description: "Build APIs with Go",
					Category:    "Development",
				},
				Score: 0.85,
			},
		},
		searchTotal: 1,
	}

	h := buildServer(t, repo, testConfig())
	rec := doRequest(t, h, http.MethodGet, "/?q=golang&type=course&sort=newest&page=1&limit=20", "", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if repo.gotSearchQuery != "golang" {
		t.Fatalf("expected query golang, got %q", repo.gotSearchQuery)
	}
	if repo.gotSearchOpts.Type != "course" {
		t.Fatalf("expected type course, got %q", repo.gotSearchOpts.Type)
	}
	if repo.gotSearchOpts.Sort != "newest" {
		t.Fatalf("expected sort newest, got %q", repo.gotSearchOpts.Sort)
	}
	if repo.gotSearchOpts.Limit != 20 {
		t.Fatalf("expected limit 20, got %d", repo.gotSearchOpts.Limit)
	}
	if repo.gotSearchOpts.Offset != 0 {
		t.Fatalf("expected offset 0, got %d", repo.gotSearchOpts.Offset)
	}

	var resp model.SearchResponse
	decodeJSON(t, rec, &resp)

	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(resp.Items))
	}
	if resp.Items[0].Type != "course" {
		t.Fatalf("expected type course, got %q", resp.Items[0].Type)
	}
	if resp.Items[0].Score != 0.85 {
		t.Fatalf("expected score 0.85, got %f", resp.Items[0].Score)
	}
	if resp.Total != 1 {
		t.Fatalf("expected total 1, got %d", resp.Total)
	}
	if resp.HasNext {
		t.Fatal("expected hasNext false")
	}
}

func TestUnifiedSearchEmptyQuery(t *testing.T) {
	repo := &stubRepo{}
	h := buildServer(t, repo, testConfig())

	rec := doRequest(t, h, http.MethodGet, "/?q=", "", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty query, got %d", rec.Code)
	}

	rec = doRequest(t, h, http.MethodGet, "/", "", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing query, got %d", rec.Code)
	}

	rec = doRequest(t, h, http.MethodGet, "/?q="+url.QueryEscape("   "), "", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for whitespace query, got %d", rec.Code)
	}
}

func TestUnifiedSearchLongQuery(t *testing.T) {
	repo := &stubRepo{}
	cfg := testConfig()
	h := buildServer(t, repo, cfg)

	long := strings.Repeat("a", cfg.Search.MaxQueryLength+1)
	rec := doRequest(t, h, http.MethodGet, "/?q="+long, "", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for long query, got %d", rec.Code)
	}
}

func TestUnifiedSearchInvalidTypeAndSort(t *testing.T) {
	repo := &stubRepo{}
	h := buildServer(t, repo, testConfig())

	rec := doRequest(t, h, http.MethodGet, "/?q=golang&type=bogus", "", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid type, got %d", rec.Code)
	}

	rec = doRequest(t, h, http.MethodGet, "/?q=golang&sort=bogus", "", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid sort, got %d", rec.Code)
	}
}

func TestUnifiedSearchPagination(t *testing.T) {
	repo := &stubRepo{searchTotal: 100}
	cfg := testConfig()
	h := buildServer(t, repo, cfg)

	rec := doRequest(t, h, http.MethodGet, "/?q=golang&page=2&limit=10", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if repo.gotSearchOpts.Offset != 10 {
		t.Fatalf("expected offset 10, got %d", repo.gotSearchOpts.Offset)
	}

	// Limit is capped at MaxPageSize.
	rec = doRequest(t, h, http.MethodGet, "/?q=golang&limit=1000", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if repo.gotSearchOpts.Limit != cfg.Pagination.MaxPageSize {
		t.Fatalf("expected limit capped at %d, got %d", cfg.Pagination.MaxPageSize, repo.gotSearchOpts.Limit)
	}

	// Invalid page and limit are rejected.
	rec = doRequest(t, h, http.MethodGet, "/?q=golang&page=0", "", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for page 0, got %d", rec.Code)
	}
	rec = doRequest(t, h, http.MethodGet, "/?q=golang&limit=0", "", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for limit 0, got %d", rec.Code)
	}
	rec = doRequest(t, h, http.MethodGet, "/?q=golang&page=abc", "", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-numeric page, got %d", rec.Code)
	}
}

func TestUnifiedSearchHasNext(t *testing.T) {
	repo := &stubRepo{
		searchItems: []*model.SearchResultItem{{Document: &model.SearchDocument{ID: uuid.New()}}},
		searchTotal: 100,
	}
	h := buildServer(t, repo, testConfig())

	rec := doRequest(t, h, http.MethodGet, "/?q=golang&limit=1", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp model.SearchResponse
	decodeJSON(t, rec, &resp)
	if !resp.HasNext {
		t.Fatal("expected hasNext true when more results exist")
	}
}

func TestSuggestions(t *testing.T) {
	repo := &stubRepo{
		suggestDocs: []*model.SearchDocument{
			{ID: uuid.New(), SourceType: "course", Title: "React Fundamentals"},
		},
	}
	h := buildServer(t, repo, testConfig())

	rec := doRequest(t, h, http.MethodGet, "/suggestions?q=rea", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp model.SuggestionResponse
	decodeJSON(t, rec, &resp)
	if len(resp.Suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(resp.Suggestions))
	}
	if resp.Suggestions[0].Title != "React Fundamentals" {
		t.Fatalf("unexpected title %q", resp.Suggestions[0].Title)
	}

	// Empty and over-long queries are rejected.
	rec = doRequest(t, h, http.MethodGet, "/suggestions?q=", "", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty suggestion query, got %d", rec.Code)
	}

	long := strings.Repeat("a", testConfig().Search.MaxSuggestLength+1)
	rec = doRequest(t, h, http.MethodGet, "/suggestions?q="+long, "", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for long suggestion query, got %d", rec.Code)
	}
}

func TestTrending(t *testing.T) {
	repo := &stubRepo{
		trendingItems: []model.TrendingItem{{Query: "golang", Count: 42}},
	}
	h := buildServer(t, repo, testConfig())

	rec := doRequest(t, h, http.MethodGet, "/trending", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp model.TrendingResponse
	decodeJSON(t, rec, &resp)
	if len(resp.Queries) != 1 {
		t.Fatalf("expected 1 trending query, got %d", len(resp.Queries))
	}
	if resp.Queries[0].Query != "golang" {
		t.Fatalf("unexpected trending query %q", resp.Queries[0].Query)
	}
}

func TestTrendingEmptyOnError(t *testing.T) {
	repo := &stubRepo{trendingErr: context.DeadlineExceeded}
	h := buildServer(t, repo, testConfig())

	rec := doRequest(t, h, http.MethodGet, "/trending", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 even when trending fails, got %d", rec.Code)
	}

	var resp model.TrendingResponse
	decodeJSON(t, rec, &resp)
	if len(resp.Queries) != 0 {
		t.Fatalf("expected empty trending on error, got %d", len(resp.Queries))
	}
}

func TestInternalUpsertDocument(t *testing.T) {
	repo := &stubRepo{}
	cfg := testConfig()
	cfg.Internal.ServiceToken = "test-token"
	h := buildServer(t, repo, cfg)

	sourceID := uuid.New()
	body := `{"title":"Go Backend Development","description":"Build APIs with Go","visibility":"public","category":"Development"}`
	target := "/internal/v1/search/documents/course/" + sourceID.String()

	rec := doRequest(t, h, http.MethodPut, target, body, "test-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if repo.gotUpsertDoc == nil {
		t.Fatal("expected upsert to be called")
	}
	if repo.gotUpsertDoc.SourceType != "course" {
		t.Fatalf("expected source type course, got %q", repo.gotUpsertDoc.SourceType)
	}
	if repo.gotUpsertDoc.SourceID != sourceID {
		t.Fatal("expected source id to match path")
	}
	if repo.gotUpsertDoc.Title != "Go Backend Development" {
		t.Fatalf("unexpected title %q", repo.gotUpsertDoc.Title)
	}
}

func TestInternalUpsertValidation(t *testing.T) {
	repo := &stubRepo{}
	cfg := testConfig()
	cfg.Internal.ServiceToken = "test-token"
	h := buildServer(t, repo, cfg)

	sourceID := uuid.New()
	target := "/internal/v1/search/documents/course/" + sourceID.String()

	// Missing title.
	rec := doRequest(t, h, http.MethodPut, target, `{"description":"no title"}`, "test-token")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing title, got %d", rec.Code)
	}

	// Invalid visibility.
	rec = doRequest(t, h, http.MethodPut, target, `{"title":"x","visibility":"bogus"}`, "test-token")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid visibility, got %d", rec.Code)
	}

	// Invalid source type in path.
	rec = doRequest(t, h, http.MethodPut, "/internal/v1/search/documents/bogus/"+sourceID.String(), `{"title":"x"}`, "test-token")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid source type, got %d", rec.Code)
	}

	// Invalid UUID in path.
	rec = doRequest(t, h, http.MethodPut, "/internal/v1/search/documents/course/not-a-uuid", `{"title":"x"}`, "test-token")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid uuid, got %d", rec.Code)
	}

	// Invalid JSON body.
	rec = doRequest(t, h, http.MethodPut, target, `{not json`, "test-token")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid body, got %d", rec.Code)
	}
}

func TestInternalRequiresAuth(t *testing.T) {
	repo := &stubRepo{}
	cfg := testConfig()
	cfg.Internal.ServiceToken = "test-token"
	h := buildServer(t, repo, cfg)

	sourceID := uuid.New()
	target := "/internal/v1/search/documents/course/" + sourceID.String()
	body := `{"title":"x"}`

	// No token.
	rec := doRequest(t, h, http.MethodPut, target, body, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", rec.Code)
	}

	// Wrong token.
	rec = doRequest(t, h, http.MethodPut, target, body, "wrong-token")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with wrong token, got %d", rec.Code)
	}

	// Delete without token.
	rec = doRequest(t, h, http.MethodDelete, target, "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for delete without token, got %d", rec.Code)
	}

	// Reindex without token.
	rec = doRequest(t, h, http.MethodPost, "/internal/v1/search/reindex", `{"documents":[]}`, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for reindex without token, got %d", rec.Code)
	}
}

func TestInternalDeleteDocument(t *testing.T) {
	repo := &stubRepo{}
	cfg := testConfig()
	cfg.Internal.ServiceToken = "test-token"
	h := buildServer(t, repo, cfg)

	sourceID := uuid.New()
	target := "/internal/v1/search/documents/video/" + sourceID.String()

	rec := doRequest(t, h, http.MethodDelete, target, "", "test-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if repo.gotDeleteType != "video" {
		t.Fatalf("expected delete type video, got %q", repo.gotDeleteType)
	}
	if repo.gotDeleteID != sourceID {
		t.Fatal("expected delete id to match path")
	}

	// Not found maps to 404.
	repo.deleteErr = repository.ErrNotFound
	rec = doRequest(t, h, http.MethodDelete, target, "", "test-token")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing document, got %d", rec.Code)
	}
}

func TestInternalReindex(t *testing.T) {
	repo := &stubRepo{}
	cfg := testConfig()
	cfg.Internal.ServiceToken = "test-token"
	h := buildServer(t, repo, cfg)

	id1 := uuid.New()
	id2 := uuid.New()
	body := `{"documents":[
		{"sourceType":"course","sourceId":"` + id1.String() + `","title":"Go I"},
		{"sourceType":"video","sourceId":"` + id2.String() + `","title":"Go II"}
	]}`

	rec := doRequest(t, h, http.MethodPost, "/internal/v1/search/reindex", body, "test-token")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	decodeJSON(t, rec, &resp)
	if resp["count"] != float64(2) {
		t.Fatalf("expected count 2, got %v", resp["count"])
	}
	if len(repo.gotUpsertDoc.SourceType) == 0 {
		t.Fatal("expected upserts to be called")
	}
}

func TestInternalReindexBatchLimit(t *testing.T) {
	repo := &stubRepo{}
	cfg := testConfig()
	cfg.Internal.ServiceToken = "test-token"
	cfg.Search.MaxReindexBatch = 2
	h := buildServer(t, repo, cfg)

	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()
	body := `{"documents":[
		{"sourceType":"course","sourceId":"` + id1.String() + `","title":"a"},
		{"sourceType":"course","sourceId":"` + id2.String() + `","title":"b"},
		{"sourceType":"course","sourceId":"` + id3.String() + `","title":"c"}
	]}`

	rec := doRequest(t, h, http.MethodPost, "/internal/v1/search/reindex", body, "test-token")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when batch exceeds limit, got %d", rec.Code)
	}
}

func TestHealthAndReady(t *testing.T) {
	repo := &stubRepo{countVal: 0}
	h := buildServer(t, repo, testConfig())

	rec := doRequest(t, h, http.MethodGet, "/health", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for health, got %d", rec.Code)
	}

	rec = doRequest(t, h, http.MethodGet, "/ready", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for ready, got %d", rec.Code)
	}

	// Ready reports unavailable when the database is down.
	repo.countErr = context.DeadlineExceeded
	rec = doRequest(t, h, http.MethodGet, "/ready", "", "")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when database is down, got %d", rec.Code)
	}
}

func TestPublicRoutesDoNotRequireAuth(t *testing.T) {
	repo := &stubRepo{searchTotal: 0}
	h := buildServer(t, repo, testConfig())

	rec := doRequest(t, h, http.MethodGet, "/?q=golang", "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for anonymous search, got %d", rec.Code)
	}
}

func TestSearchRecordsNormalizedEvent(t *testing.T) {
	repo := &stubRepo{searchTotal: 0}
	h := buildServer(t, repo, testConfig())

	rec := doRequest(t, h, http.MethodGet, "/?q="+url.QueryEscape("  Go   Lang "), "", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// Event recording is asynchronous; poll briefly.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(repo.recordedEvents) > 0 {
			if repo.recordedEvents[0] != "go lang" {
				t.Fatalf("expected normalized 'go lang', got %q", repo.recordedEvents[0])
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected a normalized search event to be recorded")
}
