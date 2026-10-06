package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/model"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/token"
)

// fakeUserStore is an in-memory UserStore for AuthService unit tests.
type fakeUserStore struct {
	mu         sync.Mutex
	byID       map[uuid.UUID]*model.User
	byEmail    map[string]*model.User
	byUsername map[string]*model.User
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{
		byID:       make(map[uuid.UUID]*model.User),
		byEmail:    make(map[string]*model.User),
		byUsername: make(map[string]*model.User),
	}
}

func (f *fakeUserStore) Create(
	_ context.Context,
	user *model.User,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	user.ID = uuid.New()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	stored := *user
	f.byID[user.ID] = &stored
	f.byEmail[user.Email] = &stored
	f.byUsername[user.Username] = &stored

	return nil
}

func (f *fakeUserStore) FindByID(
	_ context.Context,
	id uuid.UUID,
) (*model.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	user, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrUserNotFound
	}

	cp := *user

	return &cp, nil
}

func (f *fakeUserStore) FindByEmail(
	_ context.Context,
	email string,
) (*model.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	user, ok := f.byEmail[email]
	if !ok {
		return nil, repository.ErrUserNotFound
	}

	cp := *user

	return &cp, nil
}

func (f *fakeUserStore) FindByUsername(
	_ context.Context,
	username string,
) (*model.User, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	user, ok := f.byUsername[username]
	if !ok {
		return nil, repository.ErrUserNotFound
	}

	cp := *user

	return &cp, nil
}

func addFakeUser(
	t *testing.T,
	store *fakeUserStore,
	email string,
	status model.UserStatus,
) *model.User {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword(
		[]byte("Password123"),
		bcrypt.MinCost,
	)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	user := &model.User{
		Email:         email,
		Username:      "user_" + uuid.NewString()[:8],
		PasswordHash:  string(hash),
		DisplayName:   "Test User",
		Status:        status,
		EmailVerified: false,
	}

	if err := store.Create(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}

	return user
}

func newTestAuthService(
	users *fakeUserStore,
	store *fakeSessionStore,
) *AuthService {
	return NewAuthService(
		users,
		newTestSessionService(store),
		15*time.Minute,
		nil,
	)
}

