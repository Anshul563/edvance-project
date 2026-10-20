package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// HealthHandler exposes liveness and readiness endpoints. Readiness
// pings PostgreSQL so orchestrators do not route traffic before the
// database is reachable.
type HealthHandler struct {
	DB *pgxpool.Pool
}

func NewHealthHandler(db *pgxpool.Pool) *HealthHandler {
	return &HealthHandler{DB: db}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "live-streaming-service"})
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	if h.DB == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status":     "not_ready",
			"dependency": "postgres",
		})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.DB.Ping(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status":     "not_ready",
			"dependency": "postgres",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "service": "live-streaming-service"})
}
