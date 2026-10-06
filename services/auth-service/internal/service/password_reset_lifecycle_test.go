//go:build integration

package service

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/repository"
)

// Complete password-recovery lifecycle against real PostgreSQL and Redis:
//
//	Register -> Login -> forgot -> reset URL -> reset ->
//	old session dead -> old password fails -> new password works.
//	Also covers token reuse and expiry.
func TestPasswordResetLifecycleIntegration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pool, err := repository.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}
	defer pool.Close()

	redisClient, err := repository.NewRedis(ctx, "redis://localhost:6379/0")
	if err != nil {
		t.Skipf("redis not available: %v", err)
	}
	defer redisClient.Close()

	userRepo := repository.NewUserRepository(pool)
	sender := &stubSender{}

	sessionService, err := NewSessionService(
		repository.NewSessionRepository(pool),
		SessionConfig{
			AccessSecret: "integration-test-secret-0123456789abcdef",
			Issuer:       "edvance-auth",
			Audience:     "edvance-api",
			AccessTTL:    15 * time.Minute,
			RefreshTTL:   720 * time.Hour,
		},
	)
	if err != nil {
		t.Fatalf("session service: %v", err)
	}

	resetService, err := NewPasswordResetService(
		repository.NewPasswordResetRepository(pool),
		userRepo,
		sender,
		repository.NewPasswordResetLimiter(redisClient, time.Minute),
		repository.NewPasswordResetIPLimiter(redisClient, time.Minute),
		PasswordResetConfig{
			TokenTTL:     30 * time.Minute,
			ResetBaseURL: "http://localhost:3000/reset-password",
		},
	)
	if err != nil {
		t.Fatalf("reset service: %v", err)
	}

	authService := NewAuthService(userRepo, sessionService, 15*time.Minute, nil)

	stamp := time.Now().Format("150405.000000")
	email := "reset-life-" + stamp + "@example.com"

	registered, err := authService.Register(ctx, RegisterInput{
		Email:       email,
		Username:    "reset_life_" + time.Now().Format("150405"),
		Password:    "Password123",
		DisplayName: "Reset Life",
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

	// Active session before the reset.
	loggedIn, err := authService.Login(ctx, LoginInput{
		Email:    email,
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	// Forgot always succeeds silently.
	if err := resetService.RequestPasswordReset(
		ctx,
		email,
		"127.0.0.1",
	); err != nil {
		t.Fatalf("forgot: %v", err)
	}

	mail, ok := lastResetEmail(sender)
	if !ok {
		t.Fatal("expected a reset email")
	}

	rawToken := resetTokenFromIntegrationURL(t, mail.url)

	// Reset the password.
	if err := resetService.ResetPassword(
		ctx,
		rawToken,
		"NewPassword123",
	); err != nil {
		t.Fatalf("reset: %v", err)
	}

	// Old session is dead: its refresh token no longer rotates.
	_, err = sessionService.RefreshSession(ctx, RefreshSessionInput{
		RefreshToken: loggedIn.RefreshToken,
	})
	if !errors.Is(err, ErrRefreshTokenReused) {
		t.Fatalf("expected old refresh token to be revoked, got %v", err)
	}

	// Old password fails.
	if _, err := authService.Login(ctx, LoginInput{
		Email:    email,
		Password: "Password123",
	}); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected old password to fail, got %v", err)
	}

	// New password works.
	if _, err := authService.Login(ctx, LoginInput{
		Email:    email,
		Password: "NewPassword123",
	}); err != nil {
		t.Fatalf("login with new password: %v", err)
	}

	// Token reuse fails.
	if err := resetService.ResetPassword(
		ctx,
		rawToken,
		"AnotherPass1",
	); !errors.Is(err, ErrResetTokenUsed) {
		t.Fatalf("expected reuse error, got %v", err)
	}

	// Expired token fails. Flush-free: unique email keeps the limiter out
	// of the way.
	expiredEmail := "reset-expired-" + stamp + "@example.com"

	if _, err := authService.Register(ctx, RegisterInput{
		Email:       expiredEmail,
		Username:    "reset_exp_" + time.Now().Format("150405"),
		Password:    "Password123",
		DisplayName: "Reset Expired",
	}); err != nil {
		t.Fatalf("register second: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM users WHERE email = $1`,
			expiredEmail,
		)
	}()

	shortService, err := NewPasswordResetService(
		repository.NewPasswordResetRepository(pool),
		userRepo,
		sender,
		repository.NewPasswordResetLimiter(redisClient, time.Nanosecond),
		repository.NewPasswordResetIPLimiter(redisClient, time.Nanosecond),
		PasswordResetConfig{
			// TTL so short the token is born expired.
			TokenTTL:     time.Nanosecond,
			ResetBaseURL: "http://localhost:3000/reset-password",
		},
	)
	if err != nil {
		t.Fatalf("short reset service: %v", err)
	}

	if err := shortService.RequestPasswordReset(
		ctx,
		expiredEmail,
		// Distinct IP: the earlier request already consumed the
		// 127.0.0.1 cooldown window, which is exactly what the
		// IP limiter is for.
		"127.0.0.2",
	); err != nil {
		t.Fatalf("forgot for expiry test: %v", err)
	}

	expiredToken := resetTokenFromIntegrationURL(t, sender.sent[len(sender.sent)-1].url)

	// Give the nanosecond TTL no chance to survive.
	time.Sleep(10 * time.Millisecond)

	if err := resetService.ResetPassword(
		ctx,
		expiredToken,
		"NewPassword123",
	); !errors.Is(err, ErrResetTokenExpired) {
		t.Fatalf("expected expired error, got %v", err)
	}
}

func lastResetEmail(sender *stubSender) (sentEmail, bool) {
	sender.mu.Lock()
	defer sender.mu.Unlock()

	for i := len(sender.sent) - 1; i >= 0; i-- {
		if sender.sent[i].kind == "reset" {
			return sender.sent[i], true
		}
	}

	return sentEmail{}, false
}

func resetTokenFromIntegrationURL(t *testing.T, rawURL string) string {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse reset URL: %v", err)
	}

	token := parsed.Query().Get("token")
	if token == "" {
		t.Fatalf("no token in URL %q", rawURL)
	}

	return token
}
