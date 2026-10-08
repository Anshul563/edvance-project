package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
	"github.com/Anshul563/edvance-project/services/content-service/internal/notifier"
	"github.com/Anshul563/edvance-project/services/content-service/internal/repository"
)

// PostStore is the persistence contract for posts.
// *repository.PostRepository satisfies it.
type PostStore interface {
	CreateWithTags(ctx context.Context, post *model.Post, tagNames []string) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Post, error)
	UpdateWithTags(
		ctx context.Context,
		post *model.Post,
		tagNames *[]string,
	) error
	DeleteWithTags(ctx context.Context, id uuid.UUID) error
	Publish(ctx context.Context, id uuid.UUID) (*model.Post, error)
	Unpublish(ctx context.Context, id uuid.UUID) (*model.Post, error)
	IncrementCounters(
		ctx context.Context,
		id uuid.UUID,
		likeDelta int64,
		commentDelta int64,
	) (*model.Post, error)
	List(
		ctx context.Context,
		filter repository.PostFilter,
		limit int,
		offset int,
	) ([]*model.Post, error)
	Count(ctx context.Context, filter repository.PostFilter) (int64, error)
	TagsFor(
		ctx context.Context,
		postIDs []uuid.UUID,
	) (map[uuid.UUID][]model.Tag, error)
}

// PostService owns text posts: creation, owner edits, publishing, and
// like/comment counter maintenance.
type PostService struct {
	posts    PostStore
	creators CreatorResolver
	notifier notifier.Notifier
	limits   Limits
}

func NewPostService(
	posts PostStore,
	creators CreatorResolver,
	publisher notifier.Notifier,
	limits Limits,
) *PostService {
	if publisher == nil {
		publisher = notifier.Noop()
	}

	return &PostService{
		posts:    posts,
		creators: creators,
		notifier: publisher,
		limits:   limits,
	}
}

// CreatePostInput is the client-writable subset of a post. Posts have
// no slug and no media reference.
type CreatePostInput struct {
	Content    string
	Visibility string
	Tags       []string
}

// UpdatePostInput mirrors create but every field is optional.
type UpdatePostInput struct {
	Content    *string
	Visibility *string
	Tags       *[]string
}

// ListPostsParams is the parsed query string for post listings.
type ListPostsParams struct {
	Page      int
	Limit     int
	CreatorID *uuid.UUID
}

// Create registers a post in draft state. Empty content is rejected —
// an empty post is never valid.
func (s *PostService) Create(
	ctx context.Context,
	actor Actor,
	input CreatePostInput,
) (*model.Post, error) {
	creatorID, err := resolveCreatorID(ctx, s.creators, actor)
	if err != nil {
		return nil, err
	}

	content, err := s.validateContent(input.Content)
	if err != nil {
		return nil, err
	}

	visibility, err := parsePostVisibility(input.Visibility)
	if err != nil {
		return nil, err
	}

	if err := validateTags(input.Tags, s.limits); err != nil {
		return nil, err
	}

	post := &model.Post{
		CreatorID:  creatorID,
		Content:    content,
		Visibility: visibility,
		Status:     model.PostStatusDraft,
	}

	if err := s.posts.CreateWithTags(ctx, post, input.Tags); err != nil {
		return nil, mapRepoError(err)
	}

	if err := s.attachTags(ctx, post); err != nil {
		return nil, err
	}

	return post, nil
}

// Get loads a post. Published+public posts are readable by anyone;
// every other post reads as not-found for non-owners.
func (s *PostService) Get(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
) (*model.Post, error) {
	post, err := s.posts.FindByID(ctx, id)
	if err != nil {
		return nil, mapRepoError(err)
	}

	viewer := viewerCreatorID(ctx, s.creators, actor)

	if !visibleTo(
		viewer,
		post.CreatorID,
		post.Status == model.PostStatusPublished,
		post.Visibility == model.PostVisibilityPublic,
	) {
		return nil, ErrNotFound
	}

	if err := s.attachTags(ctx, post); err != nil {
		return nil, err
	}

	return post, nil
}

