package service

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/content-service/internal/creatorclient"
	"github.com/Anshul563/edvance-project/services/content-service/internal/model"
	"github.com/Anshul563/edvance-project/services/content-service/internal/notifier"
	"github.com/Anshul563/edvance-project/services/content-service/internal/repository"
)

// testLimits mirrors production defaults from config.
func testLimits() Limits {
	return Limits{
		DefaultPage:             20,
		MaxPage:                 100,
		MaxShortDurationSeconds: 180,
		MaxTitleLength:          200,
		MaxDescriptionLength:    5000,
		MaxPostContentLength:    5000,
		MaxTagsPerItem:          10,
	}
}

// fakeCreatorResolver maps users to the creators they own. fail() makes
// creator-service unreachable so tests can prove reads degrade and
// writes fail closed.
type fakeCreatorResolver struct {
	mu     sync.Mutex
	byUser map[uuid.UUID]uuid.UUID
	err    error
}

func newFakeCreatorResolver() *fakeCreatorResolver {
	return &fakeCreatorResolver{
		byUser: make(map[uuid.UUID]uuid.UUID),
	}
}

func (f *fakeCreatorResolver) set(userID, creatorID uuid.UUID) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.byUser[userID] = creatorID
}

func (f *fakeCreatorResolver) fail(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.err = err
}

func (f *fakeCreatorResolver) ResolveCreatorID(
	_ context.Context,
	userID uuid.UUID,
	_ string,
) (uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.err != nil {
		return uuid.Nil, f.err
	}

	creatorID, ok := f.byUser[userID]
	if !ok {
		return uuid.Nil, creatorclient.ErrCreatorNotFound
	}

	return creatorID, nil
}

// fakeNotifier records events so tests can assert publish fired (and
// with what) without any network.
type fakeNotifier struct {
	mu     sync.Mutex
	events []notifier.Event
	err    error
}

func (f *fakeNotifier) Notify(_ context.Context, event notifier.Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.events = append(f.events, event)

	return f.err
}

func (f *fakeNotifier) fail(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.err = err
}

func (f *fakeNotifier) types() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	types := make([]string, 0, len(f.events))

	for _, event := range f.events {
		types = append(types, event.Type)
	}

	return types
}

// normalizeTags converts raw tag input into the canonical tags a
// repository row would carry, mirroring NormalizeTag usage in SQL.
func normalizeTags(tagNames []string) []model.Tag {
	tags := make([]model.Tag, 0, len(tagNames))
	seen := make(map[string]bool, len(tagNames))

	for _, raw := range tagNames {
		name, slug, ok := model.NormalizeTag(raw)
		if !ok || seen[slug] {
			continue
		}

		seen[slug] = true

		tags = append(tags, model.Tag{
			ID:        uuid.New(),
			Name:      name,
			Slug:      slug,
			CreatedAt: time.Now(),
		})
	}

	return tags
}

func cloneVideo(video *model.Video) *model.Video {
	if video == nil {
		return nil
	}

	cp := *video

	if video.Tags != nil {
		cp.Tags = append([]model.Tag(nil), video.Tags...)
	}

	return &cp
}

func cloneShort(short *model.Short) *model.Short {
	if short == nil {
		return nil
	}

	cp := *short

	if short.Tags != nil {
		cp.Tags = append([]model.Tag(nil), short.Tags...)
	}

	return &cp
}

func clonePost(post *model.Post) *model.Post {
	if post == nil {
		return nil
	}

	cp := *post

	if post.Tags != nil {
		cp.Tags = append([]model.Tag(nil), post.Tags...)
	}

	return &cp
}

func clampCounter(value int64, delta int64) int64 {
	if value+delta < 0 {
		return 0
	}

	return value + delta
}

// fakeVideoStore is an in-memory VideoStore that reproduces the
// repository's error contract and conditional-update behaviour,
// including the publish race: Publish succeeds only while the row is
// still ready.
type fakeVideoStore struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]*model.Video
	bySlug map[string]*model.Video
	tags   map[uuid.UUID][]model.Tag

	// publishErr, when set, makes Publish fail exactly like the
	// conditional UPDATE losing a race against another writer.
	publishErr error
}

