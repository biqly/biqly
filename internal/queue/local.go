package queue

import (
	"context"
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

func (q *LocalAIJobQueue) Publish(_ context.Context, jobID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return context.Canceled
	}
	select {
	case q.ch <- jobID:
		return nil
	default:
		q.ch <- jobID
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
				return err
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
