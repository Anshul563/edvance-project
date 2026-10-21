package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
	"github.com/Anshul563/edvance-project/services/content-service/internal/service"
)

func strPtr(value string) *string {
	return &value
}

func TestCreatePassesLanguageAndCategory(t *testing.T) {
	stub := &stubVideos{}
	userID := uuid.New()

	req := httptest.NewRequest(
		http.MethodPost,
		"/videos",
		strings.NewReader(
			`{"title":"Valid title","language":"en-US","categorySlug":"science"}`,
		),
	)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userID))

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	if stub.gotInput.Language != "en-US" {
		t.Fatalf("language = %q", stub.gotInput.Language)
	}

	if stub.gotInput.CategorySlug == nil ||
		*stub.gotInput.CategorySlug != "science" {
		t.Fatalf("category = %v", stub.gotInput.CategorySlug)
	}
}

func TestCreateRejectsModerationFields(t *testing.T) {
	stub := &stubVideos{}
	userID := uuid.New()

	req := httptest.NewRequest(
		http.MethodPost,
		"/videos",
		strings.NewReader(
			`{"title":"Valid title","moderationStatus":"approved"}`,
		),
	)
	req.Header.Set("Authorization", "Bearer "+issueToken(t, userID))

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	if code := decodeError(t, rec).Code; code != "FIELD_NOT_SETTABLE" {
		t.Fatalf("expected FIELD_NOT_SETTABLE, got %s", code)
	}
}

func TestPlaybackReturnsProjection(t *testing.T) {
	videoID := uuid.New()

	stub := &stubVideos{
		playback: &service.Playback{
			MediaAssetID: uuid.NewString(),
			Status:       "ready",
			PlaybackURL:  strPtr("https://cdn.example/master.m3u8"),
			Captions:     []service.PlaybackCaption{},
		},
	}

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(
		rec,
		httptest.NewRequest(
			http.MethodGet,
			"/videos/"+videoID.String()+"/playback",
			nil,
		),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var body service.Playback

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.PlaybackURL == nil ||
		*body.PlaybackURL != "https://cdn.example/master.m3u8" {
		t.Fatalf("playback = %+v", body)
	}
}

func TestPlaybackBySlugUsesSlug(t *testing.T) {
	stub := &stubVideos{
		playback: &service.Playback{
			Status:   "ready",
			Captions: []service.PlaybackCaption{},
		},
	}

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(
		rec,
		httptest.NewRequest(http.MethodGet, "/videos/slug/intro/playback", nil),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if stub.gotSlug != "intro" {
		t.Fatalf("slug = %q", stub.gotSlug)
	}
}

func TestMediaAssetStatusCallback(t *testing.T) {
	stub := &stubVideos{
		video: &model.Video{
			ID:        uuid.New(),
			CreatorID: uuid.New(),
			Title:     "Video",
			Slug:      "video",
			Status:    model.VideoStatusReady,
		},
	}

	assetID := uuid.New()

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(
		rec,
		httptest.NewRequest(
			http.MethodPost,
			"/internal/media-assets/"+assetID.String()+"/status",
			strings.NewReader(`{"status":"failed","errorCode":"decode_failed","errorMessage":"bad file"}`),
		),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if stub.gotAssetKey != assetID {
		t.Fatalf("asset key = %s", stub.gotAssetKey)
	}

	if stub.gotStatus != string(model.VideoStatusFailed) {
		t.Fatalf("status = %q", stub.gotStatus)
	}
}

func TestModerationRoute(t *testing.T) {
	videoID := uuid.New()

	stub := &stubVideos{
		video: &model.Video{
			ID:               videoID,
			CreatorID:        uuid.New(),
			Title:            "Video",
			Slug:             "video",
			ModerationStatus: model.ModerationRejected,
		},
	}

	rec := httptest.NewRecorder()
	videoTestRouter(stub).ServeHTTP(
		rec,
		httptest.NewRequest(
			http.MethodPost,
			"/internal/videos/"+videoID.String()+"/moderation",
			strings.NewReader(`{"status":"rejected","reason":"copyright"}`),
		),
	)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if stub.gotStatus != string(model.ModerationRejected) {
		t.Fatalf("status = %q", stub.gotStatus)
	}

	if stub.gotReason == nil || *stub.gotReason != "copyright" {
		t.Fatalf("reason = %v", stub.gotReason)
	}
}