func newFakeVideoStore() *fakeVideoStore {
	return &fakeVideoStore{
		byID:   make(map[uuid.UUID]*model.Video),
		bySlug: make(map[string]*model.Video),
		tags:   make(map[uuid.UUID][]model.Tag),
	}
}

func (f *fakeVideoStore) CreateWithTags(
	_ context.Context,
	video *model.Video,
	tagNames []string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.bySlug[video.Slug]; exists {
		return repository.ErrVideoSlugTaken
	}

	now := time.Now()

	video.ID = uuid.New()
	video.CreatedAt = now
	video.UpdatedAt = now
	video.ViewCount = 0
	video.LikeCount = 0
	video.CommentCount = 0
	video.PublishedAt = nil
	video.Tags = nil

	stored := cloneVideo(video)
	f.byID[video.ID] = stored
	f.bySlug[video.Slug] = stored
	f.tags[video.ID] = normalizeTags(tagNames)

	return nil
}

func (f *fakeVideoStore) FindByID(
	_ context.Context,
	id uuid.UUID,
) (*model.Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	video, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrVideoNotFound
	}

	return cloneVideo(video), nil
}

func (f *fakeVideoStore) FindBySlug(
	_ context.Context,
	slug string,
) (*model.Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	video, ok := f.bySlug[slug]
	if !ok {
		return nil, repository.ErrVideoNotFound
	}

	return cloneVideo(video), nil
}

func (f *fakeVideoStore) UpdateWithTags(
	_ context.Context,
	video *model.Video,
	tagNames *[]string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	stored, ok := f.byID[video.ID]
	if !ok {
		return repository.ErrVideoNotFound
	}

	if other, taken := f.bySlug[video.Slug]; taken && other.ID != video.ID {
		return repository.ErrVideoSlugTaken
	}

	delete(f.bySlug, stored.Slug)

	video.UpdatedAt = time.Now()
	updated := cloneVideo(video)
	f.byID[video.ID] = updated
	f.bySlug[video.Slug] = updated

	if tagNames != nil {
		f.tags[video.ID] = normalizeTags(*tagNames)
	}

	return nil
}

func (f *fakeVideoStore) DeleteWithTags(
	_ context.Context,
	id uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	video, ok := f.byID[id]
	if !ok {
		return repository.ErrVideoNotFound
	}

	delete(f.bySlug, video.Slug)
	delete(f.byID, id)
	delete(f.tags, id)

	return nil
}

func (f *fakeVideoStore) Publish(
	_ context.Context,
	id uuid.UUID,
) (*model.Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.publishErr != nil {
		return nil, f.publishErr
	}

	video, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrVideoNotFound
	}

	if video.Status != model.VideoStatusReady {
		return nil, repository.ErrVideoInvalidPublish
	}

	now := time.Now()
	video.Status = model.VideoStatusPublished
	video.Visibility = model.VideoVisibilityPublic
	publishedAt := now
	video.PublishedAt = &publishedAt
	video.UpdatedAt = now

	return cloneVideo(video), nil
}

func (f *fakeVideoStore) Unpublish(
	_ context.Context,
	id uuid.UUID,
) (*model.Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	video, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrVideoNotFound
	}

	if video.Status != model.VideoStatusPublished {
		return nil, repository.ErrVideoInvalidPublish
	}

	now := time.Now()
	video.Status = model.VideoStatusReady
	video.Visibility = model.VideoVisibilityPrivate
	video.PublishedAt = nil
	video.UpdatedAt = now

	return cloneVideo(video), nil
}

func (f *fakeVideoStore) UpdateMediaStatus(
	_ context.Context,
	id uuid.UUID,
	status model.VideoStatus,
	durationSeconds *int,
	mediaAssetID *uuid.UUID,
) (*model.Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	video, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrVideoNotFound
	}

	if video.Status == model.VideoStatusPublished ||
		video.Status == model.VideoStatusArchived {
		return nil, repository.ErrVideoInvalidStatus
	}

	video.Status = status
	video.DurationSeconds = durationSeconds
	video.MediaAssetID = mediaAssetID
	video.UpdatedAt = time.Now()

	return cloneVideo(video), nil
}

