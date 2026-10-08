//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
)

// Needs PostgreSQL with migrations applied to edvance_content:
//
//	DATABASE_URL=postgres://... go test -tags integration ./internal/repository/
func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}

	t.Cleanup(pool.Close)

	return pool
}

// cleanupCreator removes every row the test created for one creator
// ID. Creator IDs are random UUIDs with no foreign keys, so this is a
// plain delete and also leaves the table clean for other runs.
func cleanupCreator(t *testing.T, pool *pgxpool.Pool, creatorID uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		ctx := context.Background()

		_, _ = pool.Exec(ctx, `DELETE FROM video_tags WHERE video_id IN (
			SELECT id FROM videos WHERE creator_id = $1
		)`, creatorID)
		_, _ = pool.Exec(ctx, `DELETE FROM short_tags WHERE short_id IN (
			SELECT id FROM shorts WHERE creator_id = $1
		)`, creatorID)
		_, _ = pool.Exec(ctx, `DELETE FROM post_tags WHERE post_id IN (
			SELECT id FROM posts WHERE creator_id = $1
		)`, creatorID)
		_, _ = pool.Exec(ctx, `DELETE FROM videos WHERE creator_id = $1`, creatorID)
		_, _ = pool.Exec(ctx, `DELETE FROM shorts WHERE creator_id = $1`, creatorID)
		_, _ = pool.Exec(ctx, `DELETE FROM posts WHERE creator_id = $1`, creatorID)
	})
}

func testVideo(creatorID uuid.UUID, slug string) *model.Video {
	return &model.Video{
		CreatorID:  creatorID,
		Title:      "Integration Video",
		Slug:       slug,
		Visibility: model.VideoVisibilityPrivate,
		Status:     model.VideoStatusDraft,
	}
}

