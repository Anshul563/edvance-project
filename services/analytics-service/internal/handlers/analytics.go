package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Anshul563/edvance-project/services/analytics-service/internal/middleware"
	"github.com/Anshul563/edvance-project/services/analytics-service/internal/models"
	"github.com/Anshul563/edvance-project/services/analytics-service/internal/services"
)

type AnalyticsHandler struct {
	service *services.AnalyticsService
	limit   int
}

func NewAnalyticsHandler(service *services.AnalyticsService, limit int) *AnalyticsHandler {
	return &AnalyticsHandler{service: service, limit: limit}
}

func (h *AnalyticsHandler) IngestEvent(w http.ResponseWriter, r *http.Request) {
	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}
	if err := h.service.IngestEvent(r.Context(), event); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write([]byte(`{"status":"accepted"}`))
}

func (h *AnalyticsHandler) IngestBatch(w http.ResponseWriter, r *http.Request) {
	var request models.BatchEventRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}
	if len(request.Events) == 0 {
		http.Error(w, "events list is required", http.StatusBadRequest)
		return
	}
	if len(request.Events) > h.limit {
		http.Error(w, "batch exceeds maximum supported size", http.StatusBadRequest)
		return
	}
	result, err := h.service.IngestBatch(r.Context(), request.Events)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func (h *AnalyticsHandler) Overview(w http.ResponseWriter, r *http.Request) {
	actorID, err := middleware.UserIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	from, to, err := parseRange(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.service.Overview(r.Context(), actorID, from, to)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(result)
}

func (h *AnalyticsHandler) CreatorOverview(w http.ResponseWriter, r *http.Request) {
	actorID, err := middleware.UserIDFromContext(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	from, to, err := parseRange(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.service.CreatorOverview(r.Context(), actorID, from, to)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(result)
}

func (h *AnalyticsHandler) PlatformOverview(w http.ResponseWriter, r *http.Request) {
	role, err := middleware.RoleFromContext(r.Context())
	if err != nil || role != "internal" {
		if role != "internal" {
			http.Error(w, "administrator access required", http.StatusForbidden)
			return
		}
	}
	from, to, err := parseRange(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.service.PlatformOverview(r.Context(), from, to)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(result)
}

func (h *AnalyticsHandler) CourseOverview(w http.ResponseWriter, r *http.Request) {
	courseID := chi.URLParam(r, "courseID")
	from, to, err := parseRange(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	result, err := h.service.CourseOverview(r.Context(), courseID, from, to)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func parseRange(r *http.Request) (time.Time, time.Time, error) {
	fromRaw := strings.TrimSpace(r.URL.Query().Get("from"))
	toRaw := strings.TrimSpace(r.URL.Query().Get("to"))
	if fromRaw == "" {
		fromRaw = time.Now().Add(-24 * 7 * time.Hour).Format(time.RFC3339)
	}
	if toRaw == "" {
		toRaw = time.Now().Format(time.RFC3339)
	}
	from, err := time.Parse(time.RFC3339, fromRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	to, err := time.Parse(time.RFC3339, toRaw)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, strconv.ErrSyntax
	}
	return from.UTC(), to.UTC(), nil
}