func (f *fakeVideoStore) IncrementCounters(
	_ context.Context,
	id uuid.UUID,
	viewDelta int64,
	likeDelta int64,
	commentDelta int64,
) (*model.Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	video, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrVideoNotFound
	}

	video.ViewCount = clampCounter(video.ViewCount, viewDelta)
	video.LikeCount = clampCounter(video.LikeCount, likeDelta)
	video.CommentCount = clampCounter(video.CommentCount, commentDelta)
	video.UpdatedAt = time.Now()

	return cloneVideo(video), nil
}

func (f *fakeVideoStore) List(
	_ context.Context,
	filter repository.VideoFilter,
	limit int,
	offset int,
) ([]*model.Video, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	matched := []*model.Video{}

	for _, video := range f.byID {
		if !videoVisible(video.CreatorID, video.Status == model.VideoStatusPublished, video.Visibility == model.VideoVisibilityPublic, filter.ViewerCreatorID) {
			continue
		}

		if !uuidPointed(filter.CreatorID, video.CreatorID) {
			continue
		}

		if filter.Status != nil && video.Status != *filter.Status {
			continue
		}

		if filter.Visibility != nil && video.Visibility != *filter.Visibility {
			continue
		}

		matched = append(matched, cloneVideo(video))
	}

	sortVideosDesc(matched)

	return pageSlice(matched, limit, offset), nil
}

func (f *fakeVideoStore) Count(
	_ context.Context,
	filter repository.VideoFilter,
) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var total int64

	for _, video := range f.byID {
		if !videoVisible(video.CreatorID, video.Status == model.VideoStatusPublished, video.Visibility == model.VideoVisibilityPublic, filter.ViewerCreatorID) {
			continue
		}

		if !uuidPointed(filter.CreatorID, video.CreatorID) {
			continue
		}

		if filter.Status != nil && video.Status != *filter.Status {
			continue
		}

		if filter.Visibility != nil && video.Visibility != *filter.Visibility {
			continue
		}

		total++
	}

	return total, nil
}

func (f *fakeVideoStore) TagsFor(
	_ context.Context,
	videoIDs []uuid.UUID,
) (map[uuid.UUID][]model.Tag, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	result := make(map[uuid.UUID][]model.Tag, len(videoIDs))

	for _, id := range videoIDs {
		if tags, ok := f.tags[id]; ok {
			result[id] = append([]model.Tag(nil), tags...)
		}
	}

	return result, nil
}

// fakeShortStore mirrors fakeVideoStore for short-form content.
type fakeShortStore struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]*model.Short
	bySlug map[string]*model.Short
	tags   map[uuid.UUID][]model.Tag

	publishErr error
}

func newFakeShortStore() *fakeShortStore {
	return &fakeShortStore{
		byID:   make(map[uuid.UUID]*model.Short),
		bySlug: make(map[string]*model.Short),
		tags:   make(map[uuid.UUID][]model.Tag),
	}
}

// mutate lets a test arrange a row state the service layer would
// normally prevent (for example a duration that slipped past a CHECK
// constraint), so gate logic can still be exercised.
func (f *fakeShortStore) mutate(id uuid.UUID, fn func(*model.Short)) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if short, ok := f.byID[id]; ok {
		fn(short)
	}
}

func (f *fakeShortStore) CreateWithTags(
	_ context.Context,
	short *model.Short,
	tagNames []string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.bySlug[short.Slug]; exists {
		return repository.ErrShortSlugTaken
	}

	now := time.Now()

	short.ID = uuid.New()
	short.CreatedAt = now
	short.UpdatedAt = now
	short.ViewCount = 0
	short.LikeCount = 0
	short.CommentCount = 0
	short.PublishedAt = nil
	short.Tags = nil

	stored := cloneShort(short)
	f.byID[short.ID] = stored
	f.bySlug[short.Slug] = stored
	f.tags[short.ID] = normalizeTags(tagNames)

	return nil
}

