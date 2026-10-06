package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// ResendLimiter enforces a per-key cooldown between email requests using
// Redis. Keys hash the email (or IP) so raw values never sit in Redis
// keys. The prefix namespaces independent cooldowns (verification resends,
// password resets, IPs) so they never share a window.
type ResendLimiter struct {
	redis    *redis.Client
	cooldown time.Duration
	prefix   string
}

func NewResendLimiter(
	client *redis.Client,
	cooldown time.Duration,
) *ResendLimiter {
	return NewCooldownLimiter(
		client,
		"email_verification_resend",
		cooldown,
	)
}

func NewPasswordResetLimiter(
	client *redis.Client,
	cooldown time.Duration,
) *ResendLimiter {
	return NewCooldownLimiter(
		client,
		"password_reset_request",
		cooldown,
	)
}

func NewPasswordResetIPLimiter(
	client *redis.Client,
	cooldown time.Duration,
) *ResendLimiter {
	return NewCooldownLimiter(
		client,
		"password_reset_request_ip",
		cooldown,
	)
}

func NewCooldownLimiter(
	client *redis.Client,
	prefix string,
	cooldown time.Duration,
) *ResendLimiter {
	if cooldown <= 0 {
		cooldown = time.Minute
	}

	if prefix == "" {
		prefix = "cooldown"
	}

	return &ResendLimiter{
		redis:    client,
		cooldown: cooldown,
		prefix:   prefix,
	}
}

// Allow reports whether a request for the given key may proceed. The
// first call in a cooldown window wins; the rest are refused until the
// window expires. A Redis outage fails closed (error), so the limit
// cannot be bypassed by an unavailable cache.
func (l *ResendLimiter) Allow(
	ctx context.Context,
	email string,
) (bool, error) {
	sum := sha256.Sum256([]byte(strings.ToLower(email)))

	key := fmt.Sprintf(
		"%s:%s",
		l.prefix,
		hex.EncodeToString(sum[:]),
	)

	set, err := l.redis.SetNX(ctx, key, "1", l.cooldown).Result()
	if err != nil {
		return false, fmt.Errorf("resend limiter: %w", err)
	}

	return set, nil
}
