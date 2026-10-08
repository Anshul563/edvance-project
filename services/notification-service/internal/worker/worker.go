// Package worker runs the delivery retry loop: periodically claiming
// due email deliveries and attempting them. Sends already happen inline
// at creation; the worker exists for scheduled retries after transient
// failures. It stops cleanly on context cancellation.
package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Retrier performs one retry sweep. *service.DeliveryService satisfies
// it structurally; tests inject stubs.
type Retrier interface {
	RetryDue(ctx context.Context, now time.Time) error
}

// Run polls every interval with count parallel pollers until ctx ends.
// Each poller claims at most one due delivery per sweep; SKIP LOCKED in
// the claim keeps pollers (and future replicas, once advisory locking
// or queue partitioning lands) from double-sending. Errors are logged
// without payloads, tokens, or credentials — never message bodies.
func Run(
	ctx context.Context,
	retrier Retrier,
	interval time.Duration,
	count int,
) {
	if count < 1 {
		count = 1
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case now := <-ticker.C:
			var wg sync.WaitGroup

			for i := 0; i < count; i++ {
				wg.Add(1)

				go func() {
					defer wg.Done()

					if err := retrier.RetryDue(ctx, now); err != nil {
						slog.Error(
							"worker: retry sweep failed",
							"error", err,
						)
					}
				}()
			}

			wg.Wait()
		}
	}
}
