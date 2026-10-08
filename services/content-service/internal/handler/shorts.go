package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
	"github.com/Anshul563/edvance-project/services/content-service/internal/service"
)

// shortService is the service contract this handler depends on.
type shortService interface {
	Create(
		ctx context.Context,
		actor service.Actor,
		input service.CreateShortInput,
	) (*model.Short, error)
	Get(ctx context.Context, actor service.Actor, id uuid.UUID) (*model.Short, error)
	Update(
		ctx context.Context,
		actor service.Actor,
		id uuid.UUID,
		input service.UpdateShortInput,
	) (*model.Short, error)
	Delete(ctx context.Context, actor service.Actor, id uuid.UUID) error
	Publish(ctx context.Context, actor service.Actor, id uuid.UUID) (*model.Short, error)
	Unpublish(ctx context.Context, actor service.Actor, id uuid.UUID) (*model.Short, error)
	ListShorts(
		ctx context.Context,
		actor service.Actor,
		params service.ListShortsParams,
	) (service.Page[*model.Short], error)
	SetMediaStatus(
		ctx context.Context,
		id uuid.UUID,
		status string,
		durationSeconds *int,
		mediaAssetID *uuid.UUID,
	) (*model.Short, error)
	RecordView(ctx context.Context, id uuid.UUID) (*model.Short, error)
	AdjustCounters(
		ctx context.Context,
		id uuid.UUID,
		viewDelta int64,
		likeDelta int64,
		commentDelta int64,
	) (*model.Short, error)
}

type ShortHandler struct {
	shorts shortService
}

func NewShortHandler(shorts shortService) *ShortHandler {
	return &ShortHandler{
		shorts: shorts,
	}
}

type shortResponse struct {
	ID              string       `json:"id"`
	CreatorID       string       `json:"creatorId"`
	Title           string       `json:"title"`
	Description     *string      `json:"description,omitempty"`
	Slug            string       `json:"slug"`
	Visibility      string       `json:"visibility"`
	Status          string       `json:"status"`
	MediaAssetID    *string      `json:"mediaAssetId,omitempty"`
	ThumbnailURL    *string      `json:"thumbnailUrl,omitempty"`
	DurationSeconds *int         `json:"durationSeconds,omitempty"`
	ViewCount       int64        `json:"viewCount"`
	LikeCount       int64        `json:"likeCount"`
	CommentCount    int64        `json:"commentCount"`
	PublishedAt     *time.Time   `json:"publishedAt,omitempty"`
	CreatedAt       time.Time    `json:"createdAt"`
	UpdatedAt       time.Time    `json:"updatedAt"`
	Tags            []tagResponse `json:"tags"`
}

func toShortResponse(short *model.Short) shortResponse {
	response := shortResponse{
		ID:              short.ID.String(),
		CreatorID:       short.CreatorID.String(),
		Title:           short.Title,
		Description:     short.Description,
		Slug:            short.Slug,
		Visibility:      string(short.Visibility),
		Status:          string(short.Status),
		ViewCount:       short.ViewCount,
		LikeCount:       short.LikeCount,
		CommentCount:    short.CommentCount,
		PublishedAt:     short.PublishedAt,
		CreatedAt:       short.CreatedAt,
		UpdatedAt:       short.UpdatedAt,
		Tags:            toTagResponses(short.Tags),
	}

	if short.MediaAssetID != nil {
		value := short.MediaAssetID.String()
		response.MediaAssetID = &value
	}

	response.ThumbnailURL = short.ThumbnailURL
	response.DurationSeconds = short.DurationSeconds

	return response
}