func (f *fakeShortStore) FindByID(
	_ context.Context,
	id uuid.UUID,
) (*model.Short, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	short, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrShortNotFound
	}

	return cloneShort(short), nil
}

func (f *fakeShortStore) FindBySlug(
	_ context.Context,
	slug string,
) (*model.Short, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	short, ok := f.bySlug[slug]
	if !ok {
		return nil, repository.ErrShortNotFound
	}

	return cloneShort(short), nil
}

func (f *fakeShortStore) UpdateWithTags(
	_ context.Context,
	short *model.Short,
	tagNames *[]string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	stored, ok := f.byID[short.ID]
	if !ok {
		return repository.ErrShortNotFound
	}

	if other, taken := f.bySlug[short.Slug]; taken && other.ID != short.ID {
		return repository.ErrShortSlugTaken
	}

	delete(f.bySlug, stored.Slug)

	short.UpdatedAt = time.Now()
	updated := cloneShort(short)
	f.byID[short.ID] = updated
	f.bySlug[short.Slug] = updated

	if tagNames != nil {
		f.tags[short.ID] = normalizeTags(*tagNames)
	}

	return nil
}

func (f *fakeShortStore) DeleteWithTags(
	_ context.Context,
	id uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	short, ok := f.byID[id]
	if !ok {
		return repository.ErrShortNotFound
	}

	delete(f.bySlug, short.Slug)
	delete(f.byID, id)
	delete(f.tags, id)

	return nil
}

func (f *fakeShortStore) Publish(
	_ context.Context,
	id uuid.UUID,
) (*model.Short, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.publishErr != nil {
		return nil, f.publishErr
	}

	short, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrShortNotFound
	}

	if short.Status != model.ShortStatusReady {
		return nil, repository.ErrShortInvalidPublish
	}

	now := time.Now()
	short.Status = model.ShortStatusPublished
	short.Visibility = model.ShortVisibilityPublic
	publishedAt := now
	short.PublishedAt = &publishedAt
	short.UpdatedAt = now

	return cloneShort(short), nil
}

func (f *fakeShortStore) Unpublish(
	_ context.Context,
	id uuid.UUID,
) (*model.Short, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	short, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrShortNotFound
	}

	if short.Status != model.ShortStatusPublished {
		return nil, repository.ErrShortInvalidPublish
	}

	now := time.Now()
	short.Status = model.ShortStatusReady
	short.Visibility = model.ShortVisibilityPrivate
	short.PublishedAt = nil
	short.UpdatedAt = now

	return cloneShort(short), nil
}

func (f *fakeShortStore) UpdateMediaStatus(
	_ context.Context,
	id uuid.UUID,
	status model.ShortStatus,
	durationSeconds *int,
	mediaAssetID *uuid.UUID,
) (*model.Short, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	short, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrShortNotFound
	}

	if short.Status == model.ShortStatusPublished ||
		short.Status == model.ShortStatusArchived {
		return nil, repository.ErrShortInvalidStatus
	}

	short.Status = status
	short.DurationSeconds = durationSeconds
	short.MediaAssetID = mediaAssetID
	short.UpdatedAt = time.Now()

	return cloneShort(short), nil
}

func (f *fakeShortStore) IncrementCounters(
	_ context.Context,
	id uuid.UUID,
	viewDelta int64,
	likeDelta int64,
	commentDelta int64,
) (*model.Short, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	short, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrShortNotFound
	}

	short.ViewCount = clampCounter(short.ViewCount, viewDelta)
	short.LikeCount = clampCounter(short.LikeCount, likeDelta)
	short.CommentCount = clampCounter(short.CommentCount, commentDelta)
	short.UpdatedAt = time.Now()

	return cloneShort(short), nil
}

