package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
)

func publishVideo(t *testing.T, e *env, video *model.Video) *model.Video {
	t.Helper()

	published, err := e.videoService.Publish(
		context.Background(),
		e.actor,
		video.ID,
	)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	return published
}

func TestCreateVideoLanguageAndCategory(t *testing.T) {
	e := newEnv()

	category := &model.Category{
		ID:   uuid.New(),
		Name: "Science",
		Slug: "science",
	}
	e.categories.seed(category)

	video := mustCreateVideo(t, e, e.actor, CreateVideoInput{
		Title:        "Introduction to Physics",
		Language:     "en-US",
		CategorySlug: strPtr("science"),
	})

	if video.Language != "en-US" {
		t.Fatalf("language = %q", video.Language)
	}

	if video.CategoryID == nil || *video.CategoryID != category.ID {
		t.Fatalf("category = %v", video.CategoryID)
	}

	if video.ModerationStatus != model.ModerationApproved {
		t.Fatalf("moderation = %q", video.ModerationStatus)
	}
}

func TestCreateVideoRejectsUnknownCategoryAndBadLanguage(t *testing.T) {
	e := newEnv()

	if _, err := e.videoService.Create(
		context.Background(),
		e.actor,
		CreateVideoInput{
			Title:        "Some video",
			CategorySlug: strPtr("missing"),
		},
	); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for unknown category, got %v", err)
	}

	if _, err := e.videoService.Create(
		context.Background(),
		e.actor,
		CreateVideoInput{Title: "Some video", Language: "not a tag!"},
	); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for language, got %v", err)
	}
}

func TestCreateVideoWithAssetStartsUploadPending(t *testing.T) {
	e := newEnv()

	mediaAssetID := uuid.New()

	video := mustCreateVideo(t, e, e.actor, CreateVideoInput{
		Title:        "Uploaded later",
		MediaAssetID: &mediaAssetID,
	})

	if video.Status != model.VideoStatusUploadPending {
		t.Fatalf("status = %q", video.Status)
	}
}

func TestPublishRequiresApprovedModeration(t *testing.T) {
	e := newEnv()

	video := readyVideo(t, e, e.actor, "Moderated video")

	if _, err := e.videoService.SetModeration(
		context.Background(),
		video.ID,
		"rejected",
		strPtr("not educational"),
	); err != nil {
		t.Fatalf("set moderation: %v", err)
	}

	if _, err := e.videoService.Publish(
		context.Background(),
		e.actor,
		video.ID,
	); !errors.Is(err, ErrNotPublishable) {
		t.Fatalf("expected ErrNotPublishable, got %v", err)
	}
}

func TestSetModerationUnpublishesRejectedVideo(t *testing.T) {
	e := newEnv()

	video := readyVideo(t, e, e.actor, "Published then rejected")
	published := publishVideo(t, e, video)

	rejected, err := e.videoService.SetModeration(
		context.Background(),
		published.ID,
		"rejected",
		strPtr("copyright"),
	)
	if err != nil {
		t.Fatalf("set moderation: %v", err)
	}

	if rejected.Status != model.VideoStatusReady {
		t.Fatalf("status = %q, want ready", rejected.Status)
	}

	if rejected.Visibility != model.VideoVisibilityPrivate {
		t.Fatalf("visibility = %q", rejected.Visibility)
	}

	if rejected.PublishedAt != nil {
		t.Fatal("rejected video must not keep published_at")
	}

	page, err := e.videoService.ListVideos(
		context.Background(),
		Anonymous(),
		ListVideosParams{},
	)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if page.Total != 0 {
		t.Fatalf("rejected video leaked into public discovery: %d", page.Total)
	}
}