type createShortRequest struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Visibility   string   `json:"visibility"`
	MediaAssetID string   `json:"mediaAssetId"`
	ThumbnailURL string   `json:"thumbnailUrl"`
	Tags         []string `json:"tags"`

	// Server-owned. Declared only so attempts to set them are caught.
	CreatorID    string  `json:"creatorId"`
	Status       string  `json:"status"`
	Slug         string  `json:"slug"`
	ViewCount    *int64  `json:"viewCount"`
	LikeCount    *int64  `json:"likeCount"`
	CommentCount *int64  `json:"commentCount"`
	PublishedAt  *string `json:"publishedAt"`
}

type updateShortRequest struct {
	Title        *string   `json:"title"`
	Description  *string   `json:"description"`
	Visibility   *string   `json:"visibility"`
	MediaAssetID *string   `json:"mediaAssetId"`
	ThumbnailURL *string   `json:"thumbnailUrl"`
	Tags         *[]string `json:"tags"`

	// Server-owned. Declared only so attempts to set them are caught.
	CreatorID    *string `json:"creatorId"`
	Status       *string `json:"status"`
	Slug         *string `json:"slug"`
	ViewCount    *int64  `json:"viewCount"`
	LikeCount    *int64  `json:"likeCount"`
	CommentCount *int64  `json:"commentCount"`
	PublishedAt  *string `json:"publishedAt"`
}

// Create registers a short in draft state. Identity, status, counters,
// slug, and timestamps are service-controlled.
func (h *ShortHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Authenticated() {
		writeUnauthorized(w)
		return
	}

	var request createShortRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_REQUEST", "invalid request body")
		return
	}

	if rejectOwnedFields(
		w,
		ownedField{"creatorId", request.CreatorID != ""},
		ownedField{"status", request.Status != ""},
		ownedField{"slug", request.Slug != ""},
		ownedField{"viewCount", request.ViewCount != nil},
		ownedField{"likeCount", request.LikeCount != nil},
		ownedField{"commentCount", request.CommentCount != nil},
		ownedField{"publishedAt", request.PublishedAt != nil},
	) {
		return
	}

	mediaAssetID, ok := optionalUUIDValue(w, "mediaAssetId", request.MediaAssetID)
	if !ok {
		return
	}

	short, err := h.shorts.Create(r.Context(), actor, service.CreateShortInput{
		Title:        request.Title,
		Description:  optionalString(request.Description),
		Visibility:   request.Visibility,
		MediaAssetID: mediaAssetID,
		ThumbnailURL: optionalString(request.ThumbnailURL),
		Tags:         request.Tags,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toShortResponse(short))
}

// Get returns a short. Drafts and private shorts read as 404 for
// everyone except the owning creator.
func (h *ShortHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "shortID")
	if !ok {
		return
	}

	short, err := h.shorts.Get(r.Context(), actorFrom(r), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toShortResponse(short))
}

// Update applies an owner edit. Server-owned fields are rejected.
func (h *ShortHandler) Update(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Authenticated() {
		writeUnauthorized(w)
		return
	}

	id, ok := parseUUIDParam(w, r, "shortID")
	if !ok {
		return
	}

	var request updateShortRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_REQUEST", "invalid request body")
		return
	}

	if rejectOwnedFields(
		w,
		ownedField{"creatorId", request.CreatorID != nil},
		ownedField{"status", request.Status != nil},
		ownedField{"slug", request.Slug != nil},
		ownedField{"viewCount", request.ViewCount != nil},
		ownedField{"likeCount", request.LikeCount != nil},
		ownedField{"commentCount", request.CommentCount != nil},
		ownedField{"publishedAt", request.PublishedAt != nil},
	) {
		return
	}

	input := service.UpdateShortInput{
		Title:       request.Title,
		Description: request.Description,
		Visibility:  request.Visibility,
		Tags:        request.Tags,
	}

	if request.ThumbnailURL != nil {
		input.ThumbnailURL = optionalString(*request.ThumbnailURL)
	}

	if request.MediaAssetID != nil {
		mediaAssetID, ok := optionalUUIDValue(
			w,
			"mediaAssetId",
			*request.MediaAssetID,
		)
		if !ok {
			return
		}

		if mediaAssetID == nil {
			writeBadRequest(w, "INVALID_INPUT", "mediaAssetId must be a valid uuid")
			return
		}

		input.MediaAssetID = mediaAssetID
	}

	short, err := h.shorts.Update(r.Context(), actor, id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toShortResponse(short))
}

