package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Anshul563/edvance-project/services/auth-service/internal/model"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/repository"
	"github.com/Anshul563/edvance-project/services/auth-service/internal/token"
)

// fakeVerificationStore is an in-memory VerificationTokenStore. Like the
// real repository, Consume flips the user's verified flag in the same
// critical section as marking the token used.
type fakeVerificationStore struct {
	mu     sync.Mutex
	byID   map[uuid.UUID]*model.EmailVerificationToken
	byHash map[string]*model.EmailVerificationToken
	users  *fakeUserStore
}

func newFakeVerificationStore() *fakeVerificationStore {
	return &fakeVerificationStore{
		byID:   make(map[uuid.UUID]*model.EmailVerificationToken),
		byHash: make(map[string]*model.EmailVerificationToken),
	}
}

func (f *fakeVerificationStore) Create(
	_ context.Context,
	token *model.EmailVerificationToken,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.byHash[token.TokenHash]; exists {
		return repository.ErrVerificationTokenExists
	}

	token.ID = uuid.New()
	token.CreatedAt = time.Now()

	stored := *token
	f.byID[token.ID] = &stored
	f.byHash[token.TokenHash] = &stored

	return nil
}

func (f *fakeVerificationStore) FindByTokenHash(
	_ context.Context,
	hash string,
) (*model.EmailVerificationToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	token, ok := f.byHash[hash]
	if !ok {
		return nil, repository.ErrVerificationTokenNotFound
	}

	cp := *token

	return &cp, nil
}

