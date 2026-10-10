package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"

	"github.com/Anshul563/edvance-project/services/admin-api/internal/config"
)

func TestOverviewReturnsLiveServiceStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" || r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := config.Config{
		JWT: config.JWTConfig{
			AccessSecret: "super-secret",
			Issuer:       "edvance-auth",
			Audience:     "edvance-api",
		},
		AllowedUserIDs:        map[string]struct{}{"11111111-1111-4111-8111-111111111111": {}},
		AllowedServiceHealth:  []string{"auth"},
		ServiceTimeoutSeconds: 2,
		ServiceURLs:           map[string]string{"auth": server.URL},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   "11111111-1111-4111-8111-111111111111",
		Issuer:    "edvance-auth",
		Audience:  jwt.ClaimStrings{"edvance-api"},
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	signed, err := token.SignedString([]byte("super-secret"))
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/overview", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	w := httptest.NewRecorder()

	New(cfg).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var payload map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("status = %v, want ok", payload["status"])
	}
	services, ok := payload["services"].([]any)
	if !ok || len(services) != 1 {
		t.Fatalf("unexpected services payload: %#v", payload["services"])
	}
	service, ok := services[0].(map[string]any)
	if !ok {
		t.Fatalf("service item missing: %#v", services[0])
	}
	if service["status"] != "ok" {
		t.Fatalf("service status = %v, want ok", service["status"])
	}
}
