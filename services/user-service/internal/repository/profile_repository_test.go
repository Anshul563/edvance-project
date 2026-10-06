//go:build integration

package repository

import (
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Anshul563/edvance-project/services/user-service/internal/model"
)

// Needs PostgreSQL with migration 001 applied to edvance_user:
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

func seedProfile(
	t *testing.T,
	pool *pgxpool.Pool,
	userID uuid.UUID,
	username string,
) {
	t.Helper()

	_, err := pool.Exec(
		context.Background(),
		`INSERT INTO user_profiles (user_id, username, display_name)
		 VALUES ($1, $2, 'Seed')`,
		userID,
		username,
	)
	if err != nil {
		t.Fatalf("seed profile: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM user_profiles WHERE user_id = $1`,
			userID,
		)
	})
}

func TestProfileRepositoryCRUD(t *testing.T) {
	pool := newTestPool(t)
	repo := NewProfileRepository(pool)
	ctx := context.Background()
	userID := uuid.New()

	profile := &model.UserProfile{
		UserID:      userID,
		Username:    "crud_tester",
		DisplayName: "CRUD Tester",
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM user_profiles WHERE user_id = $1`,
			userID,
		)
	})

	if err := repo.Create(ctx, profile); err != nil {
		t.Fatalf("create: %v", err)
	}

	if profile.ID == uuid.Nil {
		t.Fatal("expected id to be set")
	}

	byUser, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("find by user: %v", err)
	}

	if byUser.ID != profile.ID {
		t.Fatal("wrong profile by user id")
	}

	byName, err := repo.FindByUsername(ctx, "crud_tester")
	if err != nil {
		t.Fatalf("find by username: %v", err)
	}

	if byName.ID != profile.ID {
		t.Fatal("wrong profile by username")
	}

	exists, err := repo.UsernameExists(ctx, "crud_tester")
	if err != nil {
		t.Fatalf("exists: %v", err)
	}

	if !exists {
		t.Fatal("expected username to exist")
	}

	byUser.DisplayName = "Updated Name"

	if err := repo.Update(ctx, byUser); err != nil {
		t.Fatalf("update: %v", err)
	}

	reloaded, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	if reloaded.DisplayName != "Updated Name" {
		t.Fatal("update did not persist")
	}

	if err := repo.Delete(ctx, profile.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := repo.FindByUserID(ctx, userID); !errors.Is(
		err,
		ErrProfileNotFound,
	) {
		t.Fatalf("expected not-found after delete, got %v", err)
	}
}

func TestProfileRepositoryUniqueness(t *testing.T) {
	pool := newTestPool(t)
	repo := NewProfileRepository(pool)
	ctx := context.Background()
	userID := uuid.New()
	seedProfile(t, pool, userID, "unique_name")

	dupName := &model.UserProfile{
		UserID:      uuid.New(),
		Username:    "unique_name",
		DisplayName: "Dup",
	}

	if err := repo.Create(ctx, dupName); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("expected taken, got %v", err)
	}

	dupUser := &model.UserProfile{
		UserID:      userID,
		Username:    "other_name",
		DisplayName: "Dup",
	}

	if err := repo.Create(ctx, dupUser); !errors.Is(err, ErrProfileExists) {
		t.Fatalf("expected exists, got %v", err)
	}
}

func TestProfileRepositoryConcurrentUsernameClaim(t *testing.T) {
	pool := newTestPool(t)
	repo := NewProfileRepository(pool)
	ctx := context.Background()

	const racers = 10

	var successes atomic.Int32
	var taken atomic.Int32

	var wg sync.WaitGroup

	for i := 0; i < racers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			userID := uuid.New()

			t.Cleanup(func() {
				_, _ = pool.Exec(
					context.Background(),
					`DELETE FROM user_profiles WHERE user_id = $1`,
					userID,
				)
			})

			err := repo.Create(ctx, &model.UserProfile{
				UserID:      userID,
				Username:    "race_winner",
				DisplayName: "Racer",
			})

			switch {
			case err == nil:
				successes.Add(1)

			case errors.Is(err, ErrUsernameTaken):
				taken.Add(1)

			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}

	wg.Wait()

	if successes.Load() != 1 {
		t.Fatalf("expected exactly 1 winner, got %d", successes.Load())
	}

	if taken.Load() != racers-1 {
		t.Fatalf("expected %d losers, got %d", racers-1, taken.Load())
	}
}
