package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/user-service/internal/model"
	"github.com/Anshul563/edvance-project/services/user-service/internal/repository"
)

// fakeProfileStore is an in-memory ProfileStore.
type fakeProfileStore struct {
	mu        sync.Mutex
	byID      map[uuid.UUID]*model.UserProfile
	byUser    map[uuid.UUID]*model.UserProfile
	byName    map[string]*model.UserProfile
	updateErr error
}

func newFakeProfileStore() *fakeProfileStore {
	return &fakeProfileStore{
		byID:   make(map[uuid.UUID]*model.UserProfile),
		byUser: make(map[uuid.UUID]*model.UserProfile),
		byName: make(map[string]*model.UserProfile),
	}
}

func (f *fakeProfileStore) Create(
	_ context.Context,
	profile *model.UserProfile,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.byName[profile.Username]; exists {
		return repository.ErrUsernameTaken
	}

	if _, exists := f.byUser[profile.UserID]; exists {
		return repository.ErrProfileExists
	}

	profile.ID = uuid.New()
	profile.CreatedAt = time.Now()
	profile.UpdatedAt = time.Now()

	stored := *profile
	f.byID[profile.ID] = &stored
	f.byUser[profile.UserID] = &stored
	f.byName[profile.Username] = &stored

	return nil
}

func (f *fakeProfileStore) FindByUserID(
	_ context.Context,
	userID uuid.UUID,
) (*model.UserProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	profile, ok := f.byUser[userID]
	if !ok {
		return nil, repository.ErrProfileNotFound
	}

	cp := *profile

	return &cp, nil
}

func (f *fakeProfileStore) FindByUsername(
	_ context.Context,
	username string,
) (*model.UserProfile, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	profile, ok := f.byName[username]
	if !ok {
		return nil, repository.ErrProfileNotFound
	}

	cp := *profile

	return &cp, nil
}

func (f *fakeProfileStore) Update(
	_ context.Context,
	profile *model.UserProfile,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.updateErr != nil {
		return f.updateErr
	}

	stored, ok := f.byID[profile.ID]
	if !ok {
		return repository.ErrProfileNotFound
	}

	if other, taken := f.byName[profile.Username]; taken && other.ID != profile.ID {
		return repository.ErrUsernameTaken
	}

	delete(f.byName, stored.Username)

	profile.UpdatedAt = time.Now()
	cp := *profile
	f.byID[profile.ID] = &cp
	f.byUser[profile.UserID] = &cp
	f.byName[profile.Username] = &cp

	return nil
}

func (f *fakeProfileStore) UsernameExists(
	_ context.Context,
	username string,
) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	_, ok := f.byName[username]

	return ok, nil
}

func (f *fakeProfileStore) Delete(
	_ context.Context,
	id uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	profile, ok := f.byID[id]
	if !ok {
		return repository.ErrProfileNotFound
	}

	delete(f.byID, id)
	delete(f.byUser, profile.UserID)
	delete(f.byName, profile.Username)

	return nil
}

func newTestProfileService() (*ProfileService, *fakeProfileStore) {
	store := newFakeProfileStore()

	return NewProfileService(store), store
}

func strPtr(s string) *string { return &s }

func TestGetMyProfileProvisions(t *testing.T) {
	svc, _ := newTestProfileService()
	ctx := context.Background()
	userID := uuid.New()

	profile, err := svc.GetMyProfile(ctx, userID)
	if err != nil {
		t.Fatalf("get my profile: %v", err)
	}

	if profile.UserID != userID {
		t.Fatal("provisioned profile has wrong user")
	}

	if profile.Username == "" || profile.DisplayName == "" {
		t.Fatal("provisioned profile must have usable defaults")
	}

	// Second call returns the same row, no duplicate.
	again, err := svc.GetMyProfile(ctx, userID)
	if err != nil {
		t.Fatalf("second get: %v", err)
	}

	if again.ID != profile.ID {
		t.Fatal("expected the same profile row")
	}
}

func TestGetProfileByUsername(t *testing.T) {
	svc, _ := newTestProfileService()
	ctx := context.Background()
	userID := uuid.New()

	mine, err := svc.GetMyProfile(ctx, userID)
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	found, err := svc.GetProfileByUsername(ctx, mine.Username)
	if err != nil {
		t.Fatalf("find by username: %v", err)
	}

	if found.ID != mine.ID {
		t.Fatal("wrong profile returned")
	}

	if _, err := svc.GetProfileByUsername(ctx, "no-such-user"); !errors.Is(
		err,
		ErrProfileNotFound,
	) {
		t.Fatalf("expected not-found, got %v", err)
	}

	if _, err := svc.GetProfileByUsername(ctx, "bad name!"); !errors.Is(
		err,
		ErrUsernameInvalid,
	) {
		t.Fatalf("expected invalid, got %v", err)
	}
}

