package contentclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/video-service/internal/service"
)

func TestNotifyPostsStatusWithBearerToken(t *testing.T) {
	assetID := uuid.New()
	duration := int64(90)

	var received statusPayload

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer secret" {
				t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
			}

			if r.URL.Path != "/internal/media-assets/"+assetID.String()+"/status" {
				t.Fatalf("path = %q", r.URL.Path)
			}

			if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
				t.Fatalf("decode: %v", err)
			}

			w.WriteHeader(http.StatusOK)
		},
	))
	defer server.Close()

	client := New(server.URL, "secret")

	if err := client.Notify(context.Background(), service.StatusEvent{
		MediaAssetID:    assetID,
		Status:          "ready",
		DurationSeconds: &duration,
	}); err != nil {
		t.Fatalf("notify: %v", err)
	}

	if received.Status != "ready" {
		t.Fatalf("status = %q", received.Status)
	}

	if received.DurationSeconds == nil || *received.DurationSeconds != 90 {
		t.Fatalf("duration = %v", received.DurationSeconds)
	}
}

func TestNotifyTreatsMissingContentRowAsDelivered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
	))
	defer server.Close()

	client := New(server.URL, "secret")

	if err := client.Notify(context.Background(), service.StatusEvent{
		MediaAssetID: uuid.New(),
		Status:       "uploaded",
	}); err != nil {
		t.Fatalf("404 must be a no-op, got %v", err)
	}
}

func TestNotifyDisabledWithoutURL(t *testing.T) {
	client := New("", "secret")

	if err := client.Notify(context.Background(), service.StatusEvent{
		MediaAssetID: uuid.New(),
		Status:       "uploaded",
	}); err != nil {
		t.Fatalf("disabled notifier returned %v", err)
	}
}