// Delete removes a short and its tag relations.
func (h *ShortHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Authenticated() {
		writeUnauthorized(w)
		return
	}

	id, ok := parseUUIDParam(w, r, "shortID")
	if !ok {
		return
	}

	if err := h.shorts.Delete(r.Context(), actor, id); err != nil {
		writeServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// List returns a paginated short listing.
func (h *ShortHandler) List(w http.ResponseWriter, r *http.Request) {
	page, ok := parsePagination(w, r)
	if !ok {
		return
	}

	creatorID, ok := parseOptionalUUID(w, r, "creator_id")
	if !ok {
		return
	}

	pagination, err := h.shorts.ListShorts(
		r.Context(),
		actorFrom(r),
		service.ListShortsParams{
			Page:      page.Page,
			Limit:     page.Limit,
			CreatorID: creatorID,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		mapPage(pagination, toShortResponse),
	)
}

// ListByCreator returns a paginated listing for one creator.
func (h *ShortHandler) ListByCreator(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := parseUUIDParam(w, r, "creatorID")
	if !ok {
		return
	}

	page, ok := parsePagination(w, r)
	if !ok {
		return
	}

	pagination, err := h.shorts.ListShorts(
		r.Context(),
		actorFrom(r),
		service.ListShortsParams{
			Page:      page.Page,
			Limit:     page.Limit,
			CreatorID: &creatorID,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		mapPage(pagination, toShortResponse),
	)
}

// Publish runs the publishing gate and marks the short public.
func (h *ShortHandler) Publish(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Authenticated() {
		writeUnauthorized(w)
		return
	}

	id, ok := parseUUIDParam(w, r, "shortID")
	if !ok {
		return
	}

	short, err := h.shorts.Publish(r.Context(), actor, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toShortResponse(short))
}

// Unpublish takes a short off the public surface.
func (h *ShortHandler) Unpublish(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Authenticated() {
		writeUnauthorized(w)
		return
	}

	id, ok := parseUUIDParam(w, r, "shortID")
	if !ok {
		return
	}

	short, err := h.shorts.Unpublish(r.Context(), actor, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toShortResponse(short))
}

// SetMediaStatus is the internal pipeline callback. It lives under
// /internal/v1 and is guarded by the shared internal key; the API
// gateway never proxies it.
func (h *ShortHandler) SetMediaStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "shortID")
	if !ok {
		return
	}

	var request internalStatusRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_REQUEST", "invalid request body")
		return
	}

	var mediaAssetID *uuid.UUID

	if request.MediaAssetID != nil {
		value, ok := optionalUUIDValue(w, "mediaAssetId", *request.MediaAssetID)
		if !ok {
			return
		}

		mediaAssetID = value
	}

	short, err := h.shorts.SetMediaStatus(
		r.Context(),
		id,
		request.Status,
		request.DurationSeconds,
		mediaAssetID,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toShortResponse(short))
}

// RecordView increments the view counter. Counters are internal-only.
func (h *ShortHandler) RecordView(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "shortID")
	if !ok {
		return
	}

	short, err := h.shorts.RecordView(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toShortResponse(short))
}

// AdjustCounters applies counter deltas for social/analytics consumers.
func (h *ShortHandler) AdjustCounters(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "shortID")
	if !ok {
		return
	}

	var request countersRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_REQUEST", "invalid request body")
		return
	}

	short, err := h.shorts.AdjustCounters(
		r.Context(),
		id,
		request.ViewDelta,
		request.LikeDelta,
		request.CommentDelta,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toShortResponse(short))
}
