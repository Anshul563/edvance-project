package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/creatorclient"
	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
	"github.com/Anshul563/edvance-project/services/content-service/internal/notifier"
	"github.com/Anshul563/edvance-project/services/content-service/internal/repository"
)

func mustCreateVideo(
	t *testing.T,
	e *env,
	actor Actor,
	input CreateVideoInput,
) *model.Video {
	t.Helper()

	video, err := e.videoService.Create(context.Background(), actor, input)
	if err != nil {
		t.Fatalf("create video: %v", err)
	}

	return video
}

// readyVideo creates a video and drives it through the pipeline to the
// state publish requires: status ready plus a media asset.
func readyVideo(t *testing.T, e *env, actor Actor, title string) *model.Video {
	t.Helper()

	video := mustCreateVideo(t, e, actor, CreateVideoInput{
		Title:      title,
		Visibility: "private",
	})

	mediaAssetID := uuid.New()

	if _, err := e.videoService.SetMediaStatus(
		context.Background(),
		video.ID,
		string(model.VideoStatusReady),
		intPtr(120),
		&mediaAssetID,
	); err != nil {
		t.Fatalf("set media status: %v", err)
	}

	updated, err := e.videoService.Get(
		context.Background(),
		actor,
		video.ID,
	)
	if err != nil {
		t.Fatalf("reload video: %v", err)
	}

	return updated
}

func intPtr(value int) *int {
	return &value
}

