//go:build integration

package repository

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/model"
)

// These tests need a real PostgreSQL with migrations 001 and 002 applied:
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

func createTestUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	var id uuid.UUID

	err := pool.QueryRow(
		context.Background(),
		`INSERT INTO users (email, username, password_hash, display_name)
		 VALUES ('session-test-' || gen_random_uuid()::text || '@example.com',
		         'sess_' || substr(gen_random_uuid()::text, 1, 12),
		         'hash', 'Session Test')
		 RETURNING id`,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	return id
}

func TestSessionRepositoryCreateAndFind(t *testing.T) {
	pool := newTestPool(t)
	repo := NewSessionRepository(pool)
	ctx := context.Background()
	userID := createTestUser(t, pool)

	session := &model.AuthSession{
		UserID:           userID,
		RefreshTokenHash: "hash-create-find",
		ExpiresAt:        time.Now().Add(720 * time.Hour),
	}

	if err := repo.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if session.ID == uuid.Nil {
		t.Fatal("expected session ID to be set")
	}

	byHash, err := repo.FindByRefreshTokenHash(ctx, "hash-create-find")
	if err != nil {
		t.Fatalf("find by hash: %v", err)
	}

	if byHash.ID != session.ID || byHash.UserID != userID {
		t.Fatal("found session does not match created session")
	}

	byID, err := repo.FindByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}

	if byID.RefreshTokenHash != "hash-create-find" {
		t.Fatal("found session hash mismatch")
	}
}

func TestSessionRepositoryHashUniqueness(t *testing.T) {
	pool := newTestPool(t)
	repo := NewSessionRepository(pool)
	ctx := context.Background()
	userID := createTestUser(t, pool)

	first := &model.AuthSession{
		UserID:           userID,
		RefreshTokenHash: "hash-unique-test",
		ExpiresAt:        time.Now().Add(720 * time.Hour),
	}

	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("create first session: %v", err)
	}

	second := &model.AuthSession{
		UserID:           userID,
		RefreshTokenHash: "hash-unique-test",
		ExpiresAt:        time.Now().Add(720 * time.Hour),
	}

	if err := repo.Create(ctx, second); err == nil {
		t.Fatal("expected duplicate refresh token hash to be rejected")
	}
}

func TestSessionRepositoryRevoke(t *testing.T) {
	pool := newTestPool(t)
	repo := NewSessionRepository(pool)
	ctx := context.Background()
	userID := createTestUser(t, pool)

	session := &model.AuthSession{
		UserID:           userID,
		RefreshTokenHash: "hash-revoke-test",
		ExpiresAt:        time.Now().Add(720 * time.Hour),
	}

	if err := repo.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := repo.Revoke(ctx, session.ID); err != nil {
		t.Fatalf("revoke session: %v", err)
	}

	found, err := repo.FindByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("find session: %v", err)
	}

	if !found.Revoked() {
		t.Fatal("expected session to be revoked")
	}

	if err := repo.Revoke(ctx, session.ID); err == nil {
		t.Fatal("expected second revoke to report already revoked")
	}
}

func TestSessionRepositoryUpdateLastUsed(t *testing.T) {
	pool := newTestPool(t)
	repo := NewSessionRepository(pool)
	ctx := context.Background()
	userID := createTestUser(t, pool)

	session := &model.AuthSession{
		UserID:           userID,
		RefreshTokenHash: "hash-last-used-test",
		ExpiresAt:        time.Now().Add(720 * time.Hour),
	}

	if err := repo.Create(ctx, session); err != nil {
		t.Fatalf("create session: %v", err)
	}

	before := session.LastUsedAt

	time.Sleep(10 * time.Millisecond)

	if err := repo.UpdateLastUsed(ctx, session.ID); err != nil {
		t.Fatalf("update last used: %v", err)
	}

	found, err := repo.FindByID(ctx, session.ID)
	if err != nil {
		t.Fatalf("find session: %v", err)
	}

	if !found.LastUsedAt.After(before) {
		t.Fatal("expected last_used_at to advance")
	}
}

func TestSessionRepositoryRevokeAllForUser(t *testing.T) {
	pool := newTestPool(t)
	repo := NewSessionRepository(pool)
	ctx := context.Background()
	userID := createTestUser(t, pool)
	otherUserID := createTestUser(t, pool)

	for i := 0; i < 3; i++ {
		session := &model.AuthSession{
			UserID:           userID,
			RefreshTokenHash: "hash-revoke-all-" + uuid.NewString(),
			ExpiresAt:        time.Now().Add(720 * time.Hour),
		}

		if err := repo.Create(ctx, session); err != nil {
			t.Fatalf("create session: %v", err)
		}
	}

	other := &model.AuthSession{
		UserID:           otherUserID,
		RefreshTokenHash: "hash-revoke-all-other",
		ExpiresAt:        time.Now().Add(720 * time.Hour),
	}

	if err := repo.Create(ctx, other); err != nil {
		t.Fatalf("create other session: %v", err)
	}

	count, err := repo.RevokeAllForUser(ctx, userID)
	if err != nil {
		t.Fatalf("revoke all: %v", err)
	}

	if count != 3 {
		t.Fatalf("expected 3 revoked sessions, got %d", count)
	}

	untouched, err := repo.FindByID(ctx, other.ID)
	if err != nil {
		t.Fatalf("find other session: %v", err)
	}

	if untouched.Revoked() {
		t.Fatal("other user's session must not be revoked")
	}
}

func TestSessionRepositoryRotate(t *testing.T) {
	pool := newTestPool(t)
	repo := NewSessionRepository(pool)
	ctx := context.Background()
	userID := createTestUser(t, pool)

	old := &model.AuthSession{
		UserID:           userID,
		RefreshTokenHash: "hash-rotate-old",
		ExpiresAt:        time.Now().Add(720 * time.Hour),
	}

	if err := repo.Create(ctx, old); err != nil {
		t.Fatalf("create session: %v", err)
	}

	replacement := &model.AuthSession{
		UserID:           userID,
		RefreshTokenHash: "hash-rotate-new",
		ExpiresAt:        time.Now().Add(720 * time.Hour),
	}

	if err := repo.Rotate(ctx, old.ID, replacement); err != nil {
		t.Fatalf("rotate: %v", err)
	}

	updated, err := repo.FindByID(ctx, old.ID)
	if err != nil {
		t.Fatalf("find old session: %v", err)
	}

	if !updated.Revoked() {
		t.Fatal("expected old session to be revoked")
	}

	if updated.ReplacedBy == nil || *updated.ReplacedBy != replacement.ID {
		t.Fatal("expected old session to link to replacement")
	}
}
