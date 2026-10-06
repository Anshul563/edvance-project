package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/model"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/password"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/token"
)

// fakePasswordResetStore is an in-memory PasswordResetTokenStore. Like the
// real repository, CompleteReset updates the password and revokes the
// user's sessions in the same critical section as consuming the token.
type fakePasswordResetStore struct {
	mu       sync.Mutex
	byID     map[uuid.UUID]*model.PasswordResetToken
	byHash   map[string]*model.PasswordResetToken
	users    *fakeUserStore
	sessions *fakeSessionStore
}

func newFakePasswordResetStore() *fakePasswordResetStore {
	return &fakePasswordResetStore{
		byID:   make(map[uuid.UUID]*model.PasswordResetToken),
		byHash: make(map[string]*model.PasswordResetToken),
	}
}

func (f *fakePasswordResetStore) Create(
	_ context.Context,
	token *model.PasswordResetToken,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.byHash[token.TokenHash]; exists {
		return repository.ErrResetTokenExists
	}

	token.ID = uuid.New()
	token.CreatedAt = time.Now()

	stored := *token
	f.byID[token.ID] = &stored
	f.byHash[token.TokenHash] = &stored

	return nil
}

func (f *fakePasswordResetStore) FindByTokenHash(
	_ context.Context,
	hash string,
) (*model.PasswordResetToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	token, ok := f.byHash[hash]
	if !ok {
		return nil, repository.ErrResetTokenNotFound
	}

	cp := *token

	return &cp, nil
}

func (f *fakePasswordResetStore) MarkUsed(
	_ context.Context,
	tokenID uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	token, ok := f.byID[tokenID]
	if !ok || token.Used() {
		return repository.ErrResetTokenUsed
	}

	now := time.Now()
	token.UsedAt = &now

	return nil
}

func (f *fakePasswordResetStore) InvalidateForUser(
	_ context.Context,
	userID uuid.UUID,
) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var count int64

	for id, token := range f.byID {
		if token.UserID == userID {
			delete(f.byHash, token.TokenHash)
			delete(f.byID, id)
			count++
		}
	}

	return count, nil
}

func (f *fakePasswordResetStore) CompleteReset(
	ctx context.Context,
	tokenID uuid.UUID,
	userID uuid.UUID,
	passwordHash string,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	token, ok := f.byID[tokenID]
	if !ok || token.UserID != userID || token.Used() {
		return repository.ErrResetTokenUsed
	}

	now := time.Now()
	token.UsedAt = &now

	if f.users != nil {
		user, ok := f.users.byID[userID]
		if !ok {
			return repository.ErrUserNotFound
		}

		user.PasswordHash = passwordHash
	}

	if f.sessions != nil {
		_, _ = f.sessions.RevokeAllForUser(ctx, userID)
	}

	return nil
}

type resetFixture struct {
	service  *PasswordResetService
	auth     *AuthService
	users    *fakeUserStore
	tokens   *fakePasswordResetStore
	sessions *fakeSessionStore
	sender   *stubSender
}

func newResetFixture() *resetFixture {
	users := newFakeUserStore()
	sessions := newFakeSessionStore()
	tokens := newFakePasswordResetStore()
	tokens.users = users
	tokens.sessions = sessions
	sender := &stubSender{}

	svc, err := NewPasswordResetService(
		tokens,
		users,
		sender,
		&stubLimiter{allow: true},
		&stubLimiter{allow: true},
		PasswordResetConfig{
			TokenTTL:     30 * time.Minute,
			ResetBaseURL: "http://localhost:3000/reset-password",
		},
	)
	if err != nil {
		panic(err)
	}

	auth := NewAuthService(
		users,
		newTestSessionService(sessions),
		15*time.Minute,
		nil,
	)

	return &resetFixture{
		service:  svc,
		auth:     auth,
		users:    users,
		tokens:   tokens,
		sessions: sessions,
		sender:   sender,
	}
}

func requestResetToken(
	t *testing.T,
	fx *resetFixture,
	email string,
) string {
	t.Helper()

	if err := fx.service.RequestPasswordReset(
		context.Background(),
		email,
		"127.0.0.1",
	); err != nil {
		t.Fatalf("request reset: %v", err)
	}

	mail, ok := fx.sender.last()
	if !ok {
		t.Fatal("expected a reset email")
	}

	if mail.kind != "reset" {
		t.Fatalf("expected reset email, got %q", mail.kind)
	}

	return resetTokenFromURL(t, mail.url)
}

