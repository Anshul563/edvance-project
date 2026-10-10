package router

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/Anshul563/edvance-project/services/admin-api/internal/config"
	adminmw "github.com/Anshul563/edvance-project/services/admin-api/internal/middleware"
)

func New(cfg config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":    "ok",
			"service":   "admin-api",
			"timestamp": time.Now().UTC(),
		})
	})

	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{
				"status":    "ok",
				"service":   "admin-api",
				"timestamp": time.Now().UTC(),
			})
		})

		r.With(adminmw.Middleware(cfg)).Get("/overview", func(w http.ResponseWriter, r *http.Request) {
			userID, _ := adminmw.UserIDFromContext(r.Context())
			statuses := collectServiceStatuses(cfg)
			overall := "ok"
			for _, item := range statuses {
				if item["status"] != "ok" {
					overall = "degraded"
					break
				}
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"service":    "admin-api",
				"actor_user": userID,
				"status":     overall,
				"services":   statuses,
			})
		})

		r.With(adminmw.Middleware(cfg)).Get("/services", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{
				"services": collectServiceStatuses(cfg),
			})
		})

		r.With(adminmw.Middleware(cfg)).Get("/audit-logs", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{
				"items": []map[string]any{},
				"page":  1,
				"count": 0,
			})
		})
	})

	return r
}

func collectServiceStatuses(cfg config.Config) []map[string]any {
	client := &http.Client{Timeout: time.Duration(cfg.ServiceTimeoutSeconds) * time.Second}
	services := make([]map[string]any, 0, len(cfg.AllowedServiceHealth))
	for _, name := range cfg.AllowedServiceHealth {
		baseURL := strings.TrimSpace(cfg.ServiceURLs[name])
		if baseURL == "" {
			services = append(services, map[string]any{"name": name, "status": "unknown"})
			continue
		}
		status := "ok"
		if !serviceHealthy(client, baseURL) {
			status = "down"
		}
		services = append(services, map[string]any{"name": name, "status": status, "url": baseURL})
	}
	return services
}

func serviceHealthy(client *http.Client, baseURL string) bool {
	for _, url := range []string{
		strings.TrimRight(baseURL, "/") + "/health",
		strings.TrimRight(baseURL, "/") + "/healthz",
	} {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusBadRequest {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func normalizeServiceName(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}
