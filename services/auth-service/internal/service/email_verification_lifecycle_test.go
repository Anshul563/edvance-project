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

// Complete email-verification lifecycle against real PostgreSQL and Redis:
//
//	Register -> token emailed -> verify -> verified ->
//	reuse rejected -> resend invalidates old, new verifies ->
//	immediate second resend is rate limited
func TestEmailVerificationLifecycleIntegration(t *testing.T) {
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
	tokenRepo := repository.NewEmailVerificationRepository(pool)
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

	verification, err := NewEmailVerificationService(
		tokenRepo,
		userRepo,
		sender,
		repository.NewResendLimiter(redisClient, time.Minute),
		EmailVerificationConfig{
			TokenTTL:            24 * time.Hour,
			VerificationBaseURL: "http://localhost:3000/verify-email",
		},
	)
	if err != nil {
		t.Fatalf("verification service: %v", err)
	}

	svc := NewAuthService(userRepo, sessionService, 15*time.Minute, verification)

	email := "verify-life-" + time.Now().Format("150405.000000") + "@example.com"

	registered, err := svc.Register(ctx, RegisterInput{
		Email:       email,
		Username:    "verify_life_" + time.Now().Format("150405"),
		Password:    "Password123",
		DisplayName: "Verify Life",
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

	if registered.User.EmailVerified {
		t.Fatal("new user must start unverified")
	}

	if !registered.VerificationEmailSent {
		t.Fatal("expected verification email on registration")
	}

	mail, ok := sender.last()
	if !ok {
		t.Fatal("expected a verification email")
	}

	firstToken := tokenFromVerificationURL(t, mail.url)

	// Verify.
	if _, err := verification.VerifyEmail(ctx, firstToken); err != nil {
		t.Fatalf("verify: %v", err)
	}

	reloaded, err := userRepo.FindByID(ctx, registered.User.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}

	if !reloaded.EmailVerified {
		t.Fatal("expected email_verified=true")
	}

	// Reuse rejected.
	if _, err := verification.VerifyEmail(ctx, firstToken); !errors.Is(
		err,
		ErrVerificationTokenUsed,
	) {
		t.Fatalf("expected used error, got %v", err)
	}

	// Login still works and reports verified.
	loggedIn, err := svc.Login(ctx, LoginInput{
		Email:    email,
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if !loggedIn.User.EmailVerified {
		t.Fatal("login must report verified email")
	}

	// Resend for a verified account: silent success, no email.
	before := len(sender.sent)

	if err := verification.ResendVerification(ctx, email); err != nil {
		t.Fatalf("resend for verified account: %v", err)
	}

	if len(sender.sent) != before {
		t.Fatal("no email must be sent for verified accounts")
	}

	// Resend for unknown email: same silent success (no enumeration).
	// Unique per run so Redis cooldowns from earlier runs cannot flake it.
	if err := verification.ResendVerification(
		ctx,
		"nobody-"+time.Now().Format("150405.000000")+"@example.com",
	); err != nil {
		t.Fatalf("resend for unknown email: %v", err)
	}

	// Fresh unverified user for replacement + cooldown checks.
	email2 := "verify-life2-" + time.Now().Format("150405.000000") + "@example.com"

	second, err := svc.Register(ctx, RegisterInput{
		Email:       email2,
		Username:    "verify_life2_" + time.Now().Format("150405"),
		Password:    "Password123",
		DisplayName: "Verify Life 2",
	})
	if err != nil {
		t.Fatalf("register second: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM users WHERE id = $1`,
			second.User.ID,
		)
	}()

	oldToken := tokenFromVerificationURL(t, sender.sent[len(sender.sent)-1].url)

	// Registration issues tokens directly without consuming the resend
	// cooldown, so this resend proceeds and replaces the old token.
	if err := verification.ResendVerification(ctx, email2); err != nil {
		t.Fatalf("resend: %v", err)
	}

	// Old token invalidated by the resend.
	if _, err := verification.VerifyEmail(ctx, oldToken); !errors.Is(
		err,
		ErrInvalidVerificationToken,
	) {
		t.Fatalf("expected old token to be rejected, got %v", err)
	}

	newToken := tokenFromVerificationURL(t, sender.sent[len(sender.sent)-1].url)

	if _, err := verification.VerifyEmail(ctx, newToken); err != nil {
		t.Fatalf("new token must verify: %v", err)
	}

	// Immediate second resend hits the Redis cooldown.
	unverifiedEmail := "verify-life3-" + time.Now().Format("150405.000000") + "@example.com"

	if _, err := svc.Register(ctx, RegisterInput{
		Email:       unverifiedEmail,
		Username:    "verify_life3_" + time.Now().Format("150405"),
		Password:    "Password123",
		DisplayName: "Verify Life 3",
	}); err != nil {
		t.Fatalf("register third: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(
			context.Background(),
			`DELETE FROM users WHERE email = $1`,
			unverifiedEmail,
		)
	}()

	// Registration just sent one; an immediate resend must be limited.
	// (Registration itself does not consume the resend cooldown, so send
	// one resend first, then assert the next is limited.)
	if err := verification.ResendVerification(ctx, unverifiedEmail); err != nil {
		t.Fatalf("first resend: %v", err)
	}

	if err := verification.ResendVerification(ctx, unverifiedEmail); !errors.Is(
		err,
		ErrResendTooSoon,
	) {
		t.Fatalf("expected rate limit, got %v", err)
	}
}

func tokenFromVerificationURL(t *testing.T, rawURL string) string {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse verification URL: %v", err)
	}

	token := parsed.Query().Get("token")
	if token == "" {
		t.Fatalf("no token in URL %q", rawURL)
	}

	return token
}