func TestCreateRequiresAuthenticatedActor(t *testing.T) {
	e := newEnv()

	_, err := e.videoService.Create(
		context.Background(),
		Anonymous(),
		CreateVideoInput{Title: "Some title"},
	)

	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestCreateWithoutCreatorProfile(t *testing.T) {
	e := newEnv()

	stranger := Actor{UserID: uuid.New(), AccessToken: "token"}

	_, err := e.videoService.Create(
		context.Background(),
		stranger,
		CreateVideoInput{Title: "Some title"},
	)

	if !errors.Is(err, ErrNoCreatorProfile) {
		t.Fatalf("expected ErrNoCreatorProfile, got %v", err)
	}
}

func TestCreateWhenCreatorServiceDown(t *testing.T) {
	e := newEnv()
	e.resolver.fail(creatorclient.ErrUnavailable)

	_, err := e.videoService.Create(
		context.Background(),
		e.actor,
		CreateVideoInput{Title: "Some title"},
	)

	if !errors.Is(err, ErrCreatorUnavailable) {
		t.Fatalf("expected ErrCreatorUnavailable, got %v", err)
	}
}

func TestCreateStoresDraftWithGeneratedSlug(t *testing.T) {
	e := newEnv()

	video := mustCreateVideo(t, e, e.actor, CreateVideoInput{
		Title:      "  My First  Video! ",
		Visibility: "public",
		Tags:       []string{"Go", "  go "},
	})

	if video.Status != model.VideoStatusDraft {
		t.Fatalf("expected draft, got %q", video.Status)
	}

	if video.Slug != "my-first-video" {
		t.Fatalf("expected slug my-first-video, got %q", video.Slug)
	}

	if video.CreatorID != e.creatorID {
		t.Fatalf("expected creator %s, got %s", e.creatorID, video.CreatorID)
	}

	if video.Visibility != model.VideoVisibilityPublic {
		t.Fatalf("expected public, got %q", video.Visibility)
	}

	if video.PublishedAt != nil {
		t.Fatal("draft must not carry published_at")
	}

	if len(video.Tags) != 1 || video.Tags[0].Name != "go" {
		t.Fatalf("expected normalized tags, got %+v", video.Tags)
	}
}

func TestCreateValidation(t *testing.T) {
	e := newEnv()

	cases := []struct {
		name  string
		input CreateVideoInput
		want  error
	}{
		{"short title", CreateVideoInput{Title: "ab"}, ErrInvalidInput},
		{"bad visibility", CreateVideoInput{Title: "Valid", Visibility: "friends"}, ErrInvalidVisibility},
		{"unusable tag", CreateVideoInput{Title: "Valid", Tags: []string{"!!!"}}, ErrInvalidInput},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := e.videoService.Create(
				context.Background(),
				e.actor,
				tc.input,
			)

			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestCreateRetriesOnSlugCollision(t *testing.T) {
	e := newEnv()

	first := mustCreateVideo(t, e, e.actor, CreateVideoInput{Title: "Same Title"})
	second := mustCreateVideo(t, e, e.actor, CreateVideoInput{Title: "Same Title"})

	if first.Slug != "same-title" {
		t.Fatalf("expected same-title, got %q", first.Slug)
	}

	if second.Slug != "same-title-2" {
		t.Fatalf("expected same-title-2, got %q", second.Slug)
	}
}

func TestGetHidesDraftFromEveryoneButOwner(t *testing.T) {
	e := newEnv()

	video := mustCreateVideo(t, e, e.actor, CreateVideoInput{Title: "Draft video"})
	other := e.secondCreator()

	if _, err := e.videoService.Get(
		context.Background(),
		Anonymous(),
		video.ID,
	); !errors.Is(err, ErrNotFound) {
		t.Fatalf("anonymous read must 404, got %v", err)
	}

	if _, err := e.videoService.Get(
		context.Background(),
		other,
		video.ID,
	); !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-owner read must 404, got %v", err)
	}

	if _, err := e.videoService.Get(
		context.Background(),
		e.actor,
		video.ID,
	); err != nil {
		t.Fatalf("owner read must succeed, got %v", err)
	}
}

func TestGetPublishedVideoIsPublic(t *testing.T) {
	e := newEnv()

	video := readyVideo(t, e, e.actor, "Published video")

	if _, err := e.videoService.Publish(
		context.Background(),
		e.actor,
		video.ID,
	); err != nil {
		t.Fatalf("publish: %v", err)
	}

	loaded, err := e.videoService.Get(
		context.Background(),
		Anonymous(),
		video.ID,
	)
	if err != nil {
		t.Fatalf("anonymous read of published video: %v", err)
	}

	if loaded.Status != model.VideoStatusPublished {
		t.Fatalf("expected published, got %q", loaded.Status)
	}
}

func TestReadDegradesWhenCreatorServiceDown(t *testing.T) {
	e := newEnv()

	video := readyVideo(t, e, e.actor, "Published video")

	if _, err := e.videoService.Publish(
		context.Background(),
		e.actor,
		video.ID,
	); err != nil {
		t.Fatalf("publish: %v", err)
	}

	draft := mustCreateVideo(t, e, e.actor, CreateVideoInput{Title: "Draft video"})

	e.resolver.fail(creatorclient.ErrUnavailable)

	// Published rows stay readable; the viewer is simply anonymous.
	if _, err := e.videoService.Get(
		context.Background(),
		e.actor,
		video.ID,
	); err != nil {
		t.Fatalf("published read must survive an outage: %v", err)
	}

	// The owner's own draft reads as 404 because ownership cannot be
	// established — never a 500, never a leak.
	if _, err := e.videoService.Get(
		context.Background(),
		e.actor,
		draft.ID,
	); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound during outage, got %v", err)
	}
}

func TestUpdateForbiddenForNonOwner(t *testing.T) {
	e := newEnv()

	video := mustCreateVideo(t, e, e.actor, CreateVideoInput{Title: "Owner video"})
	other := e.secondCreator()

	_, err := e.videoService.Update(
		context.Background(),
		other,
		video.ID,
		UpdateVideoInput{Title: stringPtr("Hijacked")},
	)

	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestUpdateRejectsAnonymous(t *testing.T) {
	e := newEnv()

	video := mustCreateVideo(t, e, e.actor, CreateVideoInput{Title: "Owner video"})

	_, err := e.videoService.Update(
		context.Background(),
		Anonymous(),
		video.ID,
		UpdateVideoInput{Title: stringPtr("Hijacked")},
	)

	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestUpdateAppliesOwnerEdits(t *testing.T) {
	e := newEnv()

	video := mustCreateVideo(t, e, e.actor, CreateVideoInput{Title: "Before edit"})

	visibility := "unlisted"

	updated, err := e.videoService.Update(
		context.Background(),
		e.actor,
		video.ID,
		UpdateVideoInput{
			Title:      stringPtr("After edit"),
			Visibility: &visibility,
		},
	)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Title != "After edit" {
		t.Fatalf("expected new title, got %q", updated.Title)
	}

	if updated.Visibility != model.VideoVisibilityUnlisted {
		t.Fatalf("expected unlisted, got %q", updated.Visibility)
	}

	if updated.Slug != video.Slug {
		t.Fatalf("slug must not change on edit: %q -> %q", video.Slug, updated.Slug)
	}
}

func TestDeleteForbiddenForNonOwner(t *testing.T) {
	e := newEnv()

	video := mustCreateVideo(t, e, e.actor, CreateVideoInput{Title: "Owner video"})
	other := e.secondCreator()

	if err := e.videoService.Delete(
		context.Background(),
		other,
		video.ID,
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	if err := e.videoService.Delete(
		context.Background(),
		e.actor,
		video.ID,
	); err != nil {
		t.Fatalf("owner delete: %v", err)
	}

	if _, err := e.videos.FindByID(context.Background(), video.ID); err == nil {
		t.Fatal("video should be gone")
	}
}

func TestPublishGateRejectsIncompleteVideo(t *testing.T) {
	e := newEnv()

	draft := mustCreateVideo(t, e, e.actor, CreateVideoInput{Title: "Draft only"})

	_, err := e.videoService.Publish(context.Background(), e.actor, draft.ID)
	if !errors.Is(err, ErrNotPublishable) {
		t.Fatalf("expected ErrNotPublishable, got %v", err)
	}

	message := err.Error()
	for _, want := range []string{"status", "media asset"} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected reason %q in %q", want, message)
		}
	}
}

func TestPublishRequiresReadyStatus(t *testing.T) {
	e := newEnv()

	video := mustCreateVideo(t, e, e.actor, CreateVideoInput{
		Title:        "Has media",
		MediaAssetID: uuidPtr(uuid.New()),
	})

	_, err := e.videoService.Publish(context.Background(), e.actor, video.ID)
	if !errors.Is(err, ErrNotPublishable) {
		t.Fatalf("expected ErrNotPublishable, got %v", err)
	}
}

func TestPublishForbiddenForNonOwner(t *testing.T) {
	e := newEnv()

	video := readyVideo(t, e, e.actor, "Owner video")
	other := e.secondCreator()

	_, err := e.videoService.Publish(context.Background(), other, video.ID)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestPublishSucceedsAndNotifies(t *testing.T) {
	e := newEnv()

	video := readyVideo(t, e, e.actor, "Ready video")

	published, err := e.videoService.Publish(
		context.Background(),
		e.actor,
		video.ID,
	)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	if published.Status != model.VideoStatusPublished {
		t.Fatalf("expected published, got %q", published.Status)
	}

	if published.Visibility != model.VideoVisibilityPublic {
		t.Fatalf("expected public, got %q", published.Visibility)
	}

	if published.PublishedAt == nil {
		t.Fatal("published_at must be set")
	}

	types := e.notifier.types()
	if len(types) != 1 || types[0] != notifier.EventContentPublished {
		t.Fatalf("expected one content.published event, got %v", types)
	}
}

func TestPublishLosesRaceToConditionalUpdate(t *testing.T) {
	e := newEnv()

	video := readyVideo(t, e, e.actor, "Racing video")

	// Simulate another writer flipping the row out of ready between the
	// ownership check and the UPDATE ... WHERE status = 'ready'.
	e.videos.publishErr = repository.ErrVideoInvalidPublish

	_, err := e.videoService.Publish(context.Background(), e.actor, video.ID)
	if !errors.Is(err, ErrNotPublishable) {
		t.Fatalf("expected ErrNotPublishable, got %v", err)
	}
}

func TestNotifierFailureNeverFailsPublish(t *testing.T) {
	e := newEnv()
	e.notifier.fail(errors.New("boom"))

	video := readyVideo(t, e, e.actor, "Video with broken notifier")

	if _, err := e.videoService.Publish(
		context.Background(),
		e.actor,
		video.ID,
	); err != nil {
		t.Fatalf("publish must succeed despite notifier failure: %v", err)
	}
}

func TestUnpublishRequiresPublishedVideo(t *testing.T) {
	e := newEnv()

	video := readyVideo(t, e, e.actor, "Ready video")

	_, err := e.videoService.Unpublish(context.Background(), e.actor, video.ID)
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}

	if _, err := e.videoService.Publish(
		context.Background(),
		e.actor,
		video.ID,
	); err != nil {
		t.Fatalf("publish: %v", err)
	}

	unpublished, err := e.videoService.Unpublish(
		context.Background(),
		e.actor,
		video.ID,
	)
	if err != nil {
		t.Fatalf("unpublish: %v", err)
	}

	if unpublished.Status != model.VideoStatusReady ||
		unpublished.Visibility != model.VideoVisibilityPrivate ||
		unpublished.PublishedAt != nil {
		t.Fatalf("unexpected unpublish result: %+v", unpublished)
	}
}

func TestListScopesToViewer(t *testing.T) {
	e := newEnv()

	published := readyVideo(t, e, e.actor, "Public video")

	if _, err := e.videoService.Publish(
		context.Background(),
		e.actor,
		published.ID,
	); err != nil {
		t.Fatalf("publish: %v", err)
	}

	mustCreateVideo(t, e, e.actor, CreateVideoInput{Title: "Draft video"})

	other := e.secondCreator()
	mustCreateVideo(t, e, other, CreateVideoInput{Title: "Someone elses video"})

	anonymous, err := e.videoService.ListVideos(
		context.Background(),
		Anonymous(),
		ListVideosParams{},
	)
	if err != nil {
		t.Fatalf("anonymous list: %v", err)
	}

	if anonymous.Total != 1 {
		t.Fatalf("anonymous must see exactly the published row, got %d", anonymous.Total)
	}

	owner, err := e.videoService.ListVideos(
		context.Background(),
		e.actor,
		ListVideosParams{},
	)
	if err != nil {
		t.Fatalf("owner list: %v", err)
	}

	if owner.Total != 2 {
		t.Fatalf("owner must see own draft + published, got %d", owner.Total)
	}

	creatorFilter := e.creatorID

	filtered, err := e.videoService.ListVideos(
		context.Background(),
		Anonymous(),
		ListVideosParams{CreatorID: &creatorFilter},
	)
	if err != nil {
		t.Fatalf("filtered list: %v", err)
	}

	if filtered.Total != 1 || filtered.Items[0].ID != published.ID {
		t.Fatalf("creator filter mismatch: %+v", filtered.Items)
	}
}

func TestListSurvivesCreatorServiceOutage(t *testing.T) {
	e := newEnv()

	published := readyVideo(t, e, e.actor, "Public video")

	if _, err := e.videoService.Publish(
		context.Background(),
		e.actor,
		published.ID,
	); err != nil {
		t.Fatalf("publish: %v", err)
	}

	mustCreateVideo(t, e, e.actor, CreateVideoInput{Title: "Draft video"})

	e.resolver.fail(creatorclient.ErrUnavailable)

	page, err := e.videoService.ListVideos(
		context.Background(),
		e.actor,
		ListVideosParams{},
	)
	if err != nil {
		t.Fatalf("list must degrade to anonymous scope: %v", err)
	}

	if page.Total != 1 {
		t.Fatalf("expected only the published row, got %d", page.Total)
	}
}

func TestSetMediaStatusValidation(t *testing.T) {
	e := newEnv()

	video := mustCreateVideo(t, e, e.actor, CreateVideoInput{Title: "Draft video"})

	if _, err := e.videoService.SetMediaStatus(
		context.Background(),
		video.ID,
		"published",
		nil,
		nil,
	); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	ready, err := e.videoService.SetMediaStatus(
		context.Background(),
		video.ID,
		string(model.VideoStatusReady),
		intPtr(90),
		uuidPtr(uuid.New()),
	)
	if err != nil {
		t.Fatalf("set ready: %v", err)
	}

	if ready.Status != model.VideoStatusReady {
		t.Fatalf("expected ready, got %q", ready.Status)
	}

	if _, err := e.videoService.Publish(
		context.Background(),
		e.actor,
		video.ID,
	); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if _, err := e.videoService.SetMediaStatus(
		context.Background(),
		video.ID,
		string(model.VideoStatusProcessing),
		nil,
		nil,
	); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus on published row, got %v", err)
	}
}

func TestSetMediaStatusReadyRequiresMediaAsset(t *testing.T) {
	e := newEnv()

	video := mustCreateVideo(t, e, e.actor, CreateVideoInput{Title: "Draft video"})

	if _, err := e.videoService.SetMediaStatus(
		context.Background(),
		video.ID,
		string(model.VideoStatusReady),
		intPtr(90),
		nil,
	); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	// The failed callback must not have moved the video to ready.
	reloaded, err := e.videoService.Get(context.Background(), e.actor, video.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	if reloaded.Status != model.VideoStatusDraft {
		t.Fatalf("expected draft, got %q", reloaded.Status)
	}
}

func TestAdjustCountersValidatesAndClamps(t *testing.T) {
	e := newEnv()

	video := readyVideo(t, e, e.actor, "Counted video")

	if _, err := e.videoService.AdjustCounters(
		context.Background(),
		video.ID,
		maxCounterDelta+1,
		0,
		0,
	); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for huge delta, got %v", err)
	}

	updated, err := e.videoService.RecordView(context.Background(), video.ID)
	if err != nil {
		t.Fatalf("record view: %v", err)
	}

	if updated.ViewCount != 1 {
		t.Fatalf("expected 1 view, got %d", updated.ViewCount)
	}

	// Negative deltas clamp at zero instead of underflowing.
	clamped, err := e.videoService.AdjustCounters(
		context.Background(),
		video.ID,
		-100,
		0,
		0,
	)
	if err != nil {
		t.Fatalf("negative delta: %v", err)
	}

	if clamped.ViewCount != 0 {
		t.Fatalf("expected clamp to 0, got %d", clamped.ViewCount)
	}
}

func TestMissingVideoReadsAsNotFound(t *testing.T) {
	e := newEnv()

	if _, err := e.videoService.Get(
		context.Background(),
		Anonymous(),
		uuid.New(),
	); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func stringPtr(value string) *string {
	return &value
}

func uuidPtr(value uuid.UUID) *uuid.UUID {
	return &value
}
