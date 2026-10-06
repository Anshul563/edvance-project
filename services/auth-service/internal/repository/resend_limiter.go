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

// ResendLimiter enforces a per-key cooldown between email-verification
// resend requests using Redis. Keys hash the email so raw addresses never
// sit in Redis keys.
type ResendLimiter struct {
	redis    *redis.Client
	cooldown time.Duration
	prefix   string
}

func NewResendLimiter(
	client *redis.Client,
	cooldown time.Duration,
) *ResendLimiter {
	if cooldown <= 0 {
		cooldown = time.Minute
	}

	return &ResendLimiter{
		redis:    client,
		cooldown: cooldown,
		prefix:   "email_verification_resend",
	}
}

// Allow reports whether a resend for the given email may proceed. The
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