// Update applies an owner edit. Status, counters, and published_at
// cannot be written here.
func (s *PostService) Update(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
	input UpdatePostInput,
) (*model.Post, error) {
	post, err := s.ownedPost(ctx, actor, id)
	if err != nil {
		return nil, err
	}

	if input.Content != nil {
		content, err := s.validateContent(*input.Content)
		if err != nil {
			return nil, err
		}

		post.Content = content
	}

	if input.Visibility != nil {
		visibility, err := parsePostVisibility(*input.Visibility)
		if err != nil {
			return nil, err
		}

		post.Visibility = visibility
	}

	var tagNames *[]string

	if input.Tags != nil {
		if err := validateTags(*input.Tags, s.limits); err != nil {
			return nil, err
		}

		tagNames = input.Tags
	}

	if err := s.posts.UpdateWithTags(ctx, post, tagNames); err != nil {
		return nil, mapRepoError(err)
	}

	if err := s.attachTags(ctx, post); err != nil {
		return nil, err
	}

	return post, nil
}

// Delete removes a post and its tag relations. Only the owning creator
// may delete.
func (s *PostService) Delete(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
) error {
	if _, err := s.ownedPost(ctx, actor, id); err != nil {
		return err
	}

	if err := s.posts.DeleteWithTags(ctx, id); err != nil {
		return mapRepoError(err)
	}

	return nil
}

// Publish moves a draft post to the public surface. Posts have no media
// pipeline, so the gate is ownership plus a valid draft state.
func (s *PostService) Publish(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
) (*model.Post, error) {
	post, err := s.ownedPost(ctx, actor, id)
	if err != nil {
		return nil, err
	}

	var reasons []string

	if post.Status != model.PostStatusDraft {
		reasons = append(
			reasons,
			fmt.Sprintf(
				"post status is %q, only %q posts can publish",
				post.Status,
				model.PostStatusDraft,
			),
		)
	}

	if _, err := s.validateContent(post.Content); err != nil {
		reasons = append(reasons, "content is invalid")
	}

	if len(reasons) > 0 {
		return nil, fmt.Errorf(
			"%w: %s",
			ErrNotPublishable,
			strings.Join(reasons, "; "),
		)
	}

	published, err := s.posts.Publish(ctx, id)
	if err != nil {
		if mapRepoError(err) == ErrNotPublishable {
			return nil, fmt.Errorf(
				"%w: post is no longer %q",
				ErrNotPublishable,
				model.PostStatusDraft,
			)
		}

		return nil, mapRepoError(err)
	}

	s.notifyPublished(ctx, actor, "post", published.ID, excerpt(published.Content))

	return published, nil
}

// Unpublish takes a post off the public surface: status returns to
// draft, visibility to private, published_at cleared.
func (s *PostService) Unpublish(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
) (*model.Post, error) {
	if _, err := s.ownedPost(ctx, actor, id); err != nil {
		return nil, err
	}

	unpublished, err := s.posts.Unpublish(ctx, id)
	if err != nil {
		if mapRepoError(err) == ErrNotPublishable {
			return nil, fmt.Errorf(
				"%w: only published posts can unpublish",
				ErrInvalidStatus,
			)
		}

		return nil, mapRepoError(err)
	}

	return unpublished, nil
}

// ListPosts returns a paginated page scoped to the viewer.
func (s *PostService) ListPosts(
	ctx context.Context,
	actor Actor,
	params ListPostsParams,
) (Page[*model.Post], error) {
	page, limit := NormalizePagination(params.Page, params.Limit, s.limits)

	filter := repository.PostFilter{
		CreatorID:       params.CreatorID,
		ViewerCreatorID: viewerOrNil(ctx, s.creators, actor),
	}

	total, err := s.posts.Count(ctx, filter)
	if err != nil {
		return Page[*model.Post]{}, mapRepoError(err)
	}

	posts, err := s.posts.List(ctx, filter, limit, offset(page, limit))
	if err != nil {
		return Page[*model.Post]{}, mapRepoError(err)
	}

	if err := s.attachTags(ctx, posts...); err != nil {
		return Page[*model.Post]{}, err
	}

	return newPage(posts, page, limit, total), nil
}

