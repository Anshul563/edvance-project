package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/video-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
	"github.com/Anshul563/edvance-project/services/video-service/internal/service"
)

type videoService interface {
	CreateVideo(
		ctx context.Context,
		userID uuid.UUID,
		contentID uuid.UUID,
		creatorID uuid.UUID,
	) (*model.Video, error)
	GetVideoView(
		ctx context.Context,
		viewerID uuid.UUID,
		id uuid.UUID,
	) (*service.VideoView, error)
	GetVideoViewByContent(
		ctx context.Context,
		viewerID uuid.UUID,
		contentID uuid.UUID,
	) (*service.VideoView, error)
	SetSource(
		ctx context.Context,
		userID uuid.UUID,
		videoID uuid.UUID,
		sourceObjectKey string,
	) (*model.Video, error)
	UpdateProcessingState(
		ctx context.Context,
		videoID uuid.UUID,
		input service.ProcessingInput,
	) (*model.Video, error)
	DeleteVideo(
		ctx context.Context,
		userID uuid.UUID,
		videoID uuid.UUID,
	) (*model.Video, error)
	ListCreatorVideos(
		ctx context.Context,
		viewerID uuid.UUID,
		creatorID uuid.UUID,
		page int,
		limit int,
		status *model.VideoStatus,
		includeDeleted bool,
	) (*service.VideoPage, error)
}

type VideoHandler struct {
	videos videoService
}

func NewVideoHandler(videos videoService) *VideoHandler {
	return &VideoHandler{
		videos: videos,
	}
}

