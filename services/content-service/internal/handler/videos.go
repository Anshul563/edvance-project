package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
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
	GetBySlug(
		ctx context.Context,
		actor service.Actor,
		slug string,
	) (*model.Video, error)
	Playback(
		ctx context.Context,
		actor service.Actor,
		id uuid.UUID,
	) (*service.Playback, error)
	PlaybackBySlug(
		ctx context.Context,
		actor service.Actor,
		slug string,
	) (*service.Playback, error)
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
		update service.MediaStatusUpdate,
	) (*model.Video, error)
	SetMediaStatusByMediaAsset(
		ctx context.Context,
		mediaAssetID uuid.UUID,
		update service.MediaStatusUpdate,
	) ([]*model.Video, error)
	SetModeration(
		ctx context.Context,
		id uuid.UUID,
		status string,
		reason *string,
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
	ID          string  `json:"id"`
	CreatorID   string  `json:"creatorId"`
	Title       string  `json:"title"`
	Description *string `json:"description,omitempty"`
	Slug        string  `json:"slug"`
	Language    string  `json:"language"`
	CategoryID  *string `json:"categoryId,omitempty"`
	Visibility  string  `json:"visibility"`
	Status      string  `json:"status"`

	ModerationStatus string  `json:"moderationStatus"`
	ModerationReason *string `json:"moderationReason,omitempty"`

	MediaAssetID           *string `json:"mediaAssetId,omitempty"`
	ThumbnailURL           *string `json:"thumbnailUrl,omitempty"`
	DurationSeconds        *int    `json:"durationSeconds,omitempty"`
	ProcessingErrorCode    *string `json:"processingErrorCode,omitempty"`
	ProcessingErrorMessage *string `json:"processingErrorMessage,omitempty"`

	ViewCount    int64         `json:"viewCount"`
	LikeCount    int64         `json:"likeCount"`
	CommentCount int64         `json:"commentCount"`
	PublishedAt  *time.Time    `json:"publishedAt,omitempty"`
	CreatedAt    time.Time     `json:"createdAt"`
	UpdatedAt    time.Time     `json:"updatedAt"`
	Tags         []tagResponse `json:"tags"`
}

func toVideoResponse(video *model.Video) videoResponse {
	response := videoResponse{
		ID:                     video.ID.String(),
		CreatorID:              video.CreatorID.String(),
		Title:                  video.Title,
		Description:            video.Description,
		Slug:                   video.Slug,
		Language:               video.Language,
		Visibility:             string(video.Visibility),
		Status:                 string(video.Status),
		ModerationStatus:       string(video.ModerationStatus),
		ModerationReason:       video.ModerationReason,
		ViewCount:              video.ViewCount,
		LikeCount:              video.LikeCount,
		CommentCount:           video.CommentCount,
		PublishedAt:            video.PublishedAt,
		CreatedAt:              video.CreatedAt,
		UpdatedAt:              video.UpdatedAt,
		Tags:                   toTagResponses(video.Tags),
		ProcessingErrorCode:    video.ProcessingErrorCode,
		ProcessingErrorMessage: video.ProcessingErrorMessage,
	}

	if video.MediaAssetID != nil {
		value := video.MediaAssetID.String()
		response.MediaAssetID = &value
	}

	if video.CategoryID != nil {
		value := video.CategoryID.String()
		response.CategoryID = &value
	}

	response.ThumbnailURL = video.ThumbnailURL
	response.DurationSeconds = video.DurationSeconds

	return response
}

type createVideoRequest struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Visibility   string   `json:"visibility"`
	Language     string   `json:"language"`
	CategorySlug string   `json:"categorySlug"`
	MediaAssetID string   `json:"mediaAssetId"`
	ThumbnailURL string   `json:"thumbnailUrl"`
	Tags         []string `json:"tags"`

	// Server-owned. Declared only so attempts to set them are caught.
	CreatorID        string  `json:"creatorId"`
	Status           string  `json:"status"`
	Slug             string  `json:"slug"`
	ModerationStatus string  `json:"moderationStatus"`
	ModerationReason string  `json:"moderationReason"`
	ViewCount        *int64  `json:"viewCount"`
	LikeCount        *int64  `json:"likeCount"`
	CommentCount     *int64  `json:"commentCount"`
	PublishedAt      *string `json:"publishedAt"`
}