// AdjustCounters applies like/comment deltas for social consumers.
// Posts have no view counter.
func (s *PostService) AdjustCounters(
	ctx context.Context,
	id uuid.UUID,
	likeDelta int64,
	commentDelta int64,
) (*model.Post, error) {
	if err := validateDeltas(likeDelta, commentDelta); err != nil {
		return nil, err
	}

	post, err := s.posts.IncrementCounters(ctx, id, likeDelta, commentDelta)
	if err != nil {
		return nil, mapRepoError(err)
	}

	return post, nil
}

// ownedPost loads a post and verifies the caller's creator owns it.
func (s *PostService) ownedPost(
	ctx context.Context,
	actor Actor,
	id uuid.UUID,
) (*model.Post, error) {
	creatorID, err := resolveCreatorID(ctx, s.creators, actor)
	if err != nil {
		return nil, err
	}

	post, err := s.posts.FindByID(ctx, id)
	if err != nil {
		return nil, mapRepoError(err)
	}

	if post.CreatorID != creatorID {
		return nil, ErrForbidden
	}

	return post, nil
}

// excerpt derives a short display title for notification payloads from
// post content, which has no title column of its own.
func excerpt(content string) string {
	const maxRunes = 80

	runes := []rune(strings.TrimSpace(content))

	if len(runes) <= maxRunes {
		return string(runes)
	}

	return string(runes[:maxRunes]) + "..."
}

// validateContent trims a post body and rejects empty or oversized
// content.
func (s *PostService) validateContent(raw string) (string, error) {
	content := strings.TrimSpace(raw)
	length := utf8.RuneCountInString(content)

	if length == 0 {
		return "", fmt.Errorf("%w: post content cannot be empty", ErrInvalidInput)
	}

	if length > s.limits.MaxPostContentLength {
		return "", fmt.Errorf(
			"%w: post content must be at most %d characters",
			ErrInvalidInput,
			s.limits.MaxPostContentLength,
		)
	}

	return content, nil
}

// attachTags loads tags for one or many posts in a single batch query.
func (s *PostService) attachTags(
	ctx context.Context,
	posts ...*model.Post,
) error {
	ids := make([]uuid.UUID, 0, len(posts))

	for _, post := range posts {
		if post != nil {
			ids = append(ids, post.ID)
		}
	}

	if len(ids) == 0 {
		return nil
	}

	byContent, err := s.posts.TagsFor(ctx, ids)
	if err != nil {
		return fmt.Errorf("load tags: %w", mapRepoError(err))
	}

	for _, post := range posts {
		if post == nil {
			continue
		}

		if tags, ok := byContent[post.ID]; ok {
			post.Tags = tags
		}

		if post.Tags == nil {
			post.Tags = []model.Tag{}
		}
	}

	return nil
}

// notifyPublished emits the content publish event. Best-effort: a
// notification failure is logged, never surfaced to the creator.
func (s *PostService) notifyPublished(
	ctx context.Context,
	actor Actor,
	kind string,
	id uuid.UUID,
	title string,
) {
	err := s.notifier.Notify(ctx, notifier.Event{
		EventID: uuid.New().String(),
		Type:    notifier.EventContentPublished,
		UserID:  actor.UserID,
		Data: map[string]string{
			"contentType": kind,
			"contentId":   id.String(),
			"title":       title,
		},
	})
	if err != nil {
		slog.Warn(
			"content publish notification failed",
			"error", err,
			"content_type", kind,
			"content_id", id,
		)
	}
}
