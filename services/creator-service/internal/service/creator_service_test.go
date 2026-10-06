package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/creator-service/internal/model"
	"github.com/Anshul563/edvance-project/services/creator-service/internal/repository"
)

// fakeCreatorStore is an in-memory CreatorStore.
type fakeCreatorStore struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]*model.Creator
	byUser map[uuid.UUID]*model.Creator
}

func newFakeCreatorStore() *fakeCreatorStore {
	return &fakeCreatorStore{
		byID:   make(map[uuid.UUID]*model.Creator),
		byUser: make(map[uuid.UUID]*model.Creator),
	}
}

func (f *fakeCreatorStore) Onboard(
	_ context.Context,
	creator *model.Creator,
	channel *model.Channel,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.byUser[creator.UserID]; exists {
		return repository.ErrCreatorExists
	}

	creator.ID = uuid.New()
	creator.CreatedAt = time.Now()
	creator.UpdatedAt = time.Now()

	stored := *creator
	f.byID[creator.ID] = &stored
	f.byUser[creator.UserID] = &stored

	// Channel persistence is the channel fake's job; mirror the link.
	channel.CreatorID = creator.ID

	return nil
}

func (f *fakeCreatorStore) FindByUserID(
	_ context.Context,
	userID uuid.UUID,
) (*model.Creator, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	creator, ok := f.byUser[userID]
	if !ok {
		return nil, repository.ErrCreatorNotFound
	}

	cp := *creator

	return &cp, nil
}

func (f *fakeCreatorStore) FindByID(
	_ context.Context,
	id uuid.UUID,
) (*model.Creator, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	creator, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrCreatorNotFound
	}

	cp := *creator

	return &cp, nil
}

func (f *fakeCreatorStore) UpdateCreator(
	_ context.Context,
	creator *model.Creator,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	stored, ok := f.byID[creator.ID]
	if !ok {
		return repository.ErrCreatorNotFound
	}

	creator.UpdatedAt = time.Now()
	cp := *creator
	f.byID[creator.ID] = &cp
	f.byUser[creator.UserID] = &cp
	_ = stored

	return nil
}

func (f *fakeCreatorStore) CreatorExists(
	_ context.Context,
	userID uuid.UUID,
) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	_, ok := f.byUser[userID]

	return ok, nil
}

// fakeChannelStore is an in-memory ChannelStore.
type fakeChannelStore struct {
	mu         sync.Mutex
	byID       map[uuid.UUID]*model.Channel
	byCreator  map[uuid.UUID]*model.Channel
	byHandle   map[string]*model.Channel
	failCreate bool
}

func newFakeChannelStore() *fakeChannelStore {
	return &fakeChannelStore{
		byID:      make(map[uuid.UUID]*model.Channel),
		byCreator: make(map[uuid.UUID]*model.Channel),
		byHandle:  make(map[string]*model.Channel),
	}
}

// persist mirrors what the transactional Onboard writes.
func (f *fakeChannelStore) persist(channel *model.Channel) {
	channel.ID = uuid.New()
	channel.CreatedAt = time.Now()
	channel.UpdatedAt = time.Now()

	stored := *channel
	f.byID[channel.ID] = &stored
	f.byCreator[channel.CreatorID] = &stored
	f.byHandle[channel.Handle] = &stored
}

func (f *fakeChannelStore) FindChannelByCreatorID(
	_ context.Context,
	creatorID uuid.UUID,
) (*model.Channel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	channel, ok := f.byCreator[creatorID]
	if !ok {
		return nil, repository.ErrChannelNotFound
	}

	cp := *channel

	return &cp, nil
}

func (f *fakeChannelStore) FindByHandle(
	_ context.Context,
	handle string,
) (*model.Channel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	channel, ok := f.byHandle[handle]
	if !ok {
		return nil, repository.ErrChannelNotFound
	}

	cp := *channel

	return &cp, nil
}