// publicVideoResponse is the safe subset for anonymous viewers and
// non-owners. The playback manifest appears only when the video is
// actually playable; source keys and processing errors never leave the
// service through this shape.
type publicVideoResponse struct {
	ID                  string    `json:"id"`
	ContentID           string    `json:"contentId"`
	CreatorID           string    `json:"creatorId"`
	Status              string    `json:"status"`
	DurationSeconds     *int64    `json:"durationSeconds,omitempty"`
	Width               *int32    `json:"width,omitempty"`
	Height              *int32    `json:"height,omitempty"`
	ThumbnailURL        *string   `json:"thumbnailUrl,omitempty"`
	PlaybackManifestURL *string   `json:"playbackManifestUrl,omitempty"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

// ownerVideoResponse is the full record for verified owners (and the
// internal engine route).
type ownerVideoResponse struct {
	publicVideoResponse
	SourceObjectKey *string `json:"sourceObjectKey,omitempty"`
	ProcessingError *string `json:"processingError,omitempty"`
}

type createVideoRequest struct {
	ContentID string `json:"contentId"`
	CreatorID string `json:"creatorId"`
}

// Create registers a video in pending state for content the caller may
// manage. Media fields are service-controlled and never client-settable.
func (h *VideoHandler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	var request createVideoRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid request body",
			},
		)
		return
	}

	contentID, err := uuid.Parse(request.ContentID)
	if err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid content id",
			},
		)
		return
	}

	creatorID, err := uuid.Parse(request.CreatorID)
	if err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid creator id",
			},
		)
		return
	}

	video, err := h.videos.CreateVideo(r.Context(), userID, contentID, creatorID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toOwnerResponse(video))
}

// Get returns a video. Owners (verified via optional auth) see the full
// record; everyone else sees the public subset. Deleted rows are 404.
func (h *VideoHandler) Get(
	w http.ResponseWriter,
	r *http.Request,
) {
	id, ok := parseVideoID(w, r)
	if !ok {
		return
	}

	viewerID, _ := middleware.GetUserID(r.Context())

	view, err := h.videos.GetVideoView(r.Context(), viewerID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeView(w, view)
}

// GetByContent returns the video for a content record, with the same
// owner/public visibility as Get.
func (h *VideoHandler) GetByContent(
	w http.ResponseWriter,
	r *http.Request,
) {
	contentID, err := uuid.Parse(chi.URLParam(r, "contentID"))
	if err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid content id",
			},
		)
		return
	}

	viewerID, _ := middleware.GetUserID(r.Context())

	view, err := h.videos.GetVideoViewByContent(r.Context(), viewerID, contentID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeView(w, view)
}

type setSourceRequest struct {
	SourceObjectKey string `json:"sourceObjectKey"`
}

// SetSource assigns the source object key, moving pending -> uploading.
// Owner-only; deleted videos and empty keys are rejected.
func (h *VideoHandler) SetSource(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	id, ok := parseVideoID(w, r)
	if !ok {
		return
	}

	var request setSourceRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid request body",
			},
		)
		return
	}

	video, err := h.videos.SetSource(r.Context(), userID, id, request.SourceObjectKey)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toOwnerResponse(video))
}

// Delete soft-deletes a video. Idempotent: already-deleted rows succeed.
func (h *VideoHandler) Delete(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	id, ok := parseVideoID(w, r)
	if !ok {
		return
	}

	video, err := h.videos.DeleteVideo(r.Context(), userID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toOwnerResponse(video))
}

type listResponse struct {
	Items      []any              `json:"items"`
	Pagination paginationResponse `json:"pagination"`
}

type paginationResponse struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

// ListByCreator returns a creator's videos, newest first. Deleted rows
// appear only for owning viewers via ?includeDeleted=true.
func (h *VideoHandler) ListByCreator(
	w http.ResponseWriter,
	r *http.Request,
) {
	creatorID, err := uuid.Parse(chi.URLParam(r, "creatorID"))
	if err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid creator id",
			},
		)
		return
	}

	query := r.URL.Query()

	page, err := strconv.Atoi(query.Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(query.Get("limit"))
	if err != nil || limit < 1 {
		limit = 20
	}

	var status *model.VideoStatus

	if raw := query.Get("status"); raw != "" {
		parsed := model.VideoStatus(raw)

		switch parsed {
		case model.VideoStatusPending,
			model.VideoStatusUploading,
			model.VideoStatusProcessing,
			model.VideoStatusReady,
			model.VideoStatusFailed,
			model.VideoStatusDeleted:
			status = &parsed

		default:
			writeJSON(
				w,
				http.StatusBadRequest,
				map[string]string{
					"error": "invalid status",
				},
			)
			return
		}
	}

	includeDeleted := query.Get("includeDeleted") == "true"
	viewerID, _ := middleware.GetUserID(r.Context())

	pageOut, err := h.videos.ListCreatorVideos(
		r.Context(),
		viewerID,
		creatorID,
		page,
		limit,
		status,
		includeDeleted,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	items := make([]any, 0, len(pageOut.Items))

	for _, video := range pageOut.Items {
		if pageOut.Owner {
			items = append(items, toOwnerResponse(video))
		} else {
			items = append(items, toPublicResponse(video))
		}
	}

	writeJSON(
		w,
		http.StatusOK,
		listResponse{
			Items: items,
			Pagination: paginationResponse{
				Page:       pageOut.Page,
				Limit:      pageOut.Limit,
				Total:      pageOut.Total,
				TotalPages: pageOut.TotalPages,
			},
		},
	)
}

type processingUpdateRequest struct {
	Status              string  `json:"status"`
	DurationSeconds     *int64  `json:"durationSeconds"`
	Width               *int32  `json:"width"`
	Height              *int32  `json:"height"`
	ThumbnailURL        *string `json:"thumbnailUrl"`
	PlaybackManifestURL *string `json:"playbackManifestUrl"`
	ProcessingError     *string `json:"processingError"`
}

// UpdateProcessingState applies a media-engine callback. This route is
// NOT proxied by the gateway and is guarded by the internal shared key
// (later mTLS). There is deliberately no public status-write endpoint.
func (h *VideoHandler) UpdateProcessingState(
	w http.ResponseWriter,
	r *http.Request,
) {
	id, ok := parseVideoID(w, r)
	if !ok {
		return
	}

	var request processingUpdateRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid request body",
			},
		)
		return
	}

	video, err := h.videos.UpdateProcessingState(
		r.Context(),
		id,
		service.ProcessingInput{
			Status:              model.VideoStatus(request.Status),
			DurationSeconds:     request.DurationSeconds,
			Width:               request.Width,
			Height:              request.Height,
			ThumbnailURL:        request.ThumbnailURL,
			PlaybackManifestURL: request.PlaybackManifestURL,
			ProcessingError:     request.ProcessingError,
		},
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toOwnerResponse(video))
}

func parseVideoID(
	w http.ResponseWriter,
	r *http.Request,
) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "videoID"))
	if err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid video id",
			},
		)
		return uuid.Nil, false
	}

	return id, true
}

// writeView renders the full record for owners and the safe public
// subset for everyone else. The manifest is included only when the
// video is actually playable.
func writeView(w http.ResponseWriter, view *service.VideoView) {
	if view.Owner {
		writeJSON(w, http.StatusOK, toOwnerResponse(view.Video))
		return
	}

	writeJSON(w, http.StatusOK, toPublicResponse(view.Video))
}

func toPublicResponse(video *model.Video) publicVideoResponse {
	response := publicVideoResponse{
		ID:              video.ID.String(),
		ContentID:       video.ContentID.String(),
		CreatorID:       video.CreatorID.String(),
		Status:          string(video.Status),
		DurationSeconds: video.DurationSeconds,
		Width:           video.Width,
		Height:          video.Height,
		ThumbnailURL:    video.ThumbnailURL,
		CreatedAt:       video.CreatedAt,
		UpdatedAt:       video.UpdatedAt,
	}

	if video.Playable() {
		response.PlaybackManifestURL = video.PlaybackManifestURL
	}

	return response
}

func toOwnerResponse(video *model.Video) ownerVideoResponse {
	return ownerVideoResponse{
		publicVideoResponse: toPublicResponse(video),
		SourceObjectKey:     video.SourceObjectKey,
		ProcessingError:     video.ProcessingError,
	}
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": "unauthorized",
	})
}

func writeJSON(
	w http.ResponseWriter,
	status int,
	data any,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(data)
}
