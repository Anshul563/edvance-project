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

	"github.com/Anshul563/edvance-project/services/auth-service/internal/model"
)

// Needs PostgreSQL with migrations 001-004 applied:
//
//	DATABASE_URL=postgres://... go test -tags integration ./internal/repository/
func newResetTestPool(t *testing.T) *pgxpool.Pool {
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

func createResetTestUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	var id uuid.UUID

	err := pool.QueryRow(
		context.Background(),
		`INSERT INTO users (email, username, password_hash, display_name)
		 VALUES ('reset-test-' || gen_random_uuid()::text || '@example.com',
		         'rst_' || substr(gen_random_uuid()::text, 1, 12),
		         'hash', 'Reset Test')
		 RETURNING id`,
	).Scan(&id)
	if err != nil {
		t.Fatalf("create test user: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM users WHERE id = $1`,
			id,
		)
	})

	return id
}

func createResetSession(
	t *testing.T,
	pool *pgxpool.Pool,
	userID uuid.UUID,
	hash string,
) {
	t.Helper()

	_, err := pool.Exec(
		context.Background(),
		`INSERT INTO auth_sessions (user_id, refresh_token_hash, expires_at)
		 VALUES ($1, $2, NOW() + INTERVAL '30 days')`,
		userID,
		hash,
	)
	if err != nil {
		t.Fatalf("create test session: %v", err)
	}
}

func TestResetRepositoryCreateAndFind(t *testing.T) {
	pool := newResetTestPool(t)
	repo := NewPasswordResetRepository(pool)
	ctx := context.Background()
	userID := createResetTestUser(t, pool)

	record := &model.PasswordResetToken{
		UserID:    userID,
		TokenHash: "rhash-create-find",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}

	if err := repo.Create(ctx, record); err != nil {
		t.Fatalf("create: %v", err)
	}

	if record.ID == uuid.Nil {
		t.Fatal("expected id to be set")
	}

	found, err := repo.FindByTokenHash(ctx, "rhash-create-find")
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if found.ID != record.ID || found.UserID != userID {
		t.Fatal("found record mismatch")
	}

	if _, err := repo.FindByTokenHash(ctx, "missing"); !errors.Is(
		err,
		ErrResetTokenNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestResetRepositoryHashUniqueness(t *testing.T) {
	pool := newResetTestPool(t)
	repo := NewPasswordResetRepository(pool)
	ctx := context.Background()
	userID := createResetTestUser(t, pool)

	first := &model.PasswordResetToken{
		UserID:    userID,
		TokenHash: "rhash-unique",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}

	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("create first: %v", err)
	}

	second := &model.PasswordResetToken{
		UserID:    userID,
		TokenHash: "rhash-unique",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}

	if err := repo.Create(ctx, second); !errors.Is(err, ErrResetTokenExists) {
		t.Fatalf("expected exists error, got %v", err)
	}
}

func TestResetRepositoryMarkUsed(t *testing.T) {
	pool := newResetTestPool(t)
	repo := NewPasswordResetRepository(pool)
	ctx := context.Background()
	userID := createResetTestUser(t, pool)

	record := &model.PasswordResetToken{
		UserID:    userID,
		TokenHash: "rhash-mark-used",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}

	if err := repo.Create(ctx, record); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.MarkUsed(ctx, record.ID); err != nil {
		t.Fatalf("mark used: %v", err)
	}

	if err := repo.MarkUsed(ctx, record.ID); !errors.Is(
		err,
		ErrResetTokenUsed,
	) {
		t.Fatalf("expected used error, got %v", err)
	}
}

func TestResetRepositoryInvalidateForUser(t *testing.T) {
	pool := newResetTestPool(t)
	repo := NewPasswordResetRepository(pool)
	ctx := context.Background()
	userID := createResetTestUser(t, pool)
	otherID := createResetTestUser(t, pool)

	for _, hash := range []string{"rhash-inv-1", "rhash-inv-2"} {
		record := &model.PasswordResetToken{
			UserID:    userID,
			TokenHash: hash,
			ExpiresAt: time.Now().Add(30 * time.Minute),
		}

		if err := repo.Create(ctx, record); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	other := &model.PasswordResetToken{
		UserID:    otherID,
		TokenHash: "rhash-inv-other",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}

	if err := repo.Create(ctx, other); err != nil {
		t.Fatalf("create other: %v", err)
	}

	count, err := repo.InvalidateForUser(ctx, userID)
	if err != nil {
		t.Fatalf("invalidate: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected 2 invalidated, got %d", count)
	}

	if _, err := repo.FindByTokenHash(ctx, "rhash-inv-other"); err != nil {
		t.Fatal("other user's token must survive")
	}
}

func TestResetRepositoryCompleteReset(t *testing.T) {
	pool := newResetTestPool(t)
	repo := NewPasswordResetRepository(pool)
	ctx := context.Background()
	userID := createResetTestUser(t, pool)

	createResetSession(t, pool, userID, "rhash-session-1")
	createResetSession(t, pool, userID, "rhash-session-2")

	record := &model.PasswordResetToken{
		UserID:    userID,
		TokenHash: "rhash-complete",
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}

	if err := repo.Create(ctx, record); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.CompleteReset(ctx, record.ID, userID, "new-hash"); err != nil {
		t.Fatalf("complete: %v", err)
	}

	// All three effects landed together.
	found, err := repo.FindByTokenHash(ctx, "rhash-complete")
	if err != nil {
		t.Fatalf("find token: %v", err)
	}

	if !found.Used() {
		t.Fatal("expected token to be used")
	}

	var passwordHash string
	var activeSessions int

	err = pool.QueryRow(
		ctx,
		`SELECT password_hash FROM users WHERE id = $1`,
		userID,
	).Scan(&passwordHash)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}

	if passwordHash != "new-hash" {
		t.Fatal("expected password hash to be updated")
	}

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*) FROM auth_sessions
		  WHERE user_id = $1 AND revoked_at IS NULL`,
		userID,
	).Scan(&activeSessions)
	if err != nil {
		t.Fatalf("count sessions: %v", err)
	}

	if activeSessions != 0 {
		t.Fatalf("expected all sessions revoked, %d still active", activeSessions)
	}

	// Second completion loses the race.
	if err := repo.CompleteReset(
		ctx,
		record.ID,
		userID,
		"another-hash",
	); !errors.Is(err, ErrResetTokenUsed) {
		t.Fatalf("expected used error, got %v", err)
	}
}
