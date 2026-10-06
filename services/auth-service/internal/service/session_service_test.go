package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/model"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/token"
)

// fakeSessionStore is an in-memory SessionStore for service unit tests.
type fakeSessionStore struct {
	mu       sync.Mutex
	sessions map[uuid.UUID]*model.AuthSession
	byHash   map[string]*model.AuthSession
}

func newFakeSessionStore() *fakeSessionStore {
	return &fakeSessionStore{
		sessions: make(map[uuid.UUID]*model.AuthSession),
		byHash:   make(map[string]*model.AuthSession),
	}
}

func (f *fakeSessionStore) Create(
	_ context.Context,
	session *model.AuthSession,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.byHash[session.RefreshTokenHash]; exists {
		return repository.ErrRefreshTokenExists
	}

	now := time.Now()
	session.ID = uuid.New()
	session.CreatedAt = now
	session.LastUsedAt = now

	stored := *session
	f.sessions[session.ID] = &stored
	f.byHash[session.RefreshTokenHash] = &stored

	return nil
}

func (f *fakeSessionStore) FindByRefreshTokenHash(
	_ context.Context,
	hash string,
) (*model.AuthSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	session, ok := f.byHash[hash]
	if !ok {
		return nil, repository.ErrSessionNotFound
	}

	cp := *session

	return &cp, nil
}

func (f *fakeSessionStore) FindByID(
	_ context.Context,
	id uuid.UUID,
) (*model.AuthSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	session, ok := f.sessions[id]
	if !ok {
		return nil, repository.ErrSessionNotFound
	}

	cp := *session

	return &cp, nil
}

func (f *fakeSessionStore) ListByUserID(
	_ context.Context,
	userID uuid.UUID,
) ([]*model.AuthSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	sessions := []*model.AuthSession{}

	for _, session := range f.sessions {
		if session.UserID == userID {
			cp := *session
			sessions = append(sessions, &cp)
		}
	}

	return sessions, nil
}

func (f *fakeSessionStore) Revoke(
	_ context.Context,
	id uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	session, ok := f.sessions[id]
	if !ok || session.Revoked() {
		return repository.ErrSessionAlreadyRevoked
	}

	now := time.Now()
	session.RevokedAt = &now

	return nil
}

func (f *fakeSessionStore) Rotate(
	_ context.Context,
	oldID uuid.UUID,
	replacement *model.AuthSession,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	old, ok := f.sessions[oldID]
	if !ok || old.Revoked() {
		return repository.ErrSessionAlreadyRevoked
	}

	if _, exists := f.byHash[replacement.RefreshTokenHash]; exists {
		return repository.ErrRefreshTokenExists
	}

	now := time.Now()
	replacement.ID = uuid.New()
	replacement.CreatedAt = now
	replacement.LastUsedAt = now

	stored := *replacement
	f.sessions[replacement.ID] = &stored
	f.byHash[replacement.RefreshTokenHash] = &stored

	old.RevokedAt = &now
	old.ReplacedBy = &replacement.ID

	return nil
}

func (f *fakeSessionStore) UpdateLastUsed(
	_ context.Context,
	id uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	session, ok := f.sessions[id]
	if !ok {
		return repository.ErrSessionNotFound
	}

	session.LastUsedAt = time.Now()

	return nil
}

func (f *fakeSessionStore) RevokeAllForUser(
	_ context.Context,
	userID uuid.UUID,
) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var count int64

	for _, session := range f.sessions {
		if session.UserID == userID && !session.Revoked() {
			now := time.Now()
			session.RevokedAt = &now
			count++
		}
	}

	return count, nil
}

func newTestSessionService(store SessionStore) *SessionService {
	svc, err := NewSessionService(store, SessionConfig{
		AccessSecret: "test-access-secret-0123456789abcdef",
		Issuer:       "edvance-auth",
		Audience:     "edvance-api",
		AccessTTL:    15 * time.Minute,
		RefreshTTL:   720 * time.Hour,
	})
	if err != nil {
		panic(err)
	}

	return svc
}

