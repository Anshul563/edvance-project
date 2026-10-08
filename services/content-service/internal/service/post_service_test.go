package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
	"github.com/Anshul563/edvance-project/services/content-service/internal/notifier"
)

func mustCreatePost(
	t *testing.T,
	e *env,
	actor Actor,
	input CreatePostInput,
) *model.Post {
	t.Helper()

	post, err := e.postService.Create(context.Background(), actor, input)
	if err != nil {
		t.Fatalf("create post: %v", err)
	}

	return post
}

func TestPostCreateRejectsEmptyContent(t *testing.T) {
	e := newEnv()

	if _, err := e.postService.Create(
		context.Background(),
		e.actor,
		CreatePostInput{Content: "   ", Visibility: "public"},
	); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestPostCreateRejectsOversizedContent(t *testing.T) {
	e := newEnv()

	huge := strings.Repeat("a", 5001)

	if _, err := e.postService.Create(
		context.Background(),
		e.actor,
		CreatePostInput{Content: huge},
	); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestPostCreateRequiresCreatorProfile(t *testing.T) {
	e := newEnv()

	_, err := e.postService.Create(
		context.Background(),
		Actor{UserID: uuid.New(), AccessToken: "token"},
		CreatePostInput{Content: "Hello"},
	)

	if !errors.Is(err, ErrNoCreatorProfile) {
		t.Fatalf("expected ErrNoCreatorProfile, got %v", err)
	}
}

func TestPostCreateIsDraftAndPrivate(t *testing.T) {
	e := newEnv()

	post := mustCreatePost(t, e, e.actor, CreatePostInput{
		Content: "  Hello world  ",
	})

	if post.Status != model.PostStatusDraft ||
		post.Visibility != model.PostVisibilityPrivate {
		t.Fatalf("unexpected draft state: %+v", post)
	}

	if post.Content != "Hello world" {
		t.Fatalf("content must be trimmed, got %q", post.Content)
	}

	if post.PublishedAt != nil {
		t.Fatal("draft must not carry published_at")
	}
}

func TestPostGetScoping(t *testing.T) {
	e := newEnv()

	post := mustCreatePost(t, e, e.actor, CreatePostInput{Content: "Draft body"})
	other := e.secondCreator()

	if _, err := e.postService.Get(context.Background(), Anonymous(), post.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("anonymous read must 404, got %v", err)
	}

	if _, err := e.postService.Get(context.Background(), other, post.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-owner read must 404, got %v", err)
	}

	if _, err := e.postService.Get(context.Background(), e.actor, post.ID); err != nil {
		t.Fatalf("owner read must succeed, got %v", err)
	}
}

func TestPostPublishLifecycle(t *testing.T) {
	e := newEnv()

	post := mustCreatePost(t, e, e.actor, CreatePostInput{
		Content:    "Publish me",
		Visibility: "private",
	})

	published, err := e.postService.Publish(context.Background(), e.actor, post.ID)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	if published.Status != model.PostStatusPublished ||
		published.Visibility != model.PostVisibilityPublic {
		t.Fatalf("unexpected published state: %+v", published)
	}

	if published.PublishedAt == nil {
		t.Fatal("published_at must be set")
	}

	types := e.notifier.types()
	if len(types) != 1 || types[0] != notifier.EventContentPublished {
		t.Fatalf("expected one publish event, got %v", types)
	}

	// Published posts read for everyone.
	if _, err := e.postService.Get(context.Background(), Anonymous(), post.ID); err != nil {
		t.Fatalf("anonymous read of published post: %v", err)
	}

	// A second publish is a state conflict, not a silent no-op.
	if _, err := e.postService.Publish(context.Background(), e.actor, post.ID); !errors.Is(err, ErrNotPublishable) {
		t.Fatalf("expected ErrNotPublishable on re-publish, got %v", err)
	}

	unpublished, err := e.postService.Unpublish(context.Background(), e.actor, post.ID)
	if err != nil {
		t.Fatalf("unpublish: %v", err)
	}

	if unpublished.Status != model.PostStatusDraft ||
		unpublished.Visibility != model.PostVisibilityPrivate ||
		unpublished.PublishedAt != nil {
		t.Fatalf("unexpected unpublish state: %+v", unpublished)
	}
}

func TestPostUnpublishRequiresPublished(t *testing.T) {
	e := newEnv()

	post := mustCreatePost(t, e, e.actor, CreatePostInput{Content: "Draft body"})

	if _, err := e.postService.Unpublish(context.Background(), e.actor, post.ID); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestPostOwnershipEnforced(t *testing.T) {
	e := newEnv()

	post := mustCreatePost(t, e, e.actor, CreatePostInput{Content: "Owner post"})
	other := e.secondCreator()

	if err := e.postService.Delete(context.Background(), other, post.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	if _, err := e.postService.Publish(context.Background(), other, post.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestPostDeleteRemovesRow(t *testing.T) {
	e := newEnv()

	post := mustCreatePost(t, e, e.actor, CreatePostInput{Content: "Bye"})

	if err := e.postService.Delete(context.Background(), e.actor, post.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := e.posts.FindByID(context.Background(), post.ID); err == nil {
		t.Fatal("post should be gone")
	}
}

func TestPostCountersValidateAndClamp(t *testing.T) {
	e := newEnv()

	post := mustCreatePost(t, e, e.actor, CreatePostInput{Content: "Counted post"})

	if _, err := e.postService.AdjustCounters(
		context.Background(),
		post.ID,
		maxCounterDelta+1,
		0,
	); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	updated, err := e.postService.AdjustCounters(
		context.Background(),
		post.ID,
		3,
		1,
	)
	if err != nil {
		t.Fatalf("adjust counters: %v", err)
	}

	if updated.LikeCount != 3 || updated.CommentCount != 1 {
		t.Fatalf("unexpected counters: %+v", updated)
	}

	clamped, err := e.postService.AdjustCounters(
		context.Background(),
		post.ID,
		-10,
		-10,
	)
	if err != nil {
		t.Fatalf("clamp: %v", err)
	}

	if clamped.LikeCount != 0 || clamped.CommentCount != 0 {
		t.Fatalf("expected clamp to zero, got %+v", clamped)
	}
}

func TestPostListScopesToViewer(t *testing.T) {
	e := newEnv()

	published := mustCreatePost(t, e, e.actor, CreatePostInput{Content: "Public post"})

	if _, err := e.postService.Publish(
		context.Background(),
		e.actor,
		published.ID,
	); err != nil {
		t.Fatalf("publish: %v", err)
	}

	mustCreatePost(t, e, e.actor, CreatePostInput{Content: "Draft post"})

	other := e.secondCreator()
	mustCreatePost(t, e, other, CreatePostInput{Content: "Someone elses post"})

	anonymous, err := e.postService.ListPosts(
		context.Background(),
		Anonymous(),
		ListPostsParams{},
	)
	if err != nil {
		t.Fatalf("anonymous list: %v", err)
	}

	if anonymous.Total != 1 {
		t.Fatalf("anonymous must see 1 post, got %d", anonymous.Total)
	}

	owner, err := e.postService.ListPosts(
		context.Background(),
		e.actor,
		ListPostsParams{},
	)
	if err != nil {
		t.Fatalf("owner list: %v", err)
	}

	if owner.Total != 2 {
		t.Fatalf("owner must see 2 posts, got %d", owner.Total)
	}
}
