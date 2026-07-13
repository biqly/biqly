package queue

import (
	"context"
	"log/slog"
	"sync"
)

type LocalAIJobQueue struct {
	ch     chan string
	closed bool
	mu     sync.Mutex
}

func NewLocalAIJobQueue(buffer int) *LocalAIJobQueue {
	if buffer <= 0 {
		buffer = 64
	}
	return &LocalAIJobQueue{ch: make(chan string, buffer)}
}

func (q *LocalAIJobQueue) Publish(ctx context.Context, jobID string) (err error) {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return context.Canceled
	}
	q.mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			err = context.Canceled
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case q.ch <- jobID:
		return nil
	}
}

func (q *LocalAIJobQueue) Subscribe(ctx context.Context, _ string, handler func(ctx context.Context, jobID string) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case jobID, ok := <-q.ch:
			if !ok {
				return nil
			}
			if err := handler(ctx, jobID); err != nil {
				// Log and continue: one failing job must not tear down the whole
				// consumer. This is the dev/in-process backend (the NATS backend
				// owns retry/DLQ); dropping to at-least-once-with-logging here
				// keeps a transient failure from silently halting all processing.
				slog.ErrorContext(ctx, "local AI job handler failed; continuing", "job_id", jobID, "error", err)
			}
		}
	}
}

func (q *LocalAIJobQueue) Close() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.closed {
		q.closed = true
		close(q.ch)
	}
	return nil
}