func TestCreateSession(t *testing.T) {
	svc := newTestSessionService(newFakeSessionStore())
	ctx := context.Background()

	out, err := svc.CreateSession(ctx, CreateSessionInput{
		UserID:    uuid.New(),
		UserAgent: "test-agent",
		IPAddress: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if out.AccessToken == "" || out.RefreshToken == "" {
		t.Fatal("expected both tokens to be issued")
	}

	if out.Session.ID == uuid.Nil {
		t.Fatal("expected session to have an ID")
	}

	// Only the hash may be stored.
	if out.Session.RefreshTokenHash == out.RefreshToken {
		t.Fatal("raw refresh token must not be stored")
	}

	if out.Session.RefreshTokenHash != token.HashRefreshToken(out.RefreshToken) {
		t.Fatal("stored hash does not match issued refresh token")
	}

	// The access token must validate and carry the user as subject.
	claims, err := validateTestAccessToken(out.AccessToken)
	if err != nil {
		t.Fatalf("issued access token does not validate: %v", err)
	}

	if claims.Subject != out.Session.UserID.String() {
		t.Fatal("access token subject does not match session user")
	}
}

func TestRefreshSessionRotation(t *testing.T) {
	store := newFakeSessionStore()
	svc := newTestSessionService(store)
	ctx := context.Background()
	userID := uuid.New()

	created, err := svc.CreateSession(ctx, CreateSessionInput{UserID: userID})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	refreshed, err := svc.RefreshSession(ctx, RefreshSessionInput{
		RefreshToken: created.RefreshToken,
	})
	if err != nil {
		t.Fatalf("refresh session: %v", err)
	}

	if refreshed.RefreshToken == created.RefreshToken {
		t.Fatal("rotation must issue a new refresh token")
	}

	if refreshed.Session.ID == created.Session.ID {
		t.Fatal("rotation must create a replacement session")
	}

	// Old session must be revoked and linked to the replacement.
	old, err := store.FindByID(ctx, created.Session.ID)
	if err != nil {
		t.Fatalf("find old session: %v", err)
	}

	if !old.Revoked() {
		t.Fatal("old session must be revoked after rotation")
	}

	if old.ReplacedBy == nil || *old.ReplacedBy != refreshed.Session.ID {
		t.Fatal("old session must link to its replacement")
	}
}

func TestRefreshSessionReuseDetected(t *testing.T) {
	svc := newTestSessionService(newFakeSessionStore())
	ctx := context.Background()

	created, err := svc.CreateSession(ctx, CreateSessionInput{
		UserID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if _, err := svc.RefreshSession(ctx, RefreshSessionInput{
		RefreshToken: created.RefreshToken,
	}); err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	// Replaying the old (now revoked) refresh token must be detected.
	_, err = svc.RefreshSession(ctx, RefreshSessionInput{
		RefreshToken: created.RefreshToken,
	})
	if !errors.Is(err, ErrRefreshTokenReused) {
		t.Fatalf("expected reuse detection, got: %v", err)
	}
}

func TestRefreshSessionUnknownToken(t *testing.T) {
	svc := newTestSessionService(newFakeSessionStore())

	_, err := svc.RefreshSession(context.Background(), RefreshSessionInput{
		RefreshToken: "does-not-exist",
	})
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected not-found, got: %v", err)
	}
}

func TestRevokeSession(t *testing.T) {
	store := newFakeSessionStore()
	svc := newTestSessionService(store)
	ctx := context.Background()
	userID := uuid.New()

	created, err := svc.CreateSession(ctx, CreateSessionInput{UserID: userID})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := svc.RevokeSession(ctx, created.Session.ID, userID); err != nil {
		t.Fatalf("revoke session: %v", err)
	}

	// Logout is idempotent.
	if err := svc.RevokeSession(ctx, created.Session.ID, userID); err != nil {
		t.Fatalf("second revoke should succeed: %v", err)
	}

	// A revoked session's refresh token must not rotate.
	_, err = svc.RefreshSession(ctx, RefreshSessionInput{
		RefreshToken: created.RefreshToken,
	})
	if !errors.Is(err, ErrRefreshTokenReused) {
		t.Fatalf("expected reuse detection after logout, got: %v", err)
	}
}

func TestRevokeSessionWrongUser(t *testing.T) {
	svc := newTestSessionService(newFakeSessionStore())
	ctx := context.Background()

	created, err := svc.CreateSession(ctx, CreateSessionInput{
		UserID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	err = svc.RevokeSession(ctx, created.Session.ID, uuid.New())
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expected not-found for foreign session, got: %v", err)
	}
}

func TestRevokeAllSessions(t *testing.T) {
	svc := newTestSessionService(newFakeSessionStore())
	ctx := context.Background()
	userID := uuid.New()

	for i := 0; i < 3; i++ {
		if _, err := svc.CreateSession(ctx, CreateSessionInput{
			UserID: userID,
		}); err != nil {
			t.Fatalf("create session: %v", err)
		}
	}

	other, err := svc.CreateSession(ctx, CreateSessionInput{
		UserID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	count, err := svc.RevokeAllSessions(ctx, userID)
	if err != nil {
		t.Fatalf("revoke all sessions: %v", err)
	}

	if count != 3 {
		t.Fatalf("expected 3 revoked sessions, got %d", count)
	}

	// The other user's session must be untouched: it still rotates.
	if _, err := svc.RefreshSession(ctx, RefreshSessionInput{
		RefreshToken: other.RefreshToken,
	}); err != nil {
		t.Fatalf("other user session should still work: %v", err)
	}
}

func validateTestAccessToken(signed string) (*token.AccessClaims, error) {
	return token.ValidateAccessToken(
		signed,
		"test-access-secret-0123456789abcdef",
		"edvance-auth",
		"edvance-api",
	)
}