func TestForgotPasswordExistingEmail(t *testing.T) {
	fx := newResetFixture()
	ctx := context.Background()
	addFakeUser(t, fx.users, "forgot@example.com", model.UserStatusActive)

	if err := fx.service.RequestPasswordReset(
		ctx,
		"forgot@example.com",
		"127.0.0.1",
	); err != nil {
		t.Fatalf("request: %v", err)
	}

	if len(fx.sender.sent) != 1 {
		t.Fatalf("expected one email, got %d", len(fx.sender.sent))
	}

	// Only the hash may be stored.
	mail, _ := fx.sender.last()
	raw := resetTokenFromURL(t, mail.url)

	stored, err := fx.tokens.FindByTokenHash(
		ctx,
		token.HashPasswordResetToken(raw),
	)
	if err != nil {
		t.Fatalf("find stored token: %v", err)
	}

	if stored.TokenHash == raw {
		t.Fatal("raw token must not be stored")
	}
}

func TestForgotPasswordUnknownEmail(t *testing.T) {
	fx := newResetFixture()

	// Unknown addresses succeed silently: no enumeration.
	if err := fx.service.RequestPasswordReset(
		context.Background(),
		"nobody@example.com",
		"127.0.0.1",
	); err != nil {
		t.Fatalf("unknown email must succeed silently, got %v", err)
	}

	if len(fx.sender.sent) != 0 {
		t.Fatal("no email must be sent for unknown addresses")
	}
}

func TestForgotPasswordRateLimited(t *testing.T) {
	fx := newResetFixture()

	limited, err := NewPasswordResetService(
		fx.tokens,
		fx.users,
		fx.sender,
		&stubLimiter{allow: false},
		&stubLimiter{allow: true},
		PasswordResetConfig{
			TokenTTL:     30 * time.Minute,
			ResetBaseURL: "http://localhost:3000/reset-password",
		},
	)
	if err != nil {
		t.Fatalf("service: %v", err)
	}

	err = limited.RequestPasswordReset(
		context.Background(),
		"any@example.com",
		"127.0.0.1",
	)
	if !errors.Is(err, ErrResetTooSoon) {
		t.Fatalf("expected too-soon error, got %v", err)
	}
}

func TestForgotPasswordIPLimited(t *testing.T) {
	fx := newResetFixture()

	limited, err := NewPasswordResetService(
		fx.tokens,
		fx.users,
		fx.sender,
		&stubLimiter{allow: true},
		&stubLimiter{allow: false},
		PasswordResetConfig{
			TokenTTL:     30 * time.Minute,
			ResetBaseURL: "http://localhost:3000/reset-password",
		},
	)
	if err != nil {
		t.Fatalf("service: %v", err)
	}

	err = limited.RequestPasswordReset(
		context.Background(),
		"any@example.com",
		"127.0.0.1",
	)
	if !errors.Is(err, ErrResetTooSoon) {
		t.Fatalf("expected too-soon error, got %v", err)
	}
}

func TestForgotPasswordInvalidatesPrevious(t *testing.T) {
	fx := newResetFixture()
	ctx := context.Background()
	addFakeUser(t, fx.users, "replace@example.com", model.UserStatusActive)

	first := requestResetToken(t, fx, "replace@example.com")
	second := requestResetToken(t, fx, "replace@example.com")

	if first == second {
		t.Fatal("expected a fresh token per request")
	}

	err := fx.service.ResetPassword(ctx, first, "NewPassword123")
	if !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("expected old token to be rejected, got %v", err)
	}

	if err := fx.service.ResetPassword(ctx, second, "NewPassword123"); err != nil {
		t.Fatalf("new token must work: %v", err)
	}
}

