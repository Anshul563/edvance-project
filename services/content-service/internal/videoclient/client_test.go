package videoclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/service"
)

func TestPlaybackSendsInternalKeyAndDecodes(t *testing.T) {
	assetID := uuid.New()

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("X-Internal-Key") != "secret" {
				t.Fatalf("missing internal key: %q", r.Header.Get("X-Internal-Key"))
			}

			if r.URL.Path != "/internal/v1/videos/"+assetID.String()+"/playback" {
				t.Fatalf("path = %q", r.URL.Path)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"mediaAssetId":"` + assetID.String() + `",
				"status":"ready",
				"playbackUrl":"https://cdn.example/master.m3u8",
				"captions":[{"language":"en","label":"English","format":"vtt"}]
			}`))
		},
	))
	defer server.Close()

	client := New(server.URL, "secret")

	playback, err := client.Playback(context.Background(), assetID)
	if err != nil {
		t.Fatalf("playback: %v", err)
	}

	if playback.PlaybackURL == nil ||
		*playback.PlaybackURL != "https://cdn.example/master.m3u8" {
		t.Fatalf("playback = %+v", playback)
	}

	if len(playback.Captions) != 1 {
		t.Fatalf("captions = %+v", playback.Captions)
	}
}

func TestPlaybackMapsNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		},
	))
	defer server.Close()

	client := New(server.URL, "secret")

	if _, err := client.Playback(
		context.Background(),
		uuid.New(),
	); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPlaybackWithoutURLIsUnavailable(t *testing.T) {
	client := New("", "secret")

	if _, err := client.Playback(
		context.Background(),
		uuid.New(),
	); !errors.Is(err, service.ErrMediaUnavailable) {
		t.Fatalf("expected ErrMediaUnavailable, got %v", err)
	}
}
