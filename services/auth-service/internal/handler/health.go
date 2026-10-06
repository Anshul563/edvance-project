package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	DB    *pgxpool.Pool
	Redis *redis.Client
}

func NewHealthHandler(
	db *pgxpool.Pool,
	redisClient *redis.Client,
) *HealthHandler {
	return &HealthHandler{
		DB:    db,
		Redis: redisClient,
	}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte(`{"status":"ok","service":"auth-service"}`))
}

func (h *HealthHandler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.DB.Ping(ctx); err != nil {
		http.Error(
			w,
			`{"status":"not_ready","dependency":"postgres"}`,
			http.StatusServiceUnavailable,
		)
		return
	}

	if err := h.Redis.Ping(ctx).Err(); err != nil {
		http.Error(
			w,
			`{"status":"not_ready","dependency":"redis"}`,
			http.StatusServiceUnavailable,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte(`{"status":"ready","service":"auth-service"}`))
}