func TestResetPasswordSuccess(t *testing.T) {
	fx := newResetFixture()
	ctx := context.Background()
	addFakeUser(t, fx.users, "success@example.com", model.UserStatusActive)

	loggedIn, err := fx.auth.Login(ctx, LoginInput{
		Email:    "success@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	raw := requestResetToken(t, fx, "success@example.com")

	if err := fx.service.ResetPassword(ctx, raw, "NewPassword123"); err != nil {
		t.Fatalf("reset: %v", err)
	}

	// The pre-reset session is revoked: its refresh token is dead.
	_, err = fx.auth.sessions.RefreshSession(ctx, RefreshSessionInput{
		RefreshToken: loggedIn.RefreshToken,
	})
	if !errors.Is(err, ErrRefreshTokenReused) {
		t.Fatalf("expected old refresh token to be revoked, got %v", err)
	}

	// Old password fails, new password works.
	if _, err := fx.auth.Login(ctx, LoginInput{
		Email:    "success@example.com",
		Password: "Password123",
	}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected old password to fail, got %v", err)
	}

	out, err := fx.auth.Login(ctx, LoginInput{
		Email:    "success@example.com",
		Password: "NewPassword123",
	})
	if err != nil {
		t.Fatalf("login with new password: %v", err)
	}

	if out.AccessToken == "" {
		t.Fatal("expected fresh tokens after login")
	}

	// Stored hash matches the new password and nothing plain leaked.
	stored, err := fx.users.FindByEmail(ctx, "success@example.com")
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}

	if err := password.CheckPassword(stored.PasswordHash, "NewPassword123"); err != nil {
		t.Fatal("stored hash must verify the new password")
	}
}

func TestResetPasswordFailures(t *testing.T) {
	fx := newResetFixture()
	ctx := context.Background()
	addFakeUser(t, fx.users, "failures@example.com", model.UserStatusActive)

	raw := requestResetToken(t, fx, "failures@example.com")

	if err := fx.service.ResetPassword(ctx, raw, "NewPassword123"); err != nil {
		t.Fatalf("first reset: %v", err)
	}

	cases := []struct {
		name        string
		token       string
		newPassword string
		want        error
	}{
		{"reuse fails", raw, "AnotherPass1", ErrResetTokenUsed},
		{"unknown token", "does-not-exist", "AnotherPass1", ErrInvalidResetToken},
		{"empty token", "", "AnotherPass1", ErrInvalidResetToken},
		{"weak password", "does-not-exist", "short", ErrInvalidPassword},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := fx.service.ResetPassword(ctx, tc.token, tc.newPassword)
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestResetPasswordExpired(t *testing.T) {
	fx := newResetFixture()
	ctx := context.Background()
	user := addFakeUser(t, fx.users, "expired-reset@example.com", model.UserStatusActive)

	expired := &model.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: token.HashPasswordResetToken("expired-token"),
		ExpiresAt: time.Now().Add(-time.Hour),
	}

	if err := fx.tokens.Create(ctx, expired); err != nil {
		t.Fatalf("seed expired token: %v", err)
	}

	err := fx.service.ResetPassword(ctx, "expired-token", "NewPassword123")
	if !errors.Is(err, ErrResetTokenExpired) {
		t.Fatalf("expected expired error, got %v", err)
	}
}

func TestResetPasswordUserStatus(t *testing.T) {
	fx := newResetFixture()
	ctx := context.Background()

	suspended := addFakeUser(
		t,
		fx.users,
		"suspended-reset@example.com",
		model.UserStatusSuspended,
	)
	deleted := addFakeUser(
		t,
		fx.users,
		"deleted-reset@example.com",
		model.UserStatusDeleted,
	)

	seedToken := func(userID uuid.UUID, hash string) {
		t.Helper()

		record := &model.PasswordResetToken{
			UserID:    userID,
			TokenHash: hash,
			ExpiresAt: time.Now().Add(time.Hour),
		}

		if err := fx.tokens.Create(ctx, record); err != nil {
			t.Fatalf("seed token: %v", err)
		}
	}

	seedToken(suspended.ID, token.HashPasswordResetToken("suspended-token"))
	seedToken(deleted.ID, token.HashPasswordResetToken("deleted-token"))

	err := fx.service.ResetPassword(ctx, "suspended-token", "NewPassword123")
	if !errors.Is(err, ErrUserSuspended) {
		t.Fatalf("expected suspended error, got %v", err)
	}

	// Deleted accounts stay deleted: generic error, password untouched.
	err = fx.service.ResetPassword(ctx, "deleted-token", "NewPassword123")
	if !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("expected invalid token error, got %v", err)
	}

	reloaded, err := fx.users.FindByID(ctx, deleted.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}

	if reloaded.Status != model.UserStatusDeleted {
		t.Fatal("deleted account must stay deleted")
	}

	if err := password.CheckPassword(
		reloaded.PasswordHash,
		"Password123",
	); err != nil {
		t.Fatal("deleted account password must be unchanged")
	}
}

func resetTokenFromURL(t *testing.T, rawURL string) string {
	t.Helper()

	const marker = "token="

	for i := 0; i+len(marker) <= len(rawURL); i++ {
		if rawURL[i:i+len(marker)] == marker {
			return rawURL[i+len(marker):]
		}
	}

	t.Fatalf("no token in URL %q", rawURL)

	return ""
}
