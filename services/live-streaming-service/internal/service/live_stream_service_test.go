package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/live-streaming-service/internal/model"
)

func newTestService(store StreamStore, media MediaEngine) *LiveStreamService {
	return NewLiveStreamService(
		store,
		media,
		PublicAddresses{
			RTMPURL:    "rtmp://192.168.1.10:1935",
			HLSBaseURL: "http://192.168.1.10:8888",
			PathPrefix: "ls",
		},
		nil,
	)
}

func extractPathAndToken(t *testing.T, streamKey string) (pathName, token string) {
	t.Helper()

	pathName, token, found := strings.Cut(streamKey, "?token=")
	if !found {
		t.Fatalf("stream key %q must have the form path?token=secret", streamKey)
	}

	return pathName, token
}

func TestCreateSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	media := newFakeMedia()
	svc := newTestService(store, media)

	creator := uuid.New()
	created, err := svc.CreateSession(ctx, creator, CreateStreamInput{
		Title:       "Algebra live class",
		Description: "Quadratic equations",
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if created.Stream.Status != model.StatusWaiting {
		t.Fatalf("expected status waiting right after create, got %q", created.Stream.Status)
	}
	if created.StreamKey == "" {
		t.Fatal("expected a raw stream key at creation")
	}
	if !strings.HasPrefix(created.Stream.PathName, "ls_") {
		t.Fatalf("expected path name prefixed ls_, got %q", created.Stream.PathName)
	}
	if strings.Contains(created.Stream.PlaybackURL, created.StreamKey) ||
		strings.Contains(created.Stream.PlaybackURL, "?token=") {
		t.Fatalf("playback URL must not contain the stream key: %q", created.Stream.PlaybackURL)
	}
	if !strings.HasSuffix(created.Stream.PlaybackURL, created.Stream.PathName+"/index.m3u8") {
		t.Fatalf("playback URL must address the public path: %q", created.Stream.PlaybackURL)
	}
	if created.Stream.StreamKeyHash == "" || created.Stream.StreamKeyHash == extractToken(t, created.StreamKey) {
		t.Fatal("expected a hashed key, not the raw token")
	}

	pathName, token := extractPathAndToken(t, created.StreamKey)

	// Wrong token is rejected; the ended state is rejected too.
	if err := svc.AuthorizePublish(ctx, pathName, "wrong-token"); !errors.Is(err, ErrUnauthorizedPub) {
		t.Fatalf("expected unauthorized for wrong token, got %v", err)
	}
	if err := svc.AuthorizePublish(ctx, "ls_unknown", token); !errors.Is(err, ErrUnauthorizedPub) {
		t.Fatalf("expected unauthorized for unknown path, got %v", err)
	}
	if err := svc.AuthorizePublish(ctx, pathName, token); err != nil {
		t.Fatalf("expected authorize to accept valid token, got %v", err)
	}

	// Publisher connects (MediaMTX runOnOnline).
	if err := svc.MarkPublisherOnline(ctx, pathName); err != nil {
		t.Fatalf("MarkPublisherOnline: %v", err)
	}
	live, err := svc.GetOwnSession(ctx, creator, created.Stream.ID)
	if err != nil {
		t.Fatalf("GetOwnSession: %v", err)
	}
	if live.Status != model.StatusLive {
		t.Fatalf("expected live after publisher online, got %q", live.Status)
	}
	if live.StartedAt == nil {
		t.Fatal("expected started_at to be set")
	}

	active, err := svc.ListActiveSessions(ctx)
	if err != nil {
		t.Fatalf("ListActiveSessions: %v", err)
	}
	if len(active) != 1 || active[0].ID != created.Stream.ID {
		t.Fatalf("expected the session in the active list, got %+v", active)
	}

	// Publisher disconnects (runOnOffline): back to waiting, not ended.
	if err := svc.MarkPublisherOffline(ctx, pathName); err != nil {
		t.Fatalf("MarkPublisherOffline: %v", err)
	}
	afterDrop, err := svc.GetOwnSession(ctx, creator, created.Stream.ID)
	if err != nil {
		t.Fatalf("GetOwnSession: %v", err)
	}
	if afterDrop.Status != model.StatusWaiting {
		t.Fatalf("expected waiting after publisher offline, got %q", afterDrop.Status)
	}

	// Reconnect is allowed while waiting.
	if err := svc.AuthorizePublish(ctx, pathName, token); err != nil {
		t.Fatalf("expected reconnect to be authorized, got %v", err)
	}
	if err := svc.MarkPublisherOnline(ctx, pathName); err != nil {
		t.Fatalf("MarkPublisherOnline (reconnect): %v", err)
	}

	// Creator ends the session; publisher is kicked; key is revoked.
	media.setPublisher(pathName, "conn-1")
	ended, err := svc.EndSession(ctx, creator, created.Stream.ID)
	if err != nil {
		t.Fatalf("EndSession: %v", err)
	}
	if ended.Status != model.StatusEnded {
		t.Fatalf("expected ended status, got %q", ended.Status)
	}
	if media.kickCount() != 1 {
		t.Fatalf("expected publisher to be kicked once, got %d", media.kickCount())
	}
	if err := svc.AuthorizePublish(ctx, pathName, token); !errors.Is(err, ErrUnauthorizedPub) {
		t.Fatalf("expected publish to be rejected after end, got %v", err)
	}

	// Ending again is idempotent.
	if _, err := svc.EndSession(ctx, creator, created.Stream.ID); err != nil {
		t.Fatalf("EndSession (idempotent): %v", err)
	}
}

func TestHookEventsAreIdempotent(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(newMemoryStore(), newFakeMedia())

	created, err := svc.CreateSession(ctx, uuid.New(), CreateStreamInput{Title: "T"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	pathName, _ := extractPathAndToken(t, created.StreamKey)

	for i := 0; i < 3; i++ {
		if err := svc.MarkPublisherOnline(ctx, pathName); err != nil {
			t.Fatalf("MarkPublisherOnline #%d: %v", i, err)
		}
	}
	for i := 0; i < 3; i++ {
		if err := svc.MarkPublisherOffline(ctx, pathName); err != nil {
			t.Fatalf("MarkPublisherOffline #%d: %v", i, err)
		}
	}

	final, err := svc.GetOwnSession(ctx, created.Stream.CreatorID, created.Stream.ID)
	if err != nil {
		t.Fatalf("GetOwnSession: %v", err)
	}
	if final.Status != model.StatusWaiting {
		t.Fatalf("expected waiting after repeated hooks, got %q", final.Status)
	}

	// Stale events for unknown paths surface the not-found sentinel.
	if err := svc.MarkPublisherOnline(ctx, "ls_missing"); !errors.Is(err, model.ErrStreamNotFound) {
		t.Fatalf("expected not found for unknown path, got %v", err)
	}
}

func TestSessionOwnership(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(newMemoryStore(), newFakeMedia())

	alice := uuid.New()
	bob := uuid.New()

	created, err := svc.CreateSession(ctx, alice, CreateStreamInput{Title: "Alice class"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if _, err := svc.GetOwnSession(ctx, bob, created.Stream.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden for non-owner read, got %v", err)
	}
	if _, err := svc.EndSession(ctx, bob, created.Stream.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden for non-owner end, got %v", err)
	}
	if _, err := svc.RegenerateStreamKey(ctx, bob, created.Stream.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden for non-owner regenerate, got %v", err)
	}

	// Non-owner lists never see the other creator's sessions.
	bobSessions, err := svc.ListCreatorSessions(ctx, bob)
	if err != nil {
		t.Fatalf("ListCreatorSessions: %v", err)
	}
	if len(bobSessions) != 0 {
		t.Fatalf("expected no sessions for bob, got %d", len(bobSessions))
	}
}

func TestRegenerateStreamKey(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(newMemoryStore(), newFakeMedia())

	creator := uuid.New()
	created, err := svc.CreateSession(ctx, creator, CreateStreamInput{Title: "T"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	oldPath, oldToken := extractPathAndToken(t, created.StreamKey)

	regenerated, err := svc.RegenerateStreamKey(ctx, creator, created.Stream.ID)
	if err != nil {
		t.Fatalf("RegenerateStreamKey: %v", err)
	}

	newPath, newToken := extractPathAndToken(t, regenerated.StreamKey)
	if newPath != oldPath {
		t.Fatalf("path name must stay stable across regeneration: %q vs %q", oldPath, newPath)
	}
	if newToken == oldToken {
		t.Fatal("expected a fresh token after regeneration")
	}
	if regenerated.Stream.PlaybackURL != created.Stream.PlaybackURL {
		t.Fatal("playback URL must stay stable across regeneration")
	}

	// Old token is revoked, new one works.
	if err := svc.AuthorizePublish(ctx, oldPath, oldToken); !errors.Is(err, ErrUnauthorizedPub) {
		t.Fatalf("expected old token rejected, got %v", err)
	}
	if err := svc.AuthorizePublish(ctx, newPath, newToken); err != nil {
		t.Fatalf("expected new token accepted, got %v", err)
	}
}

func TestRegenerateRejectedAfterEnd(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(newMemoryStore(), newFakeMedia())

	creator := uuid.New()
	created, err := svc.CreateSession(ctx, creator, CreateStreamInput{Title: "T"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if _, err := svc.EndSession(ctx, creator, created.Stream.ID); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
	if _, err := svc.RegenerateStreamKey(ctx, creator, created.Stream.ID); !errors.Is(err, ErrAlreadyEnded) {
		t.Fatalf("expected already ended, got %v", err)
	}
}

func TestCreateSessionValidation(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(newMemoryStore(), newFakeMedia())
	creator := uuid.New()

	if _, err := svc.CreateSession(ctx, creator, CreateStreamInput{Title: "   "}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input for empty title, got %v", err)
	}

	longTitle := strings.Repeat("a", maxTitleLength+1)
	if _, err := svc.CreateSession(ctx, creator, CreateStreamInput{Title: longTitle}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input for long title, got %v", err)
	}

	longDesc := strings.Repeat("a", maxDescriptionLength+1)
	if _, err := svc.CreateSession(ctx, creator, CreateStreamInput{Title: "ok", Description: longDesc}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected invalid input for long description, got %v", err)
	}
}

func TestReconcileSyncsWithMediaServer(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()
	media := newFakeMedia()
	svc := newTestService(store, media)
	creator := uuid.New()

	created, err := svc.CreateSession(ctx, creator, CreateStreamInput{Title: "T"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	pathName, _ := extractPathAndToken(t, created.StreamKey)

	// Publisher present in the media server but no hook arrived.
	media.setPublisher(pathName, "conn-1")
	if err := svc.Reconcile(ctx); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	stream, err := svc.GetOwnSession(ctx, creator, created.Stream.ID)
	if err != nil {
		t.Fatalf("GetOwnSession: %v", err)
	}
	if stream.Status != model.StatusLive {
		t.Fatalf("expected reconcile to mark live, got %q", stream.Status)
	}

	// Publisher disappears without a hook.
	media.removePublisher(pathName)
	if err := svc.Reconcile(ctx); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	stream, err = svc.GetOwnSession(ctx, creator, created.Stream.ID)
	if err != nil {
		t.Fatalf("GetOwnSession: %v", err)
	}
	if stream.Status != model.StatusWaiting {
		t.Fatalf("expected reconcile to mark waiting, got %q", stream.Status)
	}
}

func TestPlaybackInfoHasNoSecrets(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(newMemoryStore(), newFakeMedia())

	created, err := svc.CreateSession(ctx, uuid.New(), CreateStreamInput{Title: "T"})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	info, err := svc.GetPlayback(ctx, created.Stream.ID)
	if err != nil {
		t.Fatalf("GetPlayback: %v", err)
	}
	if info.IsLive {
		t.Fatal("stream must not report live before a publisher connects")
	}
	if strings.Contains(info.PlaybackURL, "?token=") {
		t.Fatal("playback info must not contain a token")
	}

	if _, err := svc.GetPlayback(ctx, uuid.New()); !errors.Is(err, ErrStreamNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
}

func extractToken(t *testing.T, streamKey string) string {
	t.Helper()
	_, token := extractPathAndToken(t, streamKey)
	return token
}

func TestStreamKeyIsHighEntropy(t *testing.T) {
	ctx := context.Background()
	svc := newTestService(newMemoryStore(), newFakeMedia())

	seen := make(map[string]bool)
	for i := 0; i < 20; i++ {
		created, err := svc.CreateSession(ctx, uuid.New(), CreateStreamInput{Title: "T"})
		if err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
		_, token := extractPathAndToken(t, created.StreamKey)
		if len(token) < 32 {
			t.Fatalf("expected token of at least 32 hex chars, got %d", len(token))
		}
		if seen[token] {
			t.Fatal("stream keys must be unique")
		}
		seen[token] = true
	}
}