func TestUpdateMyProfilePartial(t *testing.T) {
	svc, _ := newTestProfileService()
	ctx := context.Background()
	userID := uuid.New()

	updated, err := svc.UpdateMyProfile(ctx, userID, UpdateProfileInput{
		DisplayName: strPtr("Anshul Shakya"),
		Bio:         strPtr("Full-stack developer."),
		CountryCode: strPtr("in"),
		Timezone:    strPtr("Asia/Kolkata"),
		WebsiteURL:  strPtr("https://example.com"),
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	if updated.DisplayName != "Anshul Shakya" {
		t.Fatalf("display name not updated: %q", updated.DisplayName)
	}

	if updated.Bio == nil || *updated.Bio != "Full-stack developer." {
		t.Fatal("bio not updated")
	}

	if updated.CountryCode == nil || *updated.CountryCode != "IN" {
		t.Fatalf("country code must normalize to uppercase, got %v", updated.CountryCode)
	}

	// Untouched fields stay at provisioned defaults.
	if updated.Username == "" {
		t.Fatal("username must be preserved")
	}

	// Clearing a nullable field with an empty string.
	cleared, err := svc.UpdateMyProfile(ctx, userID, UpdateProfileInput{
		Bio: strPtr(""),
	})
	if err != nil {
		t.Fatalf("clear bio: %v", err)
	}

	if cleared.Bio != nil {
		t.Fatal("expected bio to be cleared")
	}
}

func TestUpdateMyProfileValidation(t *testing.T) {
	svc, _ := newTestProfileService()
	ctx := context.Background()
	userID := uuid.New()

	cases := []struct {
		name  string
		input UpdateProfileInput
	}{
		{"empty display name", UpdateProfileInput{DisplayName: strPtr("")}},
		{"long display name", UpdateProfileInput{DisplayName: strPtr(strings.Repeat("x", 101))}},
		{"long bio", UpdateProfileInput{Bio: strPtr(string(make([]rune, 501)))}},
		{"bad website", UpdateProfileInput{WebsiteURL: strPtr("not-a-url")}},
		{"ftp website", UpdateProfileInput{WebsiteURL: strPtr("ftp://example.com")}},
		{"bad country", UpdateProfileInput{CountryCode: strPtr("IND")}},
		{"bad timezone", UpdateProfileInput{Timezone: strPtr("Mars/Olympus")}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := svc.UpdateMyProfile(ctx, userID, tc.input); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestUpdateUsername(t *testing.T) {
	svc, _ := newTestProfileService()
	ctx := context.Background()
	userID := uuid.New()
	otherID := uuid.New()

	if _, err := svc.GetMyProfile(ctx, userID); err != nil {
		t.Fatalf("provision: %v", err)
	}

	if _, err := svc.GetMyProfile(ctx, otherID); err != nil {
		t.Fatalf("provision other: %v", err)
	}

	updated, err := svc.UpdateUsername(ctx, userID, "Anshul_Dev")
	if err != nil {
		t.Fatalf("update username: %v", err)
	}

	if updated.Username != "anshul_dev" {
		t.Fatalf("expected normalized username, got %q", updated.Username)
	}

	// Taken by another user.
	if _, err := svc.UpdateUsername(ctx, otherID, "anshul_dev"); !errors.Is(
		err,
		ErrUsernameTaken,
	) {
		t.Fatalf("expected taken, got %v", err)
	}

	// Reserved.
	if _, err := svc.UpdateUsername(ctx, userID, "admin"); !errors.Is(
		err,
		ErrUsernameReserved,
	) {
		t.Fatalf("expected reserved, got %v", err)
	}

	// Invalid.
	if _, err := svc.UpdateUsername(ctx, userID, "a"); !errors.Is(
		err,
		ErrUsernameInvalid,
	) {
		t.Fatalf("expected invalid, got %v", err)
	}

	// Same username is a no-op success.
	if _, err := svc.UpdateUsername(ctx, userID, "anshul_dev"); err != nil {
		t.Fatalf("same username should succeed: %v", err)
	}
}

func TestCheckUsername(t *testing.T) {
	svc, _ := newTestProfileService()
	ctx := context.Background()

	free, err := svc.CheckUsername(ctx, "brand_new_name")
	if err != nil {
		t.Fatalf("check: %v", err)
	}

	if !free.Available || free.Username != "brand_new_name" {
		t.Fatalf("expected available, got %+v", free)
	}

	for username, reason := range map[string]string{
		"admin":     "reserved",
		"a":         "invalid",
		"bad name!": "invalid",
	} {
		out, err := svc.CheckUsername(ctx, username)
		if err != nil {
			t.Fatalf("check %q: %v", username, err)
		}

		if out.Available || out.Reason != reason {
			t.Fatalf("expected unavailable/%s, got %+v", reason, out)
		}
	}

	userID := uuid.New()

	mine, err := svc.GetMyProfile(ctx, userID)
	if err != nil {
		t.Fatalf("provision: %v", err)
	}

	taken, err := svc.CheckUsername(ctx, mine.Username)
	if err != nil {
		t.Fatalf("check taken: %v", err)
	}

	if taken.Available || taken.Reason != "taken" {
		t.Fatalf("expected taken, got %+v", taken)
	}
}

func TestCreateProfileConflict(t *testing.T) {
	svc, store := newTestProfileService()
	ctx := context.Background()
	userID := uuid.New()

	if _, err := svc.CreateProfile(ctx, userID, "taken_name", "Taken"); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := svc.CreateProfile(
		ctx,
		uuid.New(),
		"taken_name",
		"Other",
	); !errors.Is(err, ErrUsernameTaken) {
		t.Fatalf("expected taken, got %v", err)
	}

	// Duplicate user: the existing row is returned (provisioning race).
	existing, err := svc.CreateProfile(ctx, userID, "other_name", "Same")
	if err != nil {
		t.Fatalf("expected existing row, got %v", err)
	}

	if existing.Username != "taken_name" {
		t.Fatalf("expected original row, got %q", existing.Username)
	}

	_ = store
}
