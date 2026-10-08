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

// videoService is the service contract this handler depends on.
type videoService interface {
	Create(
		ctx context.Context,
		actor service.Actor,
		input service.CreateVideoInput,
	) (*model.Video, error)
	Get(ctx context.Context, actor service.Actor, id uuid.UUID) (*model.Video, error)
	Update(
		ctx context.Context,
		actor service.Actor,
		id uuid.UUID,
		input service.UpdateVideoInput,
	) (*model.Video, error)
	Delete(ctx context.Context, actor service.Actor, id uuid.UUID) error
	Publish(ctx context.Context, actor service.Actor, id uuid.UUID) (*model.Video, error)
	Unpublish(ctx context.Context, actor service.Actor, id uuid.UUID) (*model.Video, error)
	ListVideos(
		ctx context.Context,
		actor service.Actor,
		params service.ListVideosParams,
	) (service.Page[*model.Video], error)
	SetMediaStatus(
		ctx context.Context,
		id uuid.UUID,
		status string,
		durationSeconds *int,
		mediaAssetID *uuid.UUID,
	) (*model.Video, error)
	RecordView(ctx context.Context, id uuid.UUID) (*model.Video, error)
	AdjustCounters(
		ctx context.Context,
		id uuid.UUID,
		viewDelta int64,
		likeDelta int64,
		commentDelta int64,
	) (*model.Video, error)
}

type VideoHandler struct {
	videos videoService
}

func NewVideoHandler(videos videoService) *VideoHandler {
	return &VideoHandler{
		videos: videos,
	}
}

type videoResponse struct {
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

func toVideoResponse(video *model.Video) videoResponse {
	response := videoResponse{
		ID:              video.ID.String(),
		CreatorID:       video.CreatorID.String(),
		Title:           video.Title,
		Description:     video.Description,
		Slug:            video.Slug,
		Visibility:      string(video.Visibility),
		Status:          string(video.Status),
		ViewCount:       video.ViewCount,
		LikeCount:       video.LikeCount,
		CommentCount:    video.CommentCount,
		PublishedAt:     video.PublishedAt,
		CreatedAt:       video.CreatedAt,
		UpdatedAt:       video.UpdatedAt,
		Tags:            toTagResponses(video.Tags),
	}

	if video.MediaAssetID != nil {
		value := video.MediaAssetID.String()
		response.MediaAssetID = &value
	}

	response.ThumbnailURL = video.ThumbnailURL
	response.DurationSeconds = video.DurationSeconds

	return response
}

type createVideoRequest struct {
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

type updateVideoRequest struct {
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

// Create registers a video in draft state. Identity, status, counters,
// slug, and timestamps are service-controlled.
func (h *VideoHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Authenticated() {
		writeUnauthorized(w)
		return
	}

	var request createVideoRequest

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

	video, err := h.videos.Create(r.Context(), actor, service.CreateVideoInput{
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

	writeJSON(w, http.StatusCreated, toVideoResponse(video))
}

// Get returns a video. Drafts and private videos read as 404 for
// everyone except the owning creator.
func (h *VideoHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "videoID")
	if !ok {
		return
	}

	video, err := h.videos.Get(r.Context(), actorFrom(r), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toVideoResponse(video))
}

// Update applies an owner edit. Server-owned fields are rejected.
func (h *VideoHandler) Update(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Authenticated() {
		writeUnauthorized(w)
		return
	}

	id, ok := parseUUIDParam(w, r, "videoID")
	if !ok {
		return
	}

	var request updateVideoRequest

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

	input := service.UpdateVideoInput{
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

	video, err := h.videos.Update(r.Context(), actor, id, input)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toVideoResponse(video))
}

// Delete removes a video and its tag relations.
func (h *VideoHandler) Delete(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Authenticated() {
		writeUnauthorized(w)
		return
	}

	id, ok := parseUUIDParam(w, r, "videoID")
	if !ok {
		return
	}

	if err := h.videos.Delete(r.Context(), actor, id); err != nil {
		writeServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// List returns a paginated video listing.
func (h *VideoHandler) List(w http.ResponseWriter, r *http.Request) {
	page, ok := parsePagination(w, r)
	if !ok {
		return
	}

	creatorID, ok := parseOptionalUUID(w, r, "creator_id")
	if !ok {
		return
	}

	var status *model.VideoStatus

	if raw := r.URL.Query().Get("status"); raw != "" {
		value := model.VideoStatus(raw)
		if !value.Valid() {
			writeBadRequest(w, "INVALID_STATUS", "unknown status "+raw)
			return
		}

		status = &value
	}

	var visibility *model.VideoVisibility

	if raw := r.URL.Query().Get("visibility"); raw != "" {
		value := model.VideoVisibility(raw)
		if !value.Valid() {
			writeBadRequest(w, "INVALID_VISIBILITY", "unknown visibility "+raw)
			return
		}

		visibility = &value
	}

	pagination, err := h.videos.ListVideos(
		r.Context(),
		actorFrom(r),
		service.ListVideosParams{
			Page:       page.Page,
			Limit:      page.Limit,
			CreatorID:  creatorID,
			Status:     status,
			Visibility: visibility,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(
		w,
		http.StatusOK,
		mapPage(pagination, toVideoResponse),
	)
}

// ListByCreator returns a paginated listing for one creator.
func (h *VideoHandler) ListByCreator(w http.ResponseWriter, r *http.Request) {
	creatorID, ok := parseUUIDParam(w, r, "creatorID")
	if !ok {
		return
	}

	page, ok := parsePagination(w, r)
	if !ok {
		return
	}

	pagination, err := h.videos.ListVideos(
		r.Context(),
		actorFrom(r),
		service.ListVideosParams{
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
		mapPage(pagination, toVideoResponse),
	)
}

// Publish runs the publishing gate and marks the video public.
func (h *VideoHandler) Publish(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Authenticated() {
		writeUnauthorized(w)
		return
	}

	id, ok := parseUUIDParam(w, r, "videoID")
	if !ok {
		return
	}

	video, err := h.videos.Publish(r.Context(), actor, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toVideoResponse(video))
}

// Unpublish takes a video off the public surface.
func (h *VideoHandler) Unpublish(w http.ResponseWriter, r *http.Request) {
	actor := actorFrom(r)
	if !actor.Authenticated() {
		writeUnauthorized(w)
		return
	}

	id, ok := parseUUIDParam(w, r, "videoID")
	if !ok {
		return
	}

	video, err := h.videos.Unpublish(r.Context(), actor, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toVideoResponse(video))
}

// SetMediaStatus is the internal pipeline callback. It lives under
// /internal/v1 and is guarded by the shared internal key; the API
// gateway never proxies it.
func (h *VideoHandler) SetMediaStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "videoID")
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

	video, err := h.videos.SetMediaStatus(
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

	writeJSON(w, http.StatusOK, toVideoResponse(video))
}

// RecordView increments the view counter. Counters are internal-only.
func (h *VideoHandler) RecordView(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "videoID")
	if !ok {
		return
	}

	video, err := h.videos.RecordView(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toVideoResponse(video))
}

// AdjustCounters applies counter deltas for social/analytics consumers.
func (h *VideoHandler) AdjustCounters(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "videoID")
	if !ok {
		return
	}

	var request countersRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_REQUEST", "invalid request body")
		return
	}

	video, err := h.videos.AdjustCounters(
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

	writeJSON(w, http.StatusOK, toVideoResponse(video))
}