func (f *fakeChannelStore) UpdateChannel(
	_ context.Context,
	channel *model.Channel,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	stored, ok := f.byID[channel.ID]
	if !ok {
		return repository.ErrChannelNotFound
	}

	channel.UpdatedAt = time.Now()
	cp := *channel
	f.byID[channel.ID] = &cp
	f.byCreator[channel.CreatorID] = &cp
	delete(f.byHandle, stored.Handle)
	f.byHandle[channel.Handle] = &cp

	return nil
}

func (f *fakeChannelStore) HandleExists(
	_ context.Context,
	handle string,
) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	_, ok := f.byHandle[handle]

	return ok, nil
}

type fixture struct {
	service  *CreatorService
	creators *fakeCreatorStore
	channels *fakeChannelStore
}

func newFixture() *fixture {
	creators := newFakeCreatorStore()
	channels := newFakeChannelStore()

	return &fixture{
		service:  NewCreatorService(creators, channels),
		creators: creators,
		channels: channels,
	}
}

// onboardThroughService runs OnboardCreator and mirrors the channel write
// the real transaction performs.
func onboardThroughService(
	t *testing.T,
	fx *fixture,
	userID uuid.UUID,
	handle string,
) *CreatorProfile {
	t.Helper()

	profile, err := fx.service.OnboardCreator(context.Background(), userID, OnboardInput{
		ChannelName: "Channel " + handle,
		Handle:      handle,
		Description: "Test channel",
	})
	if err != nil {
		t.Fatalf("onboard: %v", err)
	}

	fx.channels.mu.Lock()
	fx.channels.persist(profile.Channel)
	fx.channels.mu.Unlock()

	return profile
}

func strPtr(s string) *string { return &s }

func TestOnboardValid(t *testing.T) {
	fx := newFixture()
	userID := uuid.New()

	profile, err := fx.service.OnboardCreator(context.Background(), userID, OnboardInput{
		ChannelName: "Anshul Builds",
		Handle:      "AnshulBuilds",
		Description: "Building software.",
	})
	if err != nil {
		t.Fatalf("onboard: %v", err)
	}

	if profile.Creator.Status != model.CreatorStatusActive {
		t.Fatalf("expected active status, got %s", profile.Creator.Status)
	}

	if profile.Channel.Handle != "anshulbuilds" {
		t.Fatalf("expected normalized handle, got %q", profile.Channel.Handle)
	}

	if profile.Creator.UserID != userID {
		t.Fatal("identity must come from the caller")
	}
}

func TestOnboardDuplicateCreator(t *testing.T) {
	fx := newFixture()
	userID := uuid.New()

	onboardThroughService(t, fx, userID, "first_handle")

	_, err := fx.service.OnboardCreator(context.Background(), userID, OnboardInput{
		ChannelName: "Second",
		Handle:      "second_handle",
	})
	if !errors.Is(err, ErrCreatorAlreadyExists) {
		t.Fatalf("expected already-exists, got %v", err)
	}
}

func TestOnboardHandleTaken(t *testing.T) {
	fx := newFixture()

	onboardThroughService(t, fx, uuid.New(), "taken_handle")

	_, err := fx.service.OnboardCreator(context.Background(), uuid.New(), OnboardInput{
		ChannelName: "Other",
		Handle:      "taken_handle",
	})
	if !errors.Is(err, ErrHandleTaken) {
		t.Fatalf("expected taken, got %v", err)
	}
}

func TestOnboardInvalidHandles(t *testing.T) {
	fx := newFixture()

	for _, raw := range []string{"a", "ab", "@x", "a.b", "has space"} {
		_, err := fx.service.OnboardCreator(context.Background(), uuid.New(), OnboardInput{
			ChannelName: "Name",
			Handle:      raw,
		})
		if !errors.Is(err, ErrInvalidHandle) {
			t.Fatalf("expected invalid for %q, got %v", raw, err)
		}
	}

	_, err := fx.service.OnboardCreator(context.Background(), uuid.New(), OnboardInput{
		ChannelName: "Name",
		Handle:      "admin",
	})
	if !errors.Is(err, ErrReservedHandle) {
		t.Fatalf("expected reserved, got %v", err)
	}
}