func TestLoginValid(t *testing.T) {
	users := newFakeUserStore()
	store := newFakeSessionStore()
	svc := newTestAuthService(users, store)
	user := addFakeUser(t, users, "login@example.com", model.UserStatusActive)

	out, err := svc.Login(context.Background(), LoginInput{
		Email:     "login@example.com",
		Password:  "Password123",
		UserAgent: "test-agent",
		IPAddress: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if out.AccessToken == "" || out.RefreshToken == "" {
		t.Fatal("expected both tokens")
	}

	if out.TokenType != "Bearer" || out.ExpiresIn != 900 {
		t.Fatalf("unexpected token metadata: %+v", out)
	}

	if out.User.ID != user.ID || out.User.Email != user.Email {
		t.Fatal("user mismatch")
	}

	if out.SessionID == uuid.Nil {
		t.Fatal("expected session id")
	}

	claims, err := token.ValidateAccessToken(
		out.AccessToken,
		"test-access-secret-0123456789abcdef",
		"edvance-auth",
		"edvance-api",
	)
	if err != nil {
		t.Fatalf("access token does not validate: %v", err)
	}

	if sid, ok := claims.GetSessionID(); !ok || sid != out.SessionID {
		t.Fatal("access token sid must match the session")
	}
}

func TestLoginFailures(t *testing.T) {
	users := newFakeUserStore()
	svc := newTestAuthService(users, newFakeSessionStore())
	addFakeUser(t, users, "victim@example.com", model.UserStatusActive)
	addFakeUser(t, users, "suspended@example.com", model.UserStatusSuspended)
	addFakeUser(t, users, "deleted@example.com", model.UserStatusDeleted)

	cases := []struct {
		name     string
		email    string
		password string
		want     error
	}{
		{"wrong password", "victim@example.com", "WrongPass1", ErrInvalidCredentials},
		{"unknown email", "nobody@example.com", "Password123", ErrInvalidCredentials},
		{"empty password", "victim@example.com", "", ErrInvalidCredentials},
		{"empty email", "", "Password123", ErrInvalidCredentials},
		{"suspended user", "suspended@example.com", "Password123", ErrUserSuspended},
		{"deleted user", "deleted@example.com", "Password123", ErrUserDeleted},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Login(context.Background(), LoginInput{
				Email:    tc.email,
				Password: tc.password,
			})
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestLoginEmailNormalized(t *testing.T) {
	users := newFakeUserStore()
	svc := newTestAuthService(users, newFakeSessionStore())
	addFakeUser(t, users, "case@example.com", model.UserStatusActive)

	_, err := svc.Login(context.Background(), LoginInput{
		Email:    "  CASE@EXAMPLE.COM  ",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("expected normalized email to log in: %v", err)
	}
}

func TestRefreshFlow(t *testing.T) {
	users := newFakeUserStore()
	store := newFakeSessionStore()
	svc := newTestAuthService(users, store)
	ctx := context.Background()

	addFakeUser(t, users, "refresh@example.com", model.UserStatusActive)

	first, err := svc.Login(ctx, LoginInput{
		Email:    "refresh@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	second, err := svc.Refresh(ctx, RefreshInput{
		RefreshToken: first.RefreshToken,
	})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if second.RefreshToken == first.RefreshToken {
		t.Fatal("rotation must issue a new refresh token")
	}

	if second.SessionID == first.SessionID {
		t.Fatal("rotation must create a new session")
	}

	// The old token is now revoked: replaying it is reuse.
	_, err = svc.Refresh(ctx, RefreshInput{
		RefreshToken: first.RefreshToken,
	})
	if !errors.Is(err, ErrRefreshTokenReused) {
		t.Fatalf("expected reuse detection, got %v", err)
	}
}

func TestRefreshInvalidToken(t *testing.T) {
	users := newFakeUserStore()
	svc := newTestAuthService(users, newFakeSessionStore())

	_, err := svc.Refresh(context.Background(), RefreshInput{
		RefreshToken: "nope",
	})
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected invalid refresh token, got %v", err)
	}
}

func TestRefreshExpiredSession(t *testing.T) {
	users := newFakeUserStore()
	store := newFakeSessionStore()
	svc := newTestAuthService(users, store)
	ctx := context.Background()

	user := addFakeUser(t, users, "expired@example.com", model.UserStatusActive)

	expired := &model.AuthSession{
		UserID:           user.ID,
		RefreshTokenHash: token.HashRefreshToken("expired-token"),
		ExpiresAt:        time.Now().Add(-time.Hour),
	}

	if err := store.Create(ctx, expired); err != nil {
		t.Fatalf("seed expired session: %v", err)
	}

	_, err := svc.Refresh(ctx, RefreshInput{RefreshToken: "expired-token"})
	if !errors.Is(err, ErrRefreshTokenExpired) {
		t.Fatalf("expected expired refresh token, got %v", err)
	}
}

func TestRefreshReuseRevokesFamily(t *testing.T) {
	users := newFakeUserStore()
	store := newFakeSessionStore()
	svc := newTestAuthService(users, store)
	ctx := context.Background()

	addFakeUser(t, users, "family@example.com", model.UserStatusActive)

	first, err := svc.Login(ctx, LoginInput{
		Email:    "family@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	secondLogin, err := svc.Login(ctx, LoginInput{
		Email:    "family@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("second login: %v", err)
	}

	// Rotate the first session so its token becomes revoked.
	if _, err := svc.Refresh(ctx, RefreshInput{
		RefreshToken: first.RefreshToken,
	}); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	// Replay the revoked token: reuse detected, family revoked.
	_, err = svc.Refresh(ctx, RefreshInput{
		RefreshToken: first.RefreshToken,
	})
	if !errors.Is(err, ErrRefreshTokenReused) {
		t.Fatalf("expected reuse detection, got %v", err)
	}

	// The other session of the same user must now be revoked too.
	_, err = svc.Refresh(ctx, RefreshInput{
		RefreshToken: secondLogin.RefreshToken,
	})
	if !errors.Is(err, ErrRefreshTokenReused) {
		t.Fatalf("expected family revocation, got %v", err)
	}
}

func TestRefreshSuspendedUser(t *testing.T) {
	users := newFakeUserStore()
	store := newFakeSessionStore()
	svc := newTestAuthService(users, store)
	ctx := context.Background()

	user := addFakeUser(t, users, "doomed@example.com", model.UserStatusActive)

	loggedIn, err := svc.Login(ctx, LoginInput{
		Email:    "doomed@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	// Suspend the account after the session was created.
	users.mu.Lock()
	users.byID[user.ID].Status = model.UserStatusSuspended
	users.mu.Unlock()

	_, err = svc.Refresh(ctx, RefreshInput{
		RefreshToken: loggedIn.RefreshToken,
	})
	if !errors.Is(err, ErrUserSuspended) {
		t.Fatalf("expected suspended error, got %v", err)
	}
}

func TestLogoutByRefreshToken(t *testing.T) {
	users := newFakeUserStore()
	store := newFakeSessionStore()
	svc := newTestAuthService(users, store)
	ctx := context.Background()

	user := addFakeUser(t, users, "logout@example.com", model.UserStatusActive)

	loggedIn, err := svc.Login(ctx, LoginInput{
		Email:    "logout@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if err := svc.Logout(ctx, LogoutInput{
		UserID:       user.ID,
		RefreshToken: loggedIn.RefreshToken,
	}); err != nil {
		t.Fatalf("logout: %v", err)
	}

	// Idempotent: logging out again succeeds.
	if err := svc.Logout(ctx, LogoutInput{
		UserID:       user.ID,
		RefreshToken: loggedIn.RefreshToken,
	}); err != nil {
		t.Fatalf("second logout should succeed: %v", err)
	}

	// Unknown tokens also succeed (already logged out).
	if err := svc.Logout(ctx, LogoutInput{
		UserID:       user.ID,
		RefreshToken: "unknown",
	}); err != nil {
		t.Fatalf("unknown token logout should succeed: %v", err)
	}
}

func TestLogoutBySessionID(t *testing.T) {
	users := newFakeUserStore()
	svc := newTestAuthService(users, newFakeSessionStore())
	ctx := context.Background()

	user := addFakeUser(t, users, "logout-sid@example.com", model.UserStatusActive)

	loggedIn, err := svc.Login(ctx, LoginInput{
		Email:    "logout-sid@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if err := svc.Logout(ctx, LogoutInput{
		UserID:    user.ID,
		SessionID: loggedIn.SessionID,
	}); err != nil {
		t.Fatalf("logout by session: %v", err)
	}
}

func TestLogoutForeignToken(t *testing.T) {
	users := newFakeUserStore()
	svc := newTestAuthService(users, newFakeSessionStore())
	ctx := context.Background()

	addFakeUser(t, users, "owner@example.com", model.UserStatusActive)
	other := addFakeUser(t, users, "other@example.com", model.UserStatusActive)

	loggedIn, err := svc.Login(ctx, LoginInput{
		Email:    "owner@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	err = svc.Logout(ctx, LogoutInput{
		UserID:       other.ID,
		RefreshToken: loggedIn.RefreshToken,
	})
	if err != nil {
		t.Fatalf("logout with foreign token must be a harmless no-op, got %v", err)
	}

	// The owner's session must be untouched: it still refreshes.
	if _, err := svc.Refresh(ctx, RefreshInput{
		RefreshToken: loggedIn.RefreshToken,
	}); err != nil {
		t.Fatalf("owner session should still work: %v", err)
	}
}

func TestLogoutAllAndList(t *testing.T) {
	users := newFakeUserStore()
	svc := newTestAuthService(users, newFakeSessionStore())
	ctx := context.Background()

	user := addFakeUser(t, users, "multi@example.com", model.UserStatusActive)
	addFakeUser(t, users, "bystander@example.com", model.UserStatusActive)

	for i := 0; i < 2; i++ {
		if _, err := svc.Login(ctx, LoginInput{
			Email:    "multi@example.com",
			Password: "Password123",
		}); err != nil {
			t.Fatalf("login: %v", err)
		}
	}

	if _, err := svc.Login(ctx, LoginInput{
		Email:    "bystander@example.com",
		Password: "Password123",
	}); err != nil {
		t.Fatalf("login: %v", err)
	}

	sessions, err := svc.ListSessions(ctx, user.ID)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	count, err := svc.LogoutAll(ctx, user.ID)
	if err != nil {
		t.Fatalf("logout all: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected 2 revoked, got %d", count)
	}
}

func TestRevokeForeignSession(t *testing.T) {
	users := newFakeUserStore()
	svc := newTestAuthService(users, newFakeSessionStore())
	ctx := context.Background()

	addFakeUser(t, users, "a@example.com", model.UserStatusActive)
	b := addFakeUser(t, users, "b@example.com", model.UserStatusActive)

	aLogin, err := svc.Login(ctx, LoginInput{
		Email:    "a@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if err := svc.RevokeSession(ctx, aLogin.SessionID, b.ID); !errors.Is(
		err,
		ErrSessionNotFound,
	) {
		t.Fatalf("expected not-found for foreign session, got %v", err)
	}
}
