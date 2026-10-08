package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type stubRetrier struct {
	calls atomic.Int32
}

func (s *stubRetrier) RetryDue(_ context.Context, _ time.Time) error {
	s.calls.Add(1)

	return nil
}

func TestRunSweepsAndStops(t *testing.T) {
	stub := &stubRetrier{}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})

	go func() {
		defer close(done)

		Run(ctx, stub, 5*time.Millisecond, 3)
	}()

	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop on cancel")
	}

	if stub.calls.Load() == 0 {
		t.Fatal("expected sweep calls before cancel")
	}
}

func TestRunDefaultsSinglePoller(t *testing.T) {
	stub := &stubRetrier{}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})

	go func() {
		defer close(done)

		Run(ctx, stub, 5*time.Millisecond, 0)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop on cancel")
	}

	if stub.calls.Load() == 0 {
		t.Fatal("count 0 must still run one poller")
	}
}