type updateVideoRequest struct {
	Title        *string   `json:"title"`
	Description  *string   `json:"description"`
	Visibility   *string   `json:"visibility"`
	Language     *string   `json:"language"`
	CategorySlug *string   `json:"categorySlug"`
	MediaAssetID *string   `json:"mediaAssetId"`
	ThumbnailURL *string   `json:"thumbnailUrl"`
	Tags         *[]string `json:"tags"`

	// Server-owned. Declared only so attempts to set them are caught.
	CreatorID        *string `json:"creatorId"`
	Status           *string `json:"status"`
	Slug             *string `json:"slug"`
	ModerationStatus *string `json:"moderationStatus"`
	ModerationReason *string `json:"moderationReason"`
	ViewCount        *int64  `json:"viewCount"`
	LikeCount        *int64  `json:"likeCount"`
	CommentCount     *int64  `json:"commentCount"`
	PublishedAt      *string `json:"publishedAt"`
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
		ownedField{"moderationStatus", request.ModerationStatus != ""},
		ownedField{"moderationReason", request.ModerationReason != ""},
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
		Language:     request.Language,
		CategorySlug: optionalString(request.CategorySlug),
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

// GetBySlug returns a video by its globally unique slug.
func (h *VideoHandler) GetBySlug(w http.ResponseWriter, r *http.Request) {
	video, err := h.videos.GetBySlug(
		r.Context(),
		actorFrom(r),
		chi.URLParam(r, "slug"),
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toVideoResponse(video))
}

// Playback returns player-facing facts for a video. Access is enforced
// here (public/unlisted published, or owner); video-service resolves
// the manifest itself.
func (h *VideoHandler) Playback(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "videoID")
	if !ok {
		return
	}

	playback, err := h.videos.Playback(r.Context(), actorFrom(r), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, playback)
}

// PlaybackBySlug is the direct-link form used by public video pages.
func (h *VideoHandler) PlaybackBySlug(w http.ResponseWriter, r *http.Request) {
	playback, err := h.videos.PlaybackBySlug(
		r.Context(),
		actorFrom(r),
		chi.URLParam(r, "slug"),
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, playback)
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
		ownedField{"moderationStatus", request.ModerationStatus != nil},
		ownedField{"moderationReason", request.ModerationReason != nil},
		ownedField{"viewCount", request.ViewCount != nil},
		ownedField{"likeCount", request.LikeCount != nil},
		ownedField{"commentCount", request.CommentCount != nil},
		ownedField{"publishedAt", request.PublishedAt != nil},
	) {
		return
	}

	input := service.UpdateVideoInput{
		Title:        request.Title,
		Description:  request.Description,
		Visibility:   request.Visibility,
		Language:     request.Language,
		CategorySlug: request.CategorySlug,
		Tags:         request.Tags,
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
			Page:         page.Page,
			Limit:        page.Limit,
			CreatorID:    creatorID,
			Status:       status,
			Visibility:   visibility,
			CategorySlug: optionalString(r.URL.Query().Get("category")),
			Language:     optionalString(r.URL.Query().Get("language")),
			TagSlug:      optionalString(r.URL.Query().Get("tag")),
			Query:        optionalString(r.URL.Query().Get("q")),
			Sort:         r.URL.Query().Get("sort"),
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
// /internal/* at the service root and is guarded by the shared
// internal key; the API gateway never proxies it.
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
		service.MediaStatusUpdate{
			Status:          request.Status,
			DurationSeconds: request.DurationSeconds,
			MediaAssetID:    mediaAssetID,
			ThumbnailURL:    request.ThumbnailURL,
			ErrorCode:       request.ErrorCode,
			ErrorMessage:    request.ErrorMessage,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toVideoResponse(video))
}

// SetMediaStatusByAsset is the pipeline's actual callback shape: it
// knows the media asset ID (from video-service), not the content row
// ID. It is internal-only and idempotent.
func (h *VideoHandler) SetMediaStatusByAsset(
	w http.ResponseWriter,
	r *http.Request,
) {
	mediaAssetID, ok := parseUUIDParam(w, r, "mediaAssetID")
	if !ok {
		return
	}

	var request internalStatusRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_REQUEST", "invalid request body")
		return
	}

	videos, err := h.videos.SetMediaStatusByMediaAsset(
		r.Context(),
		mediaAssetID,
		service.MediaStatusUpdate{
			Status:          request.Status,
			DurationSeconds: request.DurationSeconds,
			MediaAssetID:    &mediaAssetID,
			ThumbnailURL:    request.ThumbnailURL,
			ErrorCode:       request.ErrorCode,
			ErrorMessage:    request.ErrorMessage,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	responses := make([]videoResponse, 0, len(videos))
	for _, video := range videos {
		responses = append(responses, toVideoResponse(video))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"videos": responses,
	})
}

// SetModeration applies a review decision. Internal-only: no creator
// route accepts a moderation field.
func (h *VideoHandler) SetModeration(w http.ResponseWriter, r *http.Request) {
	id, ok := parseUUIDParam(w, r, "videoID")
	if !ok {
		return
	}

	var request moderationRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeBadRequest(w, "INVALID_REQUEST", "invalid request body")
		return
	}

	video, err := h.videos.SetModeration(
		r.Context(),
		id,
		request.Status,
		request.Reason,
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