func (f *fakeShortStore) List(
	_ context.Context,
	filter repository.ShortFilter,
	limit int,
	offset int,
) ([]*model.Short, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	matched := []*model.Short{}

	for _, short := range f.byID {
		if videoVisible(short.CreatorID, short.Status == model.ShortStatusPublished, short.Visibility == model.ShortVisibilityPublic, filter.ViewerCreatorID) &&
			uuidPointed(filter.CreatorID, short.CreatorID) {
			matched = append(matched, cloneShort(short))
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].ID.String() > matched[j].ID.String()
		}

		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	return pageSlice(matched, limit, offset), nil
}

func (f *fakeShortStore) Count(
	_ context.Context,
	filter repository.ShortFilter,
) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var total int64

	for _, short := range f.byID {
		if !videoVisible(short.CreatorID, short.Status == model.ShortStatusPublished, short.Visibility == model.ShortVisibilityPublic, filter.ViewerCreatorID) {
			continue
		}

		if !uuidPointed(filter.CreatorID, short.CreatorID) {
			continue
		}

		total++
	}

	return total, nil
}

func (f *fakeShortStore) TagsFor(
	_ context.Context,
	shortIDs []uuid.UUID,
) (map[uuid.UUID][]model.Tag, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	result := make(map[uuid.UUID][]model.Tag, len(shortIDs))

	for _, id := range shortIDs {
		if tags, ok := f.tags[id]; ok {
			result[id] = append([]model.Tag(nil), tags...)
		}
	}

	return result, nil
}

// fakePostStore is the in-memory PostStore. Posts have no slug and no
// media; publish requires draft, unpublish requires published.
type fakePostStore struct {
	mu         sync.Mutex
	byID       map[uuid.UUID]*model.Post
	tags       map[uuid.UUID][]model.Tag
	publishErr error
}

func newFakePostStore() *fakePostStore {
	return &fakePostStore{
		byID: make(map[uuid.UUID]*model.Post),
		tags: make(map[uuid.UUID][]model.Tag),
	}
}

func (f *fakePostStore) CreateWithTags(
	_ context.Context,
	post *model.Post,
	tagNames []string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	now := time.Now()

	post.ID = uuid.New()
	post.CreatedAt = now
	post.UpdatedAt = now
	post.LikeCount = 0
	post.CommentCount = 0
	post.PublishedAt = nil
	post.Tags = nil

	f.byID[post.ID] = clonePost(post)
	f.tags[post.ID] = normalizeTags(tagNames)

	return nil
}

func (f *fakePostStore) FindByID(
	_ context.Context,
	id uuid.UUID,
) (*model.Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	post, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrPostNotFound
	}

	return clonePost(post), nil
}

func (f *fakePostStore) UpdateWithTags(
	_ context.Context,
	post *model.Post,
	tagNames *[]string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.byID[post.ID]; !ok {
		return repository.ErrPostNotFound
	}

	post.UpdatedAt = time.Now()
	f.byID[post.ID] = clonePost(post)

	if tagNames != nil {
		f.tags[post.ID] = normalizeTags(*tagNames)
	}

	return nil
}

func (f *fakePostStore) DeleteWithTags(
	_ context.Context,
	id uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.byID[id]; !ok {
		return repository.ErrPostNotFound
	}

	delete(f.byID, id)
	delete(f.tags, id)

	return nil
}

func (f *fakePostStore) Publish(
	_ context.Context,
	id uuid.UUID,
) (*model.Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.publishErr != nil {
		return nil, f.publishErr
	}

	post, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrPostNotFound
	}

	if post.Status != model.PostStatusDraft {
		return nil, repository.ErrPostInvalidPublish
	}

	now := time.Now()
	post.Status = model.PostStatusPublished
	post.Visibility = model.PostVisibilityPublic
	publishedAt := now
	post.PublishedAt = &publishedAt
	post.UpdatedAt = now

	return clonePost(post), nil
}

func (f *fakePostStore) Unpublish(
	_ context.Context,
	id uuid.UUID,
) (*model.Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	post, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrPostNotFound
	}

	if post.Status != model.PostStatusPublished {
		return nil, repository.ErrPostInvalidPublish
	}

	now := time.Now()
	post.Status = model.PostStatusDraft
	post.Visibility = model.PostVisibilityPrivate
	post.PublishedAt = nil
	post.UpdatedAt = now

	return clonePost(post), nil
}