func TestVideoLifecycleIntegration(t *testing.T) {
	pool := newTestPool(t)

	ctx := context.Background()
	creatorID := uuid.New()
	cleanupCreator(t, pool, creatorID)

	repo := NewVideoRepository(pool)
	video := testVideo(creatorID, "integration-video-"+creatorID.String()[:8])

	// Tags are created and assigned inside the same transaction.
	if err := repo.CreateWithTags(ctx, video, []string{"Go", "go", " TESTING "}); err != nil {
		t.Fatalf("create: %v", err)
	}

	if video.ID == uuid.Nil || video.CreatedAt.IsZero() {
		t.Fatalf("insert must return id and timestamps: %+v", video)
	}

	// Duplicate tag spellings collapse into one row.
	tags, err := repo.TagsFor(ctx, []uuid.UUID{video.ID})
	if err != nil {
		t.Fatalf("tags for: %v", err)
	}

	if len(tags[video.ID]) != 2 {
		t.Fatalf("expected 2 normalized tags, got %+v", tags[video.ID])
	}

	// Slug uniqueness is enforced by the database.
	duplicate := testVideo(creatorID, video.Slug)

	if err := repo.CreateWithTags(ctx, duplicate, nil); !errors.Is(err, ErrVideoSlugTaken) {
		t.Fatalf("expected ErrVideoSlugTaken, got %v", err)
	}

	loaded, err := repo.FindBySlug(ctx, video.Slug)
	if err != nil {
		t.Fatalf("find by slug: %v", err)
	}

	loaded.Title = "Renamed Video"

	if err := repo.UpdateWithTags(ctx, loaded, nil); err != nil {
		t.Fatalf("update: %v", err)
	}

	renamed, err := repo.FindByID(ctx, video.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}

	if renamed.Title != "Renamed Video" || renamed.Slug != video.Slug {
		t.Fatalf("unexpected update result: %+v", renamed)
	}

	// Publishing is conditional on status = ready.
	if _, err := repo.Publish(ctx, video.ID); !errors.Is(err, ErrVideoInvalidPublish) {
		t.Fatalf("draft publish must be rejected, got %v", err)
	}

	ready, err := repo.UpdateMediaStatus(
		ctx,
		video.ID,
		model.VideoStatusReady,
		intPtr(60),
		uuidPtr(uuid.New()),
	)
	if err != nil {
		t.Fatalf("set media status: %v", err)
	}

	if ready.Status != model.VideoStatusReady {
		t.Fatalf("expected ready, got %q", ready.Status)
	}

	published, err := repo.Publish(ctx, video.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	if published.Status != model.VideoStatusPublished ||
		published.PublishedAt == nil {
		t.Fatalf("unexpected publish result: %+v", published)
	}

	// A second publish loses the conditional UPDATE.
	if _, err := repo.Publish(ctx, video.ID); !errors.Is(err, ErrVideoInvalidPublish) {
		t.Fatalf("double publish must be rejected, got %v", err)
	}

	// Published rows cannot change media status.
	if _, err := repo.UpdateMediaStatus(
		ctx,
		video.ID,
		model.VideoStatusProcessing,
		nil,
		nil,
	); !errors.Is(err, ErrVideoInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}

	counted, err := repo.IncrementCounters(ctx, video.ID, 2, 3, 4)
	if err != nil {
		t.Fatalf("counters: %v", err)
	}

	if counted.ViewCount != 2 || counted.LikeCount != 3 || counted.CommentCount != 4 {
		t.Fatalf("unexpected counters: %+v", counted)
	}

	// Counters clamp at zero instead of underflowing.
	clamped, err := repo.IncrementCounters(ctx, video.ID, -5, -5, -5)
	if err != nil {
		t.Fatalf("clamp: %v", err)
	}

	if clamped.ViewCount != 0 || clamped.LikeCount != 0 || clamped.CommentCount != 0 {
		t.Fatalf("expected zero clamp: %+v", clamped)
	}

	unpublished, err := repo.Unpublish(ctx, video.ID)
	if err != nil {
		t.Fatalf("unpublish: %v", err)
	}

	if unpublished.Status != model.VideoStatusReady ||
		unpublished.Visibility != model.VideoVisibilityPrivate ||
		unpublished.PublishedAt != nil {
		t.Fatalf("unexpected unpublish result: %+v", unpublished)
	}

	if err := repo.DeleteWithTags(ctx, video.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := repo.FindByID(ctx, video.ID); !errors.Is(err, ErrVideoNotFound) {
		t.Fatalf("expected ErrVideoNotFound, got %v", err)
	}
}

func TestVideoListScopingIntegration(t *testing.T) {
	pool := newTestPool(t)

	ctx := context.Background()
	ownerID := uuid.New()
	otherID := uuid.New()

	cleanupCreator(t, pool, ownerID)
	cleanupCreator(t, pool, otherID)

	repo := NewVideoRepository(pool)

	// Owner's draft.
	draft := testVideo(ownerID, "scoping-draft-"+ownerID.String()[:8])

	if err := repo.CreateWithTags(ctx, draft, nil); err != nil {
		t.Fatalf("create draft: %v", err)
	}

	// Owner's published+public video.
	publicVideo := testVideo(ownerID, "scoping-public-"+ownerID.String()[:8])
	publicVideo.Visibility = model.VideoVisibilityPublic

	if err := repo.CreateWithTags(ctx, publicVideo, nil); err != nil {
		t.Fatalf("create public: %v", err)
	}

	ready, err := repo.UpdateMediaStatus(
		ctx,
		publicVideo.ID,
		model.VideoStatusReady,
		nil,
		uuidPtr(uuid.New()),
	)
	if err != nil {
		t.Fatalf("set ready: %v", err)
	}

	if _, err := repo.Publish(ctx, ready.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// Someone else's video.
	otherVideo := testVideo(otherID, "scoping-other-"+otherID.String()[:8])
	otherVideo.Visibility = model.VideoVisibilityPublic

	if err := repo.CreateWithTags(ctx, otherVideo, nil); err != nil {
		t.Fatalf("create other: %v", err)
	}

	otherReady, err := repo.UpdateMediaStatus(
		ctx,
		otherVideo.ID,
		model.VideoStatusReady,
		nil,
		uuidPtr(uuid.New()),
	)
	if err != nil {
		t.Fatalf("set other ready: %v", err)
	}

	if _, err := repo.Publish(ctx, otherReady.ID); err != nil {
		t.Fatalf("publish other: %v", err)
	}

	// Anonymous listing must contain both published rows and must never
	// contain the owner's draft. The database is shared with whatever
	// else runs alongside this suite, so membership — not table-wide
	// counts — is what can be asserted exactly. Newest rows first, so
	// this test's rows always fall inside the requested window.
	anonymousListed, err := repo.List(ctx, VideoFilter{}, 50, 0)
	if err != nil {
		t.Fatalf("list anonymous: %v", err)
	}

	anonymousByID := make(map[uuid.UUID]*model.Video, len(anonymousListed))

	for _, video := range anonymousListed {
		anonymousByID[video.ID] = video
	}

	if _, ok := anonymousByID[publicVideo.ID]; !ok {
		t.Fatal("anonymous listing must include the published public row")
	}

	if _, ok := anonymousByID[otherVideo.ID]; !ok {
		t.Fatal("anonymous listing must include the other creator's published row")
	}

	if _, ok := anonymousByID[draft.ID]; ok {
		t.Fatal("anonymous listing must not include a private draft")
	}

	// Owner listing includes their draft as well.
	ownerListed, err := repo.List(ctx, VideoFilter{ViewerCreatorID: &ownerID}, 50, 0)
	if err != nil {
		t.Fatalf("list owner: %v", err)
	}

	ownerByID := make(map[uuid.UUID]*model.Video, len(ownerListed))

	for _, video := range ownerListed {
		ownerByID[video.ID] = video
	}

	for _, want := range []*model.Video{draft, publicVideo, otherVideo} {
		if _, ok := ownerByID[want.ID]; !ok {
			t.Fatalf("owner listing must include %q", want.Title)
		}
	}

	// Creator filter narrows to one creator, but an anonymous caller
	// still only sees that creator's published+public rows.
	total, err := repo.Count(ctx, VideoFilter{CreatorID: &ownerID})
	if err != nil {
		t.Fatalf("count filtered: %v", err)
	}

	if total != 1 {
		t.Fatalf("anonymous creator filter must see 1 row, got %d", total)
	}

	total, err = repo.Count(ctx, VideoFilter{CreatorID: &otherID})
	if err != nil {
		t.Fatalf("count other filtered: %v", err)
	}

	if total != 1 {
		t.Fatalf("anonymous other-creator filter must see 1 row, got %d", total)
	}

	// The owner filtering by themselves sees their drafts too.
	total, err = repo.Count(ctx, VideoFilter{
		CreatorID:       &ownerID,
		ViewerCreatorID: &ownerID,
	})
	if err != nil {
		t.Fatalf("count owner filtered: %v", err)
	}

	if total != 2 {
		t.Fatalf("owner creator filter must see 2 rows, got %d", total)
	}

	// Newest first, scoped to this creator so foreign rows cannot
	// change what the ordering check observes.
	listed, err := repo.List(ctx, VideoFilter{
		CreatorID:       &ownerID,
		ViewerCreatorID: &ownerID,
	}, 10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	if len(listed) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(listed))
	}

	for i := 1; i < len(listed); i++ {
		if listed[i].CreatedAt.After(listed[i-1].CreatedAt) {
			t.Fatal("list must be ordered created_at DESC")
		}
	}

	// Pagination never returns past the end.
	page, err := repo.List(ctx, VideoFilter{}, 10, 100_000)
	if err != nil {
		t.Fatalf("list page: %v", err)
	}

	if len(page) != 0 {
		t.Fatalf("expected empty page, got %d rows", len(page))
	}
}

func TestTagVocabularyIntegration(t *testing.T) {
	pool := newTestPool(t)

	ctx := context.Background()
	tags := NewTagRepository(pool)

	cleanup := func(slug string) {
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM tags WHERE slug = $1`, slug)
		})
	}

	cleanup("react-js")

	tag, err := tags.Create(ctx, "react js", "react-js")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if tag.ID == uuid.Nil || tag.Slug != "react-js" {
		t.Fatalf("unexpected tag: %+v", tag)
	}

	if _, err := tags.Create(ctx, "react js", "react-js"); !errors.Is(err, ErrTagDuplicate) {
		t.Fatalf("expected ErrTagDuplicate, got %v", err)
	}

	found, err := tags.FindBySlug(ctx, "react-js")
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if found.Name != "react js" {
		t.Fatalf("unexpected tag: %+v", found)
	}

	if _, err := tags.FindBySlug(ctx, "missing"); !errors.Is(err, ErrTagNotFound) {
		t.Fatalf("expected ErrTagNotFound, got %v", err)
	}
}

func TestPostPublishConditionalIntegration(t *testing.T) {
	pool := newTestPool(t)

	ctx := context.Background()
	creatorID := uuid.New()
	cleanupCreator(t, pool, creatorID)

	repo := NewPostRepository(pool)

	post := &model.Post{
		CreatorID:  creatorID,
		Content:    "Hello from the integration suite.",
		Visibility: model.PostVisibilityPrivate,
		Status:     model.PostStatusDraft,
	}

	if err := repo.CreateWithTags(ctx, post, []string{"news"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	published, err := repo.Publish(ctx, post.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	if published.Status != model.PostStatusPublished ||
		published.Visibility != model.PostVisibilityPublic ||
		published.PublishedAt == nil {
		t.Fatalf("unexpected publish result: %+v", published)
	}

	if _, err := repo.Publish(ctx, post.ID); !errors.Is(err, ErrPostInvalidPublish) {
		t.Fatalf("double publish must be rejected, got %v", err)
	}

	unpublished, err := repo.Unpublish(ctx, post.ID)
	if err != nil {
		t.Fatalf("unpublish: %v", err)
	}

	if unpublished.Status != model.PostStatusDraft ||
		unpublished.PublishedAt != nil {
		t.Fatalf("unexpected unpublish result: %+v", unpublished)
	}

	if _, err := repo.Unpublish(ctx, post.ID); !errors.Is(err, ErrPostInvalidPublish) {
		t.Fatalf("double unpublish must be rejected, got %v", err)
	}
}

func TestShortDurationCheckIntegration(t *testing.T) {
	pool := newTestPool(t)

	ctx := context.Background()
	creatorID := uuid.New()
	cleanupCreator(t, pool, creatorID)

	repo := NewShortRepository(pool)

	short := &model.Short{
		CreatorID:  creatorID,
		Title:      "Duration Short",
		Slug:       "duration-short-" + creatorID.String()[:8],
		Visibility: model.ShortVisibilityPrivate,
		Status:     model.ShortStatusDraft,
	}

	if err := repo.CreateWithTags(ctx, short, nil); err != nil {
		t.Fatalf("create: %v", err)
	}

	tooLong := 500

	// The database CHECK is the backstop behind the service limit.
	if _, err := repo.UpdateMediaStatus(
		ctx,
		short.ID,
		model.ShortStatusReady,
		&tooLong,
		nil,
	); err == nil {
		t.Fatal("expected the 180s CHECK to reject a 500s short")
	}
}

func intPtr(value int) *int {
	return &value
}

func uuidPtr(value uuid.UUID) *uuid.UUID {
	return &value
}