func TestGetMyCreator(t *testing.T) {
	fx := newFixture()
	userID := uuid.New()

	if _, err := fx.service.GetMyCreator(
		context.Background(),
		userID,
	); !errors.Is(err, ErrCreatorNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}

	mine := onboardThroughService(t, fx, userID, "my_handle")

	found, err := fx.service.GetMyCreator(context.Background(), userID)
	if err != nil {
		t.Fatalf("get mine: %v", err)
	}

	if found.Creator.ID != mine.Creator.ID {
		t.Fatal("wrong creator returned")
	}
}

func TestGetCreatorByHandleVisibility(t *testing.T) {
	fx := newFixture()
	userID := uuid.New()

	mine := onboardThroughService(t, fx, userID, "visible_handle")

	found, err := fx.service.GetCreatorByHandle(context.Background(), "visible_handle")
	if err != nil {
		t.Fatalf("public lookup: %v", err)
	}

	if found.Creator.ID != mine.Creator.ID {
		t.Fatal("wrong creator returned")
	}

	if _, err := fx.service.GetCreatorByHandle(
		context.Background(),
		"ghost_handle",
	); !errors.Is(err, ErrCreatorNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}

	// Suspended creators disappear from public lookup.
	fx.creators.mu.Lock()
	stored := fx.creators.byUser[userID]
	stored.Status = model.CreatorStatusSuspended
	fx.creators.mu.Unlock()

	if _, err := fx.service.GetCreatorByHandle(
		context.Background(),
		"visible_handle",
	); !errors.Is(err, ErrCreatorNotFound) {
		t.Fatalf("expected suspended to read as not-found, got %v", err)
	}

	// But the owner still sees their own creator.
	own, err := fx.service.GetMyCreator(context.Background(), userID)
	if err != nil {
		t.Fatalf("owner lookup: %v", err)
	}

	if own.Creator.Status != model.CreatorStatusSuspended {
		t.Fatal("owner must see true status")
	}
}

func TestUpdateCreator(t *testing.T) {
	fx := newFixture()
	userID := uuid.New()

	onboardThroughService(t, fx, userID, "update_handle")

	updated, err := fx.service.UpdateCreator(context.Background(), userID, UpdateCreatorInput{
		DisplayName: strPtr("Anshul S"),
		Headline:    strPtr("Backend engineer"),
		ChannelName: strPtr("Anshul Builds v2"),
		AvatarURL:   strPtr("https://cdn.example.com/a.png"),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.Creator.DisplayName != "Anshul S" {
		t.Fatal("creator field not updated")
	}

	if updated.Channel.Name != "Anshul Builds v2" {
		t.Fatal("channel field not updated")
	}

	if updated.Channel.AvatarURL == nil ||
		*updated.Channel.AvatarURL != "https://cdn.example.com/a.png" {
		t.Fatal("channel avatar not updated")
	}

	if updated.Creator.AvatarURL == nil ||
		*updated.Creator.AvatarURL != "https://cdn.example.com/a.png" {
		t.Fatal("creator avatar must mirror channel avatar")
	}

	// Handle is immutable: no input field exists for it, and the stored
	// handle survives any update.
	if updated.Channel.Handle != "update_handle" {
		t.Fatal("handle must be immutable")
	}

	// Unknown creator cannot update.
	if _, err := fx.service.UpdateCreator(
		context.Background(),
		uuid.New(),
		UpdateCreatorInput{DisplayName: strPtr("X")},
	); !errors.Is(err, ErrCreatorNotFound) {
		t.Fatalf("expected not-found, got %v", err)
	}
}
