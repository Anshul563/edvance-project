//go:build integration

package service

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/repository"
)

// Full authentication lifecycle against a real PostgreSQL:
//
//	Register -> Login -> Refresh -> old token rejected ->
//	Logout -> revoked -> LogoutAll
//
// Run with:
//
//	DATABASE_URL=postgres://... go test -tags integration ./internal/service/
func TestAuthLifecycleIntegration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := repository.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer pool.Close()

	userRepo := repository.NewUserRepository(pool)
	sessionStore := repository.NewSessionRepository(pool)

	sessionService, err := NewSessionService(sessionStore, SessionConfig{
		AccessSecret: "integration-test-secret-0123456789abcdef",
		Issuer:       "edvance-auth",
		Audience:     "edvance-api",
		AccessTTL:    15 * time.Minute,
		RefreshTTL:   720 * time.Hour,
	})
	if err != nil {
		t.Fatalf("session service: %v", err)
	}

	svc := NewAuthService(userRepo, sessionService, 15*time.Minute, nil)

	email := "lifecycle-" + time.Now().Format("150405.000000") + "@example.com"

	// Register.
	registered, err := svc.Register(ctx, RegisterInput{
		Email:       email,
		Username:    "lifecycle_" + time.Now().Format("150405"),
		Password:    "Password123",
		DisplayName: "Lifecycle",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM users WHERE id = $1`,
			registered.User.ID,
		)
	}()

	// Login.
	loggedIn, err := svc.Login(ctx, LoginInput{
		Email:     email,
		Password:  "Password123",
		UserAgent: "integration-test",
		IPAddress: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if loggedIn.AccessToken == "" || loggedIn.RefreshToken == "" {
		t.Fatal("expected token pair from login")
	}

	// List: exactly one session.
	sessions, err := svc.ListSessions(ctx, registered.User.ID)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}

	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	// Refresh.
	refreshed, err := svc.Refresh(ctx, RefreshInput{
		RefreshToken: loggedIn.RefreshToken,
	})
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if refreshed.RefreshToken == loggedIn.RefreshToken {
		t.Fatal("expected rotated refresh token")
	}

	// Old refresh token must be rejected as reuse.
	_, err = svc.Refresh(ctx, RefreshInput{
		RefreshToken: loggedIn.RefreshToken,
	})
	if !errors.Is(err, ErrRefreshTokenReused) {
		t.Fatalf("expected reuse detection, got %v", err)
	}

	// The reuse above revoked the whole family: the rotated token is dead.
	_, err = svc.Refresh(ctx, RefreshInput{
		RefreshToken: refreshed.RefreshToken,
	})
	if !errors.Is(err, ErrRefreshTokenReused) {
		t.Fatalf("expected family revocation, got %v", err)
	}

	// Fresh login for logout checks.
	again, err := svc.Login(ctx, LoginInput{
		Email:    email,
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("second login: %v", err)
	}

	// Logout single session.
	if err := svc.Logout(ctx, LogoutInput{
		UserID:       registered.User.ID,
		RefreshToken: again.RefreshToken,
	}); err != nil {
		t.Fatalf("logout: %v", err)
	}

	_, err = svc.Refresh(ctx, RefreshInput{
		RefreshToken: again.RefreshToken,
	})
	if !errors.Is(err, ErrRefreshTokenReused) {
		t.Fatalf("expected logged-out token to be rejected, got %v", err)
	}

	// Logout-all revokes the rest.
	third, err := svc.Login(ctx, LoginInput{
		Email:    email,
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("third login: %v", err)
	}

	fourth, err := svc.Login(ctx, LoginInput{
		Email:    email,
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("fourth login: %v", err)
	}

	count, err := svc.LogoutAll(ctx, registered.User.ID)
	if err != nil {
		t.Fatalf("logout all: %v", err)
	}

	if count != 2 {
		t.Fatalf("expected 2 revoked sessions, got %d", count)
	}

	for _, tok := range []string{third.RefreshToken, fourth.RefreshToken} {
		_, err := svc.Refresh(ctx, RefreshInput{RefreshToken: tok})
		if !errors.Is(err, ErrRefreshTokenReused) {
			t.Fatalf("expected revoked token to be rejected, got %v", err)
		}
	}
}