func (f *fakeVerificationStore) DeleteForUser(
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

func (f *fakeVerificationStore) Consume(
	_ context.Context,
	tokenID uuid.UUID,
	userID uuid.UUID,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	token, ok := f.byID[tokenID]
	if !ok || token.UserID != userID {
		return repository.ErrVerificationTokenUsed
	}

	if token.Used() {
		return repository.ErrVerificationTokenUsed
	}

	now := time.Now()
	token.UsedAt = &now

	if f.users != nil {
		if user, ok := f.users.byID[userID]; ok {
			user.EmailVerified = true
		}
	}

	return nil
}

// stubSender records verification and reset emails without delivering them.
type stubSender struct {
	mu   sync.Mutex
	sent []sentEmail
	err  error
}

type sentEmail struct {
	to   string
	url  string
	kind string
}

func (s *stubSender) SendVerificationEmail(
	_ context.Context,
	toEmail string,
	verificationURL string,
) error {
	if s.err != nil {
		return s.err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sent = append(s.sent, sentEmail{to: toEmail, url: verificationURL, kind: "verify"})

	return nil
}

func (s *stubSender) SendPasswordResetEmail(
	_ context.Context,
	toEmail string,
	resetURL string,
) error {
	if s.err != nil {
		return s.err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sent = append(s.sent, sentEmail{to: toEmail, url: resetURL, kind: "reset"})

	return nil
}

func (s *stubSender) last() (sentEmail, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.sent) == 0 {
		return sentEmail{}, false
	}

	return s.sent[len(s.sent)-1], true
}

// stubLimiter is a programmable ResendLimiter.
type stubLimiter struct {
	allow bool
	err   error
	calls int
}

func (s *stubLimiter) Allow(
	_ context.Context,
	_ string,
) (bool, error) {
	s.calls++

	return s.allow, s.err
}

type verificationFixture struct {
	service *EmailVerificationService
	users   *fakeUserStore
	tokens  *fakeVerificationStore
	sender  *stubSender
	limiter *stubLimiter
}

func newVerificationFixture() *verificationFixture {
	users := newFakeUserStore()
	tokens := newFakeVerificationStore()
	tokens.users = users
	sender := &stubSender{}
	limiter := &stubLimiter{allow: true}

	svc, err := NewEmailVerificationService(
		tokens,
		users,
		sender,
		limiter,
		EmailVerificationConfig{
			TokenTTL:            24 * time.Hour,
			VerificationBaseURL: "http://localhost:3000/verify-email",
		},
	)
	if err != nil {
		panic(err)
	}

	return &verificationFixture{
		service: svc,
		users:   users,
		tokens:  tokens,
		sender:  sender,
		limiter: limiter,
	}
}

func TestVerifyEmailSuccess(t *testing.T) {
	fx := newVerificationFixture()
	ctx := context.Background()
	user := addFakeUser(t, fx.users, "verify@example.com", model.UserStatusActive)

	raw, err := fx.service.IssueVerification(ctx, user.ID, user.Email)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	// Only the hash may be stored.
	stored, err := fx.tokens.FindByTokenHash(
		ctx,
		token.HashEmailVerificationToken(raw),
	)
	if err != nil {
		t.Fatalf("find stored token: %v", err)
	}

	if stored.TokenHash == raw {
		t.Fatal("raw token must not be stored")
	}

	verified, err := fx.service.VerifyEmail(ctx, raw)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if verified.ID != user.ID || !verified.EmailVerified {
		t.Fatalf("unexpected verified user: %+v", verified)
	}

	updated, err := fx.users.FindByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("reload user: %v", err)
	}

	if !updated.EmailVerified {
		t.Fatal("user must be marked verified")
	}
}

func TestVerifyEmailFailures(t *testing.T) {
	fx := newVerificationFixture()
	ctx := context.Background()
	user := addFakeUser(t, fx.users, "failures@example.com", model.UserStatusActive)

	raw, err := fx.service.IssueVerification(ctx, user.ID, user.Email)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if _, err := fx.service.VerifyEmail(ctx, raw); err != nil {
		t.Fatalf("first verify: %v", err)
	}

	cases := []struct {
		name  string
		token string
		want  error
	}{
		{"reuse fails", raw, ErrVerificationTokenUsed},
		{"unknown token", "does-not-exist", ErrInvalidVerificationToken},
		{"empty token", "", ErrInvalidVerificationToken},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := fx.service.VerifyEmail(ctx, tc.token)
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestVerifyEmailExpired(t *testing.T) {
	fx := newVerificationFixture()
	ctx := context.Background()
	user := addFakeUser(t, fx.users, "expired-verify@example.com", model.UserStatusActive)

	expired := &model.EmailVerificationToken{
		UserID:    user.ID,
		TokenHash: token.HashEmailVerificationToken("expired-token"),
		ExpiresAt: time.Now().Add(-time.Hour),
	}

	if err := fx.tokens.Create(ctx, expired); err != nil {
		t.Fatalf("seed expired token: %v", err)
	}

	_, err := fx.service.VerifyEmail(ctx, "expired-token")
	if !errors.Is(err, ErrVerificationTokenExpired) {
		t.Fatalf("expected expired error, got %v", err)
	}
}

func TestVerifyEmailUserNotFound(t *testing.T) {
	fx := newVerificationFixture()
	ctx := context.Background()

	orphan := &model.EmailVerificationToken{
		UserID:    uuid.New(),
		TokenHash: token.HashEmailVerificationToken("orphan-token"),
		ExpiresAt: time.Now().Add(time.Hour),
	}

	if err := fx.tokens.Create(ctx, orphan); err != nil {
		t.Fatalf("seed orphan token: %v", err)
	}

	_, err := fx.service.VerifyEmail(ctx, "orphan-token")
	if !errors.Is(err, ErrInvalidVerificationToken) {
		t.Fatalf("expected invalid token error, got %v", err)
	}

	// The orphan token must remain unused: failed verification changes nothing.
	stored, err := fx.tokens.FindByTokenHash(
		ctx,
		token.HashEmailVerificationToken("orphan-token"),
	)
	if err != nil {
		t.Fatalf("reload token: %v", err)
	}

	if stored.Used() {
		t.Fatal("failed verification must not consume the token")
	}
}

func TestResendUnverified(t *testing.T) {
	fx := newVerificationFixture()
	ctx := context.Background()
	user := addFakeUser(t, fx.users, "resend@example.com", model.UserStatusActive)

	first, err := fx.service.IssueVerification(ctx, user.ID, user.Email)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if err := fx.service.ResendVerification(ctx, "resend@example.com"); err != nil {
		t.Fatalf("resend: %v", err)
	}

	// The old token is invalidated by the resend.
	_, err = fx.service.VerifyEmail(ctx, first)
	if !errors.Is(err, ErrInvalidVerificationToken) {
		t.Fatalf("expected old token to be rejected, got %v", err)
	}

	// The new emailed token verifies.
	mail, ok := fx.sender.last()
	if !ok {
		t.Fatal("expected a verification email to be sent")
	}

	raw := verificationTokenFromURL(t, mail.url)

	if _, err := fx.service.VerifyEmail(ctx, raw); err != nil {
		t.Fatalf("new token must verify: %v", err)
	}
}

func TestResendNoEnumeration(t *testing.T) {
	fx := newVerificationFixture()
	ctx := context.Background()

	verified := addFakeUser(t, fx.users, "already@example.com", model.UserStatusActive)
	verified.EmailVerified = true
	fx.users.mu.Lock()
	fx.users.byID[verified.ID].EmailVerified = true
	fx.users.byEmail[verified.Email].EmailVerified = true
	fx.users.byUsername[verified.Username].EmailVerified = true
	fx.users.mu.Unlock()

	for _, email := range []string{"nobody@example.com", "already@example.com"} {
		if err := fx.service.ResendVerification(ctx, email); err != nil {
			t.Fatalf("resend for %q must succeed silently, got %v", email, err)
		}
	}

	if len(fx.sender.sent) != 0 {
		t.Fatal("no email must be sent for unknown or verified addresses")
	}
}

func TestResendRateLimited(t *testing.T) {
	fx := newVerificationFixture()
	fx.limiter.allow = false

	err := fx.service.ResendVerification(context.Background(), "any@example.com")
	if !errors.Is(err, ErrResendTooSoon) {
		t.Fatalf("expected too-soon error, got %v", err)
	}
}

func TestRegisterSendsVerification(t *testing.T) {
	users := newFakeUserStore()
	sender := &stubSender{}

	verification, err := NewEmailVerificationService(
		newFakeVerificationStore(),
		users,
		sender,
		&stubLimiter{allow: true},
		EmailVerificationConfig{
			TokenTTL:            24 * time.Hour,
			VerificationBaseURL: "http://localhost:3000/verify-email",
		},
	)
	if err != nil {
		t.Fatalf("verification service: %v", err)
	}

	svc := NewAuthService(
		users,
		newTestSessionService(newFakeSessionStore()),
		15*time.Minute,
		verification,
	)

	out, err := svc.Register(context.Background(), RegisterInput{
		Email:       "newbie@example.com",
		Username:    "newbie",
		Password:    "Password123",
		DisplayName: "Newbie",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if !out.VerificationEmailSent {
		t.Fatal("expected verification email to be sent")
	}

	if len(sender.sent) != 1 || sender.sent[0].to != "newbie@example.com" {
		t.Fatalf("unexpected sent emails: %+v", sender.sent)
	}
}

func TestRegisterSurvivesEmailFailure(t *testing.T) {
	users := newFakeUserStore()
	sender := &stubSender{err: errors.New("SMTP down")}

	verification, err := NewEmailVerificationService(
		newFakeVerificationStore(),
		users,
		sender,
		&stubLimiter{allow: true},
		EmailVerificationConfig{
			TokenTTL:            24 * time.Hour,
			VerificationBaseURL: "http://localhost:3000/verify-email",
		},
	)
	if err != nil {
		t.Fatalf("verification service: %v", err)
	}

	svc := NewAuthService(
		users,
		newTestSessionService(newFakeSessionStore()),
		15*time.Minute,
		verification,
	)

	out, err := svc.Register(context.Background(), RegisterInput{
		Email:       "unlucky@example.com",
		Username:    "unlucky",
		Password:    "Password123",
		DisplayName: "Unlucky",
	})
	if err != nil {
		t.Fatalf("registration must succeed despite email failure: %v", err)
	}

	if out.VerificationEmailSent {
		t.Fatal("expected sent flag to be false after delivery failure")
	}

	if _, err := users.FindByEmail(context.Background(), "unlucky@example.com"); err != nil {
		t.Fatalf("user must exist: %v", err)
	}
}

func verificationTokenFromURL(t *testing.T, rawURL string) string {
	t.Helper()

	const marker = "token="

	idx := -1

	for i := 0; i+len(marker) <= len(rawURL); i++ {
		if rawURL[i:i+len(marker)] == marker {
			idx = i + len(marker)
			break
		}
	}

	if idx < 0 {
		t.Fatalf("no token in URL %q", rawURL)
	}

	return rawURL[idx:]
}
