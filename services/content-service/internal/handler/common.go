package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
	"github.com/Anshul563/edvance-project/services/content-service/internal/service"
)

// actorFrom builds the service actor from middleware-injected identity.
// The access token travels with it so ownership checks can call
// creator-service on the caller's behalf — never on behalf of an ID the
// client claimed.
func actorFrom(r *http.Request) service.Actor {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		return service.Anonymous()
	}

	return service.Actor{
		UserID:      userID,
		AccessToken: middleware.GetAccessToken(r.Context()),
	}
}

// parseUUIDParam reads and validates a UUID path parameter.
func parseUUIDParam(
	w http.ResponseWriter,
	r *http.Request,
	name string,
) (uuid.UUID, bool) {
	raw := chi.URLParam(r, name)

	id, err := uuid.Parse(raw)
	if err != nil {
		writeBadRequest(w, "INVALID_ID", name+" must be a valid uuid")

		return uuid.Nil, false
	}

	return id, true
}

// paginationQuery is the shared ?page=&limit= parser. Invalid values are
// rejected loudly instead of silently corrected; the service still
// clamps limit into the configured maximum.
type paginationQuery struct {
	Page  int
	Limit int
}

func parsePagination(w http.ResponseWriter, r *http.Request) (paginationQuery, bool) {
	query := paginationQuery{}

	pageValue := r.URL.Query().Get("page")
	if pageValue != "" {
		page, err := strconv.Atoi(pageValue)
		if err != nil {
			writeBadRequest(w, "INVALID_PAGINATION", "page must be an integer")

			return paginationQuery{}, false
		}

		query.Page = page
	}

	limitValue := r.URL.Query().Get("limit")
	if limitValue != "" {
		limit, err := strconv.Atoi(limitValue)
		if err != nil {
			writeBadRequest(w, "INVALID_PAGINATION", "limit must be an integer")

			return paginationQuery{}, false
		}

		query.Limit = limit
	}

	return query, true
}

// parseOptionalUUID reads an optional ?name=uuid query parameter.
func parseOptionalUUID(
	w http.ResponseWriter,
	r *http.Request,
	name string,
) (*uuid.UUID, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, true
	}

	value, err := uuid.Parse(raw)
	if err != nil {
		writeBadRequest(w, "INVALID_QUERY", name+" must be a valid uuid")

		return nil, false
	}

	return &value, true
}

// optionalString converts a form value into a *string, treating blank
// as unset.
func optionalString(value string) *string {
	trimmed := strings.TrimSpace(value)

	if trimmed == "" {
		return nil
	}

	return &trimmed
}

// mapPage converts a service page of entities into a page of response
// DTOs.
func mapPage[T any, U any](
	page service.Page[T],
	mapper func(T) U,
) service.Page[U] {
	items := make([]U, 0, len(page.Items))

	for _, item := range page.Items {
		items = append(items, mapper(item))
	}

	return service.Page[U]{
		Items:   items,
		Page:    page.Page,
		Limit:   page.Limit,
		Total:   page.Total,
		HasNext: page.HasNext,
	}
}

// ownedField describes a server-owned request field and whether the
// client supplied it. Clients may never write creator IDs, statuses,
// counters, slugs, or published timestamps — attempts are rejected
// loudly instead of silently dropped, so a client bug surfaces in
// development rather than in production data.
type ownedField struct {
	name string
	set  bool
}

func firstOwnedField(fields ...ownedField) string {
	for _, field := range fields {
		if field.set {
			return field.name
		}
	}

	return ""
}

// rejectOwnedFields writes a 400 naming the first server-owned field
// the client tried to set. It reports whether it wrote a response.
func rejectOwnedFields(w http.ResponseWriter, fields ...ownedField) bool {
	name := firstOwnedField(fields...)
	if name == "" {
		return false
	}

	writeBadRequest(
		w,
		"FIELD_NOT_SETTABLE",
		name+" is controlled by the service and cannot be set by clients",
	)

	return true
}

type tagResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type internalStatusRequest struct {
	Status          string  `json:"status"`
	DurationSeconds *int    `json:"durationSeconds"`
	MediaAssetID    *string `json:"mediaAssetId"`
}

type countersRequest struct {
	ViewDelta    int64 `json:"viewDelta"`
	LikeDelta    int64 `json:"likeDelta"`
	CommentDelta int64 `json:"commentDelta"`
}

// optionalUUIDValue parses an optional UUID form value. ok is false
// only when the value was present but malformed.
func optionalUUIDValue(
	w http.ResponseWriter,
	field string,
	raw string,
) (*uuid.UUID, bool) {
	trimmed := optionalString(raw)
	if trimmed == nil {
		return nil, true
	}

	value, err := uuid.Parse(*trimmed)
	if err != nil {
		writeBadRequest(w, "INVALID_INPUT", field+" must be a valid uuid")

		return nil, false
	}

	return &value, true
}

func toTagResponses(tags []model.Tag) []tagResponse {
	responses := make([]tagResponse, 0, len(tags))

	for _, tag := range tags {
		responses = append(responses, tagResponse{
			ID:   tag.ID.String(),
			Name: tag.Name,
			Slug: tag.Slug,
		})
	}

	return responses
}