func TestSetModerationRejectsUnknownStatus(t *testing.T) {
	e := newEnv()

	video := readyVideo(t, e, e.actor, "Video")

	if _, err := e.videoService.SetModeration(
		context.Background(),
		video.ID,
		"maybe",
		nil,
	); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestDiscoveryExcludesHeldAndIncludesApproved(t *testing.T) {
	e := newEnv()

	approved := publishVideo(t, e, readyVideo(t, e, e.actor, "Approved video"))

	held := readyVideo(t, e, e.actor, "Held video")
	publishVideo(t, e, held)

	if _, err := e.videoService.SetModeration(
		context.Background(),
		held.ID,
		"held",
		nil,
	); err != nil {
		t.Fatalf("hold: %v", err)
	}

	page, err := e.videoService.ListVideos(
		context.Background(),
		Anonymous(),
		ListVideosParams{},
	)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if page.Total != 1 || page.Items[0].ID != approved.ID {
		t.Fatalf("unexpected public page: %+v", page.Items)
	}
}

func TestSetMediaStatusByMediaAsset(t *testing.T) {
	e := newEnv()

	mediaAssetID := uuid.New()

	video := mustCreateVideo(t, e, e.actor, CreateVideoInput{
		Title:        "Pipeline video",
		MediaAssetID: &mediaAssetID,
	})

	updated, err := e.videoService.SetMediaStatusByMediaAsset(
		context.Background(),
		mediaAssetID,
		MediaStatusUpdate{
			Status:       string(model.VideoStatusFailed),
			MediaAssetID: &mediaAssetID,
			ErrorCode:    strPtr("decode_failed"),
			ErrorMessage: strPtr("cannot decode"),
		},
	)
	if err != nil {
		t.Fatalf("set status: %v", err)
	}

	if len(updated) != 1 || updated[0].ID != video.ID {
		t.Fatalf("updated = %+v", updated)
	}

	if updated[0].Status != model.VideoStatusFailed {
		t.Fatalf("status = %q", updated[0].Status)
	}

	if updated[0].ProcessingErrorCode == nil ||
		*updated[0].ProcessingErrorCode != "decode_failed" {
		t.Fatalf("error code = %v", updated[0].ProcessingErrorCode)
	}

	if _, err := e.videoService.SetMediaStatusByMediaAsset(
		context.Background(),
		uuid.New(),
		MediaStatusUpdate{Status: string(model.VideoStatusProcessing)},
	); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestSetMediaStatusByAssetSkipsPublished(t *testing.T) {
	e := newEnv()

	video := readyVideo(t, e, e.actor, "Live video")
	published := publishVideo(t, e, video)
	mediaAssetID := *published.MediaAssetID

	if _, err := e.videoService.SetMediaStatusByMediaAsset(
		context.Background(),
		mediaAssetID,
		MediaStatusUpdate{
			Status:       string(model.VideoStatusFailed),
			MediaAssetID: &mediaAssetID,
		},
	); err != nil {
		t.Fatalf("late callback must not fail: %v", err)
	}

	reloaded, err := e.videoService.Get(
		context.Background(),
		e.actor,
		published.ID,
	)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if reloaded.Status != model.VideoStatusPublished {
		t.Fatalf("published row was mutated: %q", reloaded.Status)
	}
}

func TestPlaybackAccessRules(t *testing.T) {
	e := newEnv()

	video := readyVideo(t, e, e.actor, "Playable video")
	published := publishVideo(t, e, video)

	playback := &Playback{
		MediaAssetID: published.MediaAssetID.String(),
		Status:       "ready",
		PlaybackURL:  strPtr("https://cdn.example/master.m3u8"),
		Captions:     []PlaybackCaption{},
	}
	e.playback.set(*published.MediaAssetID, playback)

	anonymous, err := e.videoService.Playback(
		context.Background(),
		Anonymous(),
		published.ID,
	)
	if err != nil {
		t.Fatalf("anonymous playback: %v", err)
	}

	if anonymous.PlaybackURL == nil ||
		*anonymous.PlaybackURL != "https://cdn.example/master.m3u8" {
		t.Fatalf("playback = %+v", anonymous)
	}

	private := readyVideo(t, e, e.actor, "Private draft")
	e.playback.set(*private.MediaAssetID, &Playback{
		MediaAssetID: private.MediaAssetID.String(),
		Status:       "ready",
		PlaybackURL:  strPtr("https://cdn.example/private.m3u8"),
		Captions:     []PlaybackCaption{},
	})

	other := e.secondCreator()

	if _, err := e.videoService.Playback(
		context.Background(),
		other,
		private.ID,
	); !errors.Is(err, ErrNotFound) {
		t.Fatalf("private draft leaked to other user: %v", err)
	}

	if _, err := e.videoService.Playback(
		context.Background(),
		e.actor,
		private.ID,
	); err != nil {
		t.Fatalf("owner preview: %v", err)
	}
}

func TestUnlistedPublishedVideoIsPlayableByLinkOnly(t *testing.T) {
	e := newEnv()

	video := mustCreateVideo(t, e, e.actor, CreateVideoInput{
		Title:      "Unlisted lecture",
		Visibility: "unlisted",
	})

	mediaAssetID := uuid.New()

	if _, err := e.videoService.SetMediaStatus(
		context.Background(),
		video.ID,
		MediaStatusUpdate{
			Status:       string(model.VideoStatusReady),
			MediaAssetID: &mediaAssetID,
		},
	); err != nil {
		t.Fatalf("set ready: %v", err)
	}

	published := publishVideo(t, e, video)

	if published.Visibility != model.VideoVisibilityUnlisted {
		t.Fatalf("visibility = %q", published.Visibility)
	}

	e.playback.set(mediaAssetID, &Playback{
		MediaAssetID: mediaAssetID.String(),
		Status:       "ready",
		PlaybackURL:  strPtr("https://cdn.example/unlisted.m3u8"),
		Captions:     []PlaybackCaption{},
	})

	if _, err := e.videoService.Playback(
		context.Background(),
		Anonymous(),
		published.ID,
	); err != nil {
		t.Fatalf("unlisted direct-link playback: %v", err)
	}

	page, err := e.videoService.ListVideos(
		context.Background(),
		Anonymous(),
		ListVideosParams{},
	)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	for _, item := range page.Items {
		if item.ID == published.ID {
			t.Fatal("unlisted video leaked into public discovery")
		}
	}
}

func TestPlaybackRequiresMediaAsset(t *testing.T) {
	e := newEnv()

	video := mustCreateVideo(t, e, e.actor, CreateVideoInput{
		Title: "No media yet",
	})

	if _, err := e.videoService.Playback(
		context.Background(),
		e.actor,
		video.ID,
	); !errors.Is(err, ErrNotPublishable) {
		t.Fatalf("expected ErrNotPublishable, got %v", err)
	}
}

func TestPlaybackUnavailableWhenClientMissing(t *testing.T) {
	e := newEnv()
	e.videoService.playback = nil

	video := readyVideo(t, e, e.actor, "No playback client")

	if _, err := e.videoService.Playback(
		context.Background(),
		e.actor,
		video.ID,
	); !errors.Is(err, ErrMediaUnavailable) {
		t.Fatalf("expected ErrMediaUnavailable, got %v", err)
	}
}

func TestDiscoveryFiltersAndSort(t *testing.T) {
	e := newEnv()

	category := &model.Category{
		ID:   uuid.New(),
		Name: "Mathematics",
		Slug: "mathematics",
	}
	e.categories.seed(category)

	first := mustCreateVideo(t, e, e.actor, CreateVideoInput{
		Title:        "Algebra basics",
		Language:     "en",
		CategorySlug: strPtr("mathematics"),
		Tags:         []string{"algebra"},
	})
	e.videos.linkCategory(*first.CategoryID, "mathematics")

	mediaAssetID := uuid.New()

	if _, err := e.videoService.SetMediaStatus(
		context.Background(),
		first.ID,
		MediaStatusUpdate{
			Status:       string(model.VideoStatusReady),
			MediaAssetID: &mediaAssetID,
		},
	); err != nil {
		t.Fatalf("set ready: %v", err)
	}

	publishVideo(t, e, first)

	second := mustCreateVideo(t, e, e.actor, CreateVideoInput{
		Title:    "Spanish conversation",
		Language: "es",
		Tags:     []string{"spanish"},
	})

	secondAssetID := uuid.New()

	if _, err := e.videoService.SetMediaStatus(
		context.Background(),
		second.ID,
		MediaStatusUpdate{
			Status:       string(model.VideoStatusReady),
			MediaAssetID: &secondAssetID,
		},
	); err != nil {
		t.Fatalf("set ready: %v", err)
	}

	publishVideo(t, e, second)

	cases := []struct {
		name   string
		params ListVideosParams
		wantID uuid.UUID
	}{
		{"by language", ListVideosParams{Language: strPtr("es")}, second.ID},
		{"by tag", ListVideosParams{TagSlug: strPtr("algebra")}, first.ID},
		{"by category", ListVideosParams{CategorySlug: strPtr("mathematics")}, first.ID},
		{"by query", ListVideosParams{Query: strPtr("algebra")}, first.ID},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			page, err := e.videoService.ListVideos(
				context.Background(),
				Anonymous(),
				testCase.params,
			)
			if err != nil {
				t.Fatalf("list: %v", err)
			}

			if page.Total != 1 || page.Items[0].ID != testCase.wantID {
				t.Fatalf("unexpected page: %+v", page.Items)
			}
		})
	}

	if _, err := e.videoService.ListVideos(
		context.Background(),
		Anonymous(),
		ListVideosParams{Sort: "sideways"},
	); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for bad sort, got %v", err)
	}
}

func TestGetBySlugHidesNonPublic(t *testing.T) {
	e := newEnv()

	video := mustCreateVideo(t, e, e.actor, CreateVideoInput{
		Title: "Slug lookup video",
	})

	if _, err := e.videoService.GetBySlug(
		context.Background(),
		Anonymous(),
		video.Slug,
	); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	if _, err := e.videoService.GetBySlug(
		context.Background(),
		e.actor,
		video.Slug,
	); err != nil {
		t.Fatalf("owner lookup: %v", err)
	}
}

func strPtr(value string) *string {
	return &value
}