func (f *fakePostStore) IncrementCounters(
	_ context.Context,
	id uuid.UUID,
	likeDelta int64,
	commentDelta int64,
) (*model.Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	post, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrPostNotFound
	}

	post.LikeCount = clampCounter(post.LikeCount, likeDelta)
	post.CommentCount = clampCounter(post.CommentCount, commentDelta)
	post.UpdatedAt = time.Now()

	return clonePost(post), nil
}

func (f *fakePostStore) List(
	_ context.Context,
	filter repository.PostFilter,
	limit int,
	offset int,
) ([]*model.Post, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	matched := []*model.Post{}

	for _, post := range f.byID {
		if videoVisible(post.CreatorID, post.Status == model.PostStatusPublished, post.Visibility == model.PostVisibilityPublic, filter.ViewerCreatorID) &&
			uuidPointed(filter.CreatorID, post.CreatorID) {
			matched = append(matched, clonePost(post))
		}
	}

	sort.Slice(matched, func(i, j int) bool {
		if matched[i].CreatedAt.Equal(matched[j].CreatedAt) {
			return matched[i].ID.String() > matched[j].ID.String()
		}

		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	return pageSlice(matched, limit, offset), nil
}

func (f *fakePostStore) Count(
	_ context.Context,
	filter repository.PostFilter,
) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var total int64

	for _, post := range f.byID {
		if !videoVisible(post.CreatorID, post.Status == model.PostStatusPublished, post.Visibility == model.PostVisibilityPublic, filter.ViewerCreatorID) {
			continue
		}

		if !uuidPointed(filter.CreatorID, post.CreatorID) {
			continue
		}

		total++
	}

	return total, nil
}

func (f *fakePostStore) TagsFor(
	_ context.Context,
	postIDs []uuid.UUID,
) (map[uuid.UUID][]model.Tag, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	result := make(map[uuid.UUID][]model.Tag, len(postIDs))

	for _, id := range postIDs {
		if tags, ok := f.tags[id]; ok {
			result[id] = append([]model.Tag(nil), tags...)
		}
	}

	return result, nil
}

// fakeTagStore is the in-memory TagStore. It enforces unique name and
// slug like the database does.
type fakeTagStore struct {
	mu     sync.Mutex
	bySlug map[string]*model.Tag
	byName map[string]*model.Tag
}

func newFakeTagStore() *fakeTagStore {
	return &fakeTagStore{
		bySlug: make(map[string]*model.Tag),
		byName: make(map[string]*model.Tag),
	}
}

func (f *fakeTagStore) Create(
	_ context.Context,
	name string,
	slug string,
) (*model.Tag, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.bySlug[slug]; exists {
		return nil, repository.ErrTagDuplicate
	}

	if _, exists := f.byName[name]; exists {
		return nil, repository.ErrTagDuplicate
	}

	tag := &model.Tag{
		ID:        uuid.New(),
		Name:      name,
		Slug:      slug,
		CreatedAt: time.Now(),
	}

	f.bySlug[slug] = tag
	f.byName[name] = tag

	cp := *tag

	return &cp, nil
}

func (f *fakeTagStore) FindBySlug(
	_ context.Context,
	slug string,
) (*model.Tag, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	tag, ok := f.bySlug[slug]
	if !ok {
		return nil, repository.ErrTagNotFound
	}

	cp := *tag

	return &cp, nil
}

func (f *fakeTagStore) List(
	_ context.Context,
	limit int,
	offset int,
) ([]*model.Tag, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	all := make([]*model.Tag, 0, len(f.bySlug))

	for _, tag := range f.bySlug {
		cp := *tag
		all = append(all, &cp)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Slug < all[j].Slug
	})

	return pageSlice(all, limit, offset), nil
}

func (f *fakeTagStore) Count(_ context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return int64(len(f.bySlug)), nil
}

// fakeCategoryStore is the in-memory CategoryStore.
type fakeCategoryStore struct {
	mu     sync.Mutex
	bySlug map[string]*model.Category
}

