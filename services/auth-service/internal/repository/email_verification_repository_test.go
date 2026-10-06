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

// Needs PostgreSQL with migrations 001-003 applied:
//
//	DATABASE_URL=postgres://... go test -tags integration ./internal/repository/
func newVerificationTestPool(t *testing.T) *pgxpool.Pool {
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

func createVerificationTestUser(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()

	var id uuid.UUID

	err := pool.QueryRow(
		context.Background(),
		`INSERT INTO users (email, username, password_hash, display_name)
		 VALUES ('verify-test-' || gen_random_uuid()::text || '@example.com',
		         'ver_' || substr(gen_random_uuid()::text, 1, 12),
		         'hash', 'Verify Test')
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

func TestVerificationRepositoryCreateAndFind(t *testing.T) {
	pool := newVerificationTestPool(t)
	repo := NewEmailVerificationRepository(pool)
	ctx := context.Background()
	userID := createVerificationTestUser(t, pool)

	record := &model.EmailVerificationToken{
		UserID:    userID,
		TokenHash: "vhash-create-find",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := repo.Create(ctx, record); err != nil {
		t.Fatalf("create: %v", err)
	}

	if record.ID == uuid.Nil {
		t.Fatal("expected id to be set")
	}

	found, err := repo.FindByTokenHash(ctx, "vhash-create-find")
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if found.ID != record.ID || found.UserID != userID {
		t.Fatal("found record mismatch")
	}

	if _, err := repo.FindByTokenHash(ctx, "missing"); !errors.Is(
		err,
		ErrVerificationTokenNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestVerificationRepositoryHashUniqueness(t *testing.T) {
	pool := newVerificationTestPool(t)
	repo := NewEmailVerificationRepository(pool)
	ctx := context.Background()
	userID := createVerificationTestUser(t, pool)

	first := &model.EmailVerificationToken{
		UserID:    userID,
		TokenHash: "vhash-unique",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("create first: %v", err)
	}

	second := &model.EmailVerificationToken{
		UserID:    userID,
		TokenHash: "vhash-unique",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := repo.Create(ctx, second); !errors.Is(
		err,
		ErrVerificationTokenExists,
	) {
		t.Fatalf("expected exists error, got %v", err)
	}
}

func TestVerificationRepositoryDeleteForUser(t *testing.T) {
	pool := newVerificationTestPool(t)
	repo := NewEmailVerificationRepository(pool)
	ctx := context.Background()
	userID := createVerificationTestUser(t, pool)
	otherID := createVerificationTestUser(t, pool)

	for _, hash := range []string{"vhash-del-1", "vhash-del-2"} {
		record := &model.EmailVerificationToken{
			UserID:    userID,
			TokenHash: hash,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		if err := repo.Create(ctx, record); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	other := &model.EmailVerificationToken{
		UserID:    otherID,
		TokenHash: "vhash-del-other",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := repo.Create(ctx, other); err != nil {
		t.Fatalf("create other: %v", err)
	}

	count, err := repo.DeleteForUser(ctx, userID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected 2 deleted, got %d", count)
	}

	if _, err := repo.FindByTokenHash(ctx, "vhash-del-other"); err != nil {
		t.Fatal("other user's token must survive")
	}
}

func TestVerificationRepositoryConsume(t *testing.T) {
	pool := newVerificationTestPool(t)
	repo := NewEmailVerificationRepository(pool)
	ctx := context.Background()
	userID := createVerificationTestUser(t, pool)

	record := &model.EmailVerificationToken{
		UserID:    userID,
		TokenHash: "vhash-consume",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := repo.Create(ctx, record); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := repo.Consume(ctx, record.ID, userID); err != nil {
		t.Fatalf("consume: %v", err)
	}

	// Token and user flipped together.
	found, err := repo.FindByTokenHash(ctx, "vhash-consume")
	if err != nil {
		t.Fatalf("find: %v", err)
	}

	if !found.Used() {
		t.Fatal("expected token to be used")
	}

	var verified bool

	err = pool.QueryRow(
		ctx,
		`SELECT email_verified FROM users WHERE id = $1`,
		userID,
	).Scan(&verified)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}

	if !verified {
		t.Fatal("expected user to be verified")
	}

	// Second consume loses the race.
	if err := repo.Consume(ctx, record.ID, userID); !errors.Is(
		err,
		ErrVerificationTokenUsed,
	) {
		t.Fatalf("expected used error, got %v", err)
	}
}

func TestVerificationRepositoryConsumeMissingUser(t *testing.T) {
	pool := newVerificationTestPool(t)
	repo := NewEmailVerificationRepository(pool)
	ctx := context.Background()

	// A token row whose user is gone: Consume must fail without marking
	// the token used, keeping state consistent.
	userID := createVerificationTestUser(t, pool)

	record := &model.EmailVerificationToken{
		UserID:    userID,
		TokenHash: "vhash-orphan-consume",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := repo.Create(ctx, record); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := pool.Exec(
		ctx,
		`DELETE FROM users WHERE id = $1`,
		userID,
	); err != nil {
		t.Fatalf("delete user: %v", err)
	}

	// Cascade removes the token too; consuming the stale id must fail.
	if err := repo.Consume(ctx, record.ID, userID); err == nil {
		t.Fatal("expected consume to fail for missing token/user")
	}
}
