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

func mustCreateShort(
	t *testing.T,
	e *env,
	actor Actor,
	input CreateShortInput,
) *model.Short {
	t.Helper()

	short, err := e.shortService.Create(context.Background(), actor, input)
	if err != nil {
		t.Fatalf("create short: %v", err)
	}

	return short
}

// readyShort drives a short through the pipeline: status ready, media
// asset set, duration inside the configured limit.
func readyShort(t *testing.T, e *env, actor Actor, title string) *model.Short {
	t.Helper()

	short := mustCreateShort(t, e, actor, CreateShortInput{
		Title:      title,
		Visibility: "private",
	})

	if _, err := e.shortService.SetMediaStatus(
		context.Background(),
		short.ID,
		string(model.ShortStatusReady),
		intPtr(45),
		uuidPtr(uuid.New()),
	); err != nil {
		t.Fatalf("set media status: %v", err)
	}

	updated, err := e.shortService.Get(context.Background(), actor, short.ID)
	if err != nil {
		t.Fatalf("reload short: %v", err)
	}

	return updated
}

func TestShortCreateRequiresCreatorProfile(t *testing.T) {
	e := newEnv()

	_, err := e.shortService.Create(
		context.Background(),
		Actor{UserID: uuid.New(), AccessToken: "token"},
		CreateShortInput{Title: "Some title"},
	)

	if !errors.Is(err, ErrNoCreatorProfile) {
		t.Fatalf("expected ErrNoCreatorProfile, got %v", err)
	}
}

func TestShortCreateIsDraft(t *testing.T) {
	e := newEnv()

	short := mustCreateShort(t, e, e.actor, CreateShortInput{
		Title:      "My Short",
		Visibility: "public",
	})

	if short.Status != model.ShortStatusDraft {
		t.Fatalf("expected draft, got %q", short.Status)
	}

	if short.Slug != "my-short" {
		t.Fatalf("expected slug my-short, got %q", short.Slug)
	}

	if short.DurationSeconds != nil {
		t.Fatal("duration must never come from client input")
	}
}

func TestShortPublishRejectsDraftWithoutMedia(t *testing.T) {
	e := newEnv()

	short := mustCreateShort(t, e, e.actor, CreateShortInput{Title: "No media"})

	_, err := e.shortService.Publish(context.Background(), e.actor, short.ID)
	if !errors.Is(err, ErrNotPublishable) {
		t.Fatalf("expected ErrNotPublishable, got %v", err)
	}
}

func TestShortPublishRejectsOverlongDuration(t *testing.T) {
	e := newEnv()

	short := readyShort(t, e, e.actor, "Too long")

	// Arrange a duration beyond the limit, as a bad upstream report
	// would leave behind.
	e.shorts.mutate(short.ID, func(row *model.Short) {
		row.DurationSeconds = intPtr(500)
	})

	_, err := e.shortService.Publish(context.Background(), e.actor, short.ID)
	if !errors.Is(err, ErrNotPublishable) {
		t.Fatalf("expected ErrNotPublishable, got %v", err)
	}

	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected duration reason in %q", err.Error())
	}
}

func TestShortSetMediaStatusRejectsOverlongDuration(t *testing.T) {
	e := newEnv()

	short := mustCreateShort(t, e, e.actor, CreateShortInput{Title: "Draft short"})

	_, err := e.shortService.SetMediaStatus(
		context.Background(),
		short.ID,
		string(model.ShortStatusReady),
		intPtr(500),
		nil,
	)

	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestShortSetMediaStatusReadyRequiresMediaAsset(t *testing.T) {
	e := newEnv()

	short := mustCreateShort(t, e, e.actor, CreateShortInput{Title: "Draft short"})

	if _, err := e.shortService.SetMediaStatus(
		context.Background(),
		short.ID,
		string(model.ShortStatusReady),
		intPtr(45),
		nil,
	); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}

	reloaded, err := e.shortService.Get(context.Background(), e.actor, short.ID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	if reloaded.Status != model.ShortStatusDraft {
		t.Fatalf("expected draft, got %q", reloaded.Status)
	}
}

func TestShortPublishAndNotify(t *testing.T) {
	e := newEnv()

	short := readyShort(t, e, e.actor, "Ready short")

	published, err := e.shortService.Publish(
		context.Background(),
		e.actor,
		short.ID,
	)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}

	if published.Status != model.ShortStatusPublished ||
		published.Visibility != model.ShortVisibilityPublic {
		t.Fatalf("unexpected publish state: %+v", published)
	}

	types := e.notifier.types()
	if len(types) != 1 || types[0] != notifier.EventContentPublished {
		t.Fatalf("expected one publish event, got %v", types)
	}
}

func TestShortUnpublishRestoresReadyPrivate(t *testing.T) {
	e := newEnv()

	short := readyShort(t, e, e.actor, "Ready short")

	if _, err := e.shortService.Publish(
		context.Background(),
		e.actor,
		short.ID,
	); err != nil {
		t.Fatalf("publish: %v", err)
	}

	unpublished, err := e.shortService.Unpublish(
		context.Background(),
		e.actor,
		short.ID,
	)
	if err != nil {
		t.Fatalf("unpublish: %v", err)
	}

	if unpublished.Status != model.ShortStatusReady ||
		unpublished.Visibility != model.ShortVisibilityPrivate ||
		unpublished.PublishedAt != nil {
		t.Fatalf("unexpected unpublish state: %+v", unpublished)
	}
}

func TestShortOwnershipEnforced(t *testing.T) {
	e := newEnv()

	short := mustCreateShort(t, e, e.actor, CreateShortInput{Title: "Owner short"})
	other := e.secondCreator()

	if _, err := e.shortService.Update(
		context.Background(),
		other,
		short.ID,
		UpdateShortInput{Title: stringPtr("Hijacked")},
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	if _, err := e.shortService.Get(context.Background(), other, short.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("draft must 404 for non-owner, got %v", err)
	}
}

func TestShortListScopesToViewer(t *testing.T) {
	e := newEnv()

	published := readyShort(t, e, e.actor, "Public short")

	if _, err := e.shortService.Publish(
		context.Background(),
		e.actor,
		published.ID,
	); err != nil {
		t.Fatalf("publish: %v", err)
	}

	mustCreateShort(t, e, e.actor, CreateShortInput{Title: "Draft short"})

	anonymous, err := e.shortService.ListShorts(
		context.Background(),
		Anonymous(),
		ListShortsParams{},
	)
	if err != nil {
		t.Fatalf("anonymous list: %v", err)
	}

	if anonymous.Total != 1 {
		t.Fatalf("anonymous must see 1 short, got %d", anonymous.Total)
	}

	owner, err := e.shortService.ListShorts(
		context.Background(),
		e.actor,
		ListShortsParams{},
	)
	if err != nil {
		t.Fatalf("owner list: %v", err)
	}

	if owner.Total != 2 {
		t.Fatalf("owner must see 2 shorts, got %d", owner.Total)
	}
}

func TestShortRecordView(t *testing.T) {
	e := newEnv()

	short := readyShort(t, e, e.actor, "Counted short")

	updated, err := e.shortService.RecordView(context.Background(), short.ID)
	if err != nil {
		t.Fatalf("record view: %v", err)
	}

	if updated.ViewCount != 1 {
		t.Fatalf("expected 1 view, got %d", updated.ViewCount)
	}
}
