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

	"github.com/Anshul563/edvance-project/services/creator-service/internal/model"
)

// Needs PostgreSQL with migration 001 applied to edvance_creator:
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

func cleanupCreator(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM creator_channels WHERE creator_id IN (
				SELECT id FROM creators WHERE user_id = $1
			)`,
			userID,
		)
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM creators WHERE user_id = $1`,
			userID,
		)
	})
}

func TestCreatorOnboardAndFind(t *testing.T) {
	pool := newTestPool(t)
	repo := NewCreatorRepository(pool)
	channels := NewChannelRepository(pool)
	ctx := context.Background()
	userID := uuid.New()
	cleanupCreator(t, pool, userID)

	creator := &model.Creator{
		UserID:      userID,
		Status:      model.CreatorStatusActive,
		DisplayName: "Tester",
	}
	channel := &model.Channel{
		Handle: "tester_channel",
		Name:   "Tester Channel",
		Status: "active",
	}

	if err := repo.Onboard(ctx, creator, channel); err != nil {
		t.Fatalf("onboard: %v", err)
	}

	if creator.ID == uuid.Nil || channel.ID == uuid.Nil {
		t.Fatal("expected ids to be set")
	}

	if channel.CreatorID != creator.ID {
		t.Fatal("channel must link to creator")
	}

	found, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("find by user: %v", err)
	}

	if found.ID != creator.ID {
		t.Fatal("wrong creator")
	}

	ch, err := channels.FindChannelByCreatorID(ctx, creator.ID)
	if err != nil {
		t.Fatalf("find channel: %v", err)
	}

	if ch.Handle != "tester_channel" {
		t.Fatal("wrong channel")
	}

	byHandle, err := channels.FindByHandle(ctx, "tester_channel")
	if err != nil {
		t.Fatalf("find by handle: %v", err)
	}

	if byHandle.ID != ch.ID {
		t.Fatal("wrong channel by handle")
	}

	exists, err := repo.CreatorExists(ctx, userID)
	if err != nil {
		t.Fatalf("exists: %v", err)
	}

	if !exists {
		t.Fatal("expected creator to exist")
	}

	handleExists, err := channels.HandleExists(ctx, "tester_channel")
	if err != nil {
		t.Fatalf("handle exists: %v", err)
	}

	if !handleExists {
		t.Fatal("expected handle to exist")
	}
}

func TestCreatorUniqueUserID(t *testing.T) {
	pool := newTestPool(t)
	repo := NewCreatorRepository(pool)
	ctx := context.Background()
	userID := uuid.New()
	cleanupCreator(t, pool, userID)

	first := &model.Creator{
		UserID:      userID,
		Status:      model.CreatorStatusActive,
		DisplayName: "First",
	}
	firstChannel := &model.Channel{Handle: "first_handle", Name: "First", Status: "active"}

	if err := repo.Onboard(ctx, first, firstChannel); err != nil {
		t.Fatalf("first onboard: %v", err)
	}

	second := &model.Creator{
		UserID:      userID,
		Status:      model.CreatorStatusActive,
		DisplayName: "Second",
	}
	secondChannel := &model.Channel{Handle: "second_handle", Name: "Second", Status: "active"}

	if err := repo.Onboard(ctx, second, secondChannel); !errors.Is(
		err,
		ErrCreatorExists,
	) {
		t.Fatalf("expected exists, got %v", err)
	}
}

func TestChannelUniqueHandle(t *testing.T) {
	pool := newTestPool(t)
	repo := NewCreatorRepository(pool)
	ctx := context.Background()
	userA := uuid.New()
	userB := uuid.New()
	cleanupCreator(t, pool, userA)
	cleanupCreator(t, pool, userB)

	creatorA := &model.Creator{
		UserID:      userA,
		Status:      model.CreatorStatusActive,
		DisplayName: "A",
	}

	if err := repo.Onboard(
		ctx,
		creatorA,
		&model.Channel{Handle: "shared_handle", Name: "A", Status: "active"},
	); err != nil {
		t.Fatalf("first onboard: %v", err)
	}

	creatorB := &model.Creator{
		UserID:      userB,
		Status:      model.CreatorStatusActive,
		DisplayName: "B",
	}

	// Same handle from another user: the whole onboard (creator AND
	// channel) must roll back — no partial creator may survive.
	if err := repo.Onboard(
		ctx,
		creatorB,
		&model.Channel{Handle: "shared_handle", Name: "B", Status: "active"},
	); !errors.Is(err, ErrHandleTaken) {
		t.Fatalf("expected taken, got %v", err)
	}

	if _, err := repo.FindByUserID(ctx, userB); !errors.Is(
		err,
		ErrCreatorNotFound,
	) {
		t.Fatalf("partial creator must not survive rollback, got %v", err)
	}
}

func TestCreatorUpdate(t *testing.T) {
	pool := newTestPool(t)
	repo := NewCreatorRepository(pool)
	channels := NewChannelRepository(pool)
	ctx := context.Background()
	userID := uuid.New()
	cleanupCreator(t, pool, userID)

	creator := &model.Creator{
		UserID:      userID,
		Status:      model.CreatorStatusActive,
		DisplayName: "Before",
	}

	if err := repo.Onboard(
		ctx,
		creator,
		&model.Channel{Handle: "update_handle", Name: "Before", Status: "active"},
	); err != nil {
		t.Fatalf("onboard: %v", err)
	}

	creator.DisplayName = "After"
	creator.Status = model.CreatorStatusSuspended

	if err := repo.UpdateCreator(ctx, creator); err != nil {
		t.Fatalf("update: %v", err)
	}

	reloaded, err := repo.FindByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}

	if reloaded.DisplayName != "After" || reloaded.Status != model.CreatorStatusSuspended {
		t.Fatal("update did not persist")
	}

	ch, err := channels.FindChannelByCreatorID(ctx, creator.ID)
	if err != nil {
		t.Fatalf("find channel: %v", err)
	}

	ch.Name = "After Channel"

	if err := channels.UpdateChannel(ctx, ch); err != nil {
		t.Fatalf("update channel: %v", err)
	}

	if _, err := channels.FindByHandle(ctx, "update_handle"); err != nil {
		t.Fatalf("handle lookup after update: %v", err)
	}
}
