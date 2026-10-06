package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/media-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/media-service/internal/model"
	"github.com/Anshul563/edvance-project/services/media-service/internal/service"
)

type mediaService interface {
	CreateJob(
		ctx context.Context,
		userID uuid.UUID,
		input service.CreateJobInput,
	) (*model.MediaJob, error)
	GetJob(
		ctx context.Context,
		userID uuid.UUID,
		jobID uuid.UUID,
	) (*model.MediaJob, error)
	RefreshJob(
		ctx context.Context,
		userID uuid.UUID,
		jobID uuid.UUID,
	) (*model.MediaJob, error)
	CancelJob(
		ctx context.Context,
		userID uuid.UUID,
		jobID uuid.UUID,
	) (*model.MediaJob, error)
	RetryJob(
		ctx context.Context,
		userID uuid.UUID,
		jobID uuid.UUID,
	) (*model.MediaJob, error)
	ListVideoJobs(
		ctx context.Context,
		userID uuid.UUID,
		videoID uuid.UUID,
		page int,
		limit int,
		jobType *model.MediaJobType,
		status *model.MediaJobStatus,
	) (*service.MediaJobPage, error)
}

type MediaHandler struct {
	jobs mediaService
}

func NewMediaHandler(jobs mediaService) *MediaHandler {
	return &MediaHandler{
		jobs: jobs,
	}
}

type jobErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type jobResponse struct {
	ID                 string            `json:"id"`
	VideoID            string            `json:"videoId"`
	JobType            string            `json:"jobType"`
	Status             string            `json:"status"`
	Progress           int32             `json:"progress"`
	AttemptCount       int32             `json:"attemptCount"`
	SourceObjectKey    *string           `json:"sourceObjectKey,omitempty"`
	OutputManifestURL  *string           `json:"outputManifestUrl,omitempty"`
	OutputThumbnailURL *string           `json:"outputThumbnailUrl,omitempty"`
	DurationSeconds    *int64            `json:"durationSeconds,omitempty"`
	Width              *int32            `json:"width,omitempty"`
	Height             *int32            `json:"height,omitempty"`
	EngineJobID        *string           `json:"engineJobId,omitempty"`
	Error              *jobErrorResponse `json:"error"`
	CreatedAt          time.Time         `json:"createdAt"`
	UpdatedAt          time.Time         `json:"updatedAt"`
}

type createJobRequest struct {
	VideoID         string `json:"videoId"`
	JobType         string `json:"jobType"`
	SourceObjectKey string `json:"sourceObjectKey"`
	IdempotencyKey  string `json:"idempotencyKey"`
}

// Create registers a job and dispatches it. Engine outage preserves the
// queued job and reports 502 with its id — the job is never lost.
func (h *MediaHandler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	var request createJobRequest

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

	videoID, err := uuid.Parse(request.VideoID)
	if err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid video id",
			},
		)
		return
	}

	job, err := h.jobs.CreateJob(
		r.Context(),
		userID,
		service.CreateJobInput{
			VideoID:         videoID,
			JobType:         model.MediaJobType(request.JobType),
			SourceObjectKey: request.SourceObjectKey,
			IdempotencyKey:  request.IdempotencyKey,
		},
	)
	if err != nil {
		if errors.Is(err, service.ErrEngineUnavailable) && job != nil {
			writeJSON(
				w,
				http.StatusBadGateway,
				map[string]string{
					"error": "media engine unavailable",
					"jobId": job.ID.String(),
				},
			)
			return
		}

		writeServiceError(w, err)
		return
	}

	status := http.StatusCreated

	writeJSON(w, status, toJobResponse(job))
}

// Get returns a job the caller may manage.
func (h *MediaHandler) Get(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	id, ok := parseJobID(w, r)
	if !ok {
		return
	}

	job, err := h.jobs.GetJob(r.Context(), userID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toJobResponse(job))
}

// Refresh polls the engine and reconciles local state.
func (h *MediaHandler) Refresh(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	id, ok := parseJobID(w, r)
	if !ok {
		return
	}

	job, err := h.jobs.RefreshJob(r.Context(), userID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toJobResponse(job))
}

// Cancel cancels a queued/running job, engine first.
func (h *MediaHandler) Cancel(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	id, ok := parseJobID(w, r)
	if !ok {
		return
	}

	job, err := h.jobs.CancelJob(r.Context(), userID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toJobResponse(job))
}

// Retry creates a fresh job from a failed one; the original is kept.
func (h *MediaHandler) Retry(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	id, ok := parseJobID(w, r)
	if !ok {
		return
	}

	job, err := h.jobs.RetryJob(r.Context(), userID, id)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toJobResponse(job))
}

type historyResponse struct {
	Items      []jobResponse      `json:"items"`
	Pagination paginationResponse `json:"pagination"`
}

type paginationResponse struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

// HistoryByVideo lists a video's jobs, newest first. Ownership is always
// verified: job metadata belongs to the video's managers.
func (h *MediaHandler) HistoryByVideo(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeUnauthorized(w)
		return
	}

	videoID, err := uuid.Parse(chi.URLParam(r, "videoID"))
	if err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid video id",
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

	var jobType *model.MediaJobType

	if raw := query.Get("jobType"); raw != "" {
		parsed := model.MediaJobType(raw)

		switch parsed {
		case model.MediaJobVideoTranscode,
			model.MediaJobThumbnailGenerate,
			model.MediaJobVideoProbe:
			jobType = &parsed

		default:
			writeJSON(
				w,
				http.StatusBadRequest,
				map[string]string{
					"error": "invalid job type",
				},
			)
			return
		}
	}

	var status *model.MediaJobStatus

	if raw := query.Get("status"); raw != "" {
		parsed := model.MediaJobStatus(raw)

		switch parsed {
		case model.MediaJobQueued,
			model.MediaJobRunning,
			model.MediaJobCompleted,
			model.MediaJobFailed,
			model.MediaJobCancelled:
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

	pageOut, err := h.jobs.ListVideoJobs(
		r.Context(),
		userID,
		videoID,
		page,
		limit,
		jobType,
		status,
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	items := make([]jobResponse, 0, len(pageOut.Items))

	for _, job := range pageOut.Items {
		items = append(items, toJobResponse(job))
	}

	writeJSON(
		w,
		http.StatusOK,
		historyResponse{
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

func parseJobID(
	w http.ResponseWriter,
	r *http.Request,
) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		writeJSON(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "invalid job id",
			},
		)
		return uuid.Nil, false
	}

	return id, true
}

func toJobResponse(job *model.MediaJob) jobResponse {
	response := jobResponse{
		ID:                 job.ID.String(),
		VideoID:            job.VideoID.String(),
		JobType:            string(job.JobType),
		Status:             string(job.Status),
		Progress:           job.Progress,
		AttemptCount:       job.AttemptCount,
		SourceObjectKey:    job.SourceObjectKey,
		OutputManifestURL:  job.OutputManifestURL,
		OutputThumbnailURL: job.OutputThumbnailURL,
		DurationSeconds:    job.DurationSeconds,
		Width:              job.Width,
		Height:             job.Height,
		EngineJobID:        job.EngineJobID,
		CreatedAt:          job.CreatedAt,
		UpdatedAt:          job.UpdatedAt,
	}

	if job.ErrorCode != nil || job.ErrorMessage != nil {
		response.Error = &jobErrorResponse{}

		if job.ErrorCode != nil {
			response.Error.Code = *job.ErrorCode
		}

		if job.ErrorMessage != nil {
			response.Error.Message = *job.ErrorMessage
		}
	}

	return response
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
