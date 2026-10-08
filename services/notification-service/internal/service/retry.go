package service

import (
	"time"
)

// Retry delays between email attempts: 1m, 5m, 15m, 1h. Attempt N (1-based)
// waits delays[N-1]; after maxAttempts the row goes terminally failed.
// Isolated here so the policy is one edit and fully unit-testable.
var retryDelays = []time.Duration{
	time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	time.Hour,
}

const maxAttempts = 4

// NextRetry returns the instant for the attempt that just failed
// (attempt is 1-based), or false when attempts are exhausted.
func NextRetry(attempt int32, now time.Time) (time.Time, bool) {
	if attempt < 1 || int(attempt) > len(retryDelays) {
		return time.Time{}, false
	}

	return now.Add(retryDelays[attempt-1]), true
}

// MaxAttempts caps total email send attempts per delivery.
func MaxAttempts() int {
	return len(retryDelays)
}
