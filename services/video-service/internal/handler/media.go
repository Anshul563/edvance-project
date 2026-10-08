package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/video-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/video-service/internal/model"
	"github.com/Anshul563/edvance-project/services/video-service/internal/service"
)

// MediaHandler exposes the upload pipeline over HTTP. All user-facing
// routes authenticate via the shared JWT secret and derive ownership
// from the token; the callback route is internal-only.
type MediaHandler struct {
	Service *service.MediaService
}

func NewMediaHandler(service *service.MediaService) *MediaHandler {
	return &MediaHandler{Service: service}
}

// Initiate creates the asset and issues a presigned PUT URL.
func (h *MediaHandler) Initiate(w http.ResponseWriter, r *http.Request) {
	var params service.InitUploadParams

	if err := readJSON(w, r, &params); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid request body",
		})

		return
	}

	ownerID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
		})

		return
	}

	result, err := h.Service.InitiateUpload(r.Context(), ownerID, params)
	if err != nil {
		writeServiceError(w, err)

		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// Complete confirms the object reached storage and queues processing.
func (h *MediaHandler) Complete(w http.ResponseWriter, r *http.Request) {
	assetID, ownerID, ok := h.loadOwnerAndAssetID(w, r)
	if !ok {
		return
	}

	view, err := h.Service.CompleteUpload(r.Context(), ownerID, assetID)
	if err != nil {
		writeServiceError(w, err)

		return
	}

	writeJSON(w, http.StatusOK, view)
}

// Get returns one asset plus its renditions.
func (h *MediaHandler) Get(w http.ResponseWriter, r *http.Request) {
	assetID, ownerID, ok := h.loadOwnerAndAssetID(w, r)
	if !ok {
		return
	}

	view, err := h.Service.Get(r.Context(), ownerID, assetID)
	if err != nil {
		writeServiceError(w, err)

		return
	}

	writeJSON(w, http.StatusOK, view)
}

// List returns an owner's assets, newest first.
func (h *MediaHandler) List(w http.ResponseWriter, r *http.Request) {
	ownerID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
		})

		return
	}

	status, ok := parseStatusQuery(w, r)
	if !ok {
		return
	}

	limit, ok := parseIntQuery(w, r, "limit", 20, 100, 1)
	if !ok {
		return
	}

	offset, ok := parseIntQuery(w, r, "offset", 0, 0, 0)
	if !ok {
		return
	}

	views, err := h.Service.List(r.Context(), ownerID, status, limit, offset)
	if err != nil {
		writeServiceError(w, err)

		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"assets": views,
	})
}

// Delete marks the asset deleted and schedules cleanup.
func (h *MediaHandler) Delete(w http.ResponseWriter, r *http.Request) {
	assetID, ownerID, ok := h.loadOwnerAndAssetID(w, r)
	if !ok {
		return
	}

	if err := h.Service.Delete(r.Context(), ownerID, assetID); err != nil {
		writeServiceError(w, err)

		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ProcessingCallback ingests engine results (internal-only route).
func (h *MediaHandler) ProcessingCallback(w http.ResponseWriter, r *http.Request) {
	var callback service.CallbackRequest

	if err := readJSON(w, r, &callback); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid callback body",
		})

		return
	}

	if err := h.Service.HandleCallback(r.Context(), callback); err != nil {
		writeServiceError(w, err)

		return
	}

	// Engines redeliver until they see 2xx; 202 signals accepted and
	// handled.
	w.WriteHeader(http.StatusAccepted)
}

func (h *MediaHandler) loadOwnerAndAssetID(
	w http.ResponseWriter,
	r *http.Request,
) (uuid.UUID, uuid.UUID, bool) {
	assetID, err := uuid.Parse(chi.URLParam(r, "mediaAssetID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid media asset id",
		})

		return uuid.Nil, uuid.Nil, false
	}

	ownerID, ok := middleware.GetUserID(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{
			"error": "unauthorized",
		})

		return uuid.Nil, uuid.Nil, false
	}

	return assetID, ownerID, true
}

func parseStatusQuery(
	w http.ResponseWriter,
	r *http.Request,
) (*model.AssetStatus, bool) {
	raw := r.URL.Query().Get("status")

	if raw == "" {
		return nil, true
	}

	status := model.AssetStatus(raw)

	switch status {
	case model.AssetStatusCreated, model.AssetStatusUploading,
		model.AssetStatusUploaded, model.AssetStatusProcessing,
		model.AssetStatusReady, model.AssetStatusFailed,
		model.AssetStatusDeleted:
		return &status, true
	}

	writeJSON(w, http.StatusBadRequest, map[string]string{
		"error": "invalid status filter",
	})

	return nil, false
}

// parseIntQuery reads a query integer with bounds. upper=0 means no
// upper bound.
func parseIntQuery(
	w http.ResponseWriter,
	r *http.Request,
	key string,
	fallback int,
	upper int,
	lower int,
) (int, bool) {
	raw := r.URL.Query().Get(key)

	if raw == "" {
		return fallback, true
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < lower || (upper != 0 && value > upper) {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "invalid " + key + " parameter",
		})

		return 0, false
	}

	return value, true
}