func newFakeCategoryStore() *fakeCategoryStore {
	return &fakeCategoryStore{
		bySlug: make(map[string]*model.Category),
	}
}

func (f *fakeCategoryStore) seed(categories ...*model.Category) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, category := range categories {
		f.bySlug[category.Slug] = category
	}
}

func (f *fakeCategoryStore) List(
	_ context.Context,
	limit int,
	offset int,
) ([]*model.Category, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	all := make([]*model.Category, 0, len(f.bySlug))

	for _, category := range f.bySlug {
		cp := *category
		all = append(all, &cp)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Slug < all[j].Slug
	})

	return pageSlice(all, limit, offset), nil
}

func (f *fakeCategoryStore) Count(_ context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return int64(len(f.bySlug)), nil
}

func (f *fakeCategoryStore) FindBySlug(
	_ context.Context,
	slug string,
) (*model.Category, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	category, ok := f.bySlug[slug]
	if !ok {
		return nil, repository.ErrCategoryNotFound
	}

	cp := *category

	return &cp, nil
}

// videoVisible reproduces the SQL visibility clause shared by every
// content type: published+public for everyone, plus the viewer's own
// rows when a viewer creator ID is known.
func videoVisible(
	creatorID uuid.UUID,
	published bool,
	public bool,
	viewerCreatorID *uuid.UUID,
) bool {
	if published && public {
		return true
	}

	return viewerCreatorID != nil && *viewerCreatorID == creatorID
}

func uuidPointed(filter *uuid.UUID, value uuid.UUID) bool {
	return filter == nil || *filter == value
}

func pageSlice[T any](items []T, limit int, offset int) []T {
	if offset >= len(items) {
		return []T{}
	}

	items = items[offset:]

	if limit >= 0 && limit < len(items) {
		items = items[:limit]
	}

	return items
}

func sortVideosDesc(videos []*model.Video) {
	sort.Slice(videos, func(i, j int) bool {
		if videos[i].CreatedAt.Equal(videos[j].CreatedAt) {
			return videos[i].ID.String() > videos[j].ID.String()
		}

		return videos[i].CreatedAt.After(videos[j].CreatedAt)
	})
}

// env wires every fake into real services so tests exercise the actual
// service code paths.
type env struct {
	resolver *fakeCreatorResolver
	notifier *fakeNotifier

	videos     *fakeVideoStore
	shorts     *fakeShortStore
	posts      *fakePostStore
	tags       *fakeTagStore
	categories *fakeCategoryStore

	videoService    *VideoService
	shortService    *ShortService
	postService     *PostService
	tagService      *TagService
	categoryService *CategoryService

	userID    uuid.UUID
	creatorID uuid.UUID
	actor     Actor
}

func newEnv() *env {
	limits := testLimits()

	resolver := newFakeCreatorResolver()
	events := &fakeNotifier{}

	userID := uuid.New()
	creatorID := uuid.New()
	resolver.set(userID, creatorID)

	e := &env{
		resolver:   resolver,
		notifier:   events,
		videos:     newFakeVideoStore(),
		shorts:     newFakeShortStore(),
		posts:      newFakePostStore(),
		tags:       newFakeTagStore(),
		categories: newFakeCategoryStore(),
		userID:     userID,
		creatorID:  creatorID,
		actor:      Actor{UserID: userID, AccessToken: "test-token"},
	}

	e.videoService = NewVideoService(e.videos, resolver, events, limits)
	e.shortService = NewShortService(e.shorts, resolver, events, limits)
	e.postService = NewPostService(e.posts, resolver, events, limits)
	e.tagService = NewTagService(e.tags, limits)
	e.categoryService = NewCategoryService(e.categories, limits)

	return e
}

// secondCreator registers another (user, creator) pair so ownership
// checks have a real non-owner to reject.
func (e *env) secondCreator() Actor {
	userID := uuid.New()
	creatorID := uuid.New()

	e.resolver.set(userID, creatorID)

	return Actor{UserID: userID, AccessToken: "other-token"}
}
