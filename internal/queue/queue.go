// Package queue provides AI job publish and consume abstractions with local and NATS backends.
package queue

import "context"

const (
	AIJobSubject    = "biqly.ai.jobs"
	AIJobDLQSubject = "biqly.ai.jobs.dlq"
	AIJobStream     = "BIQLY_AI_JOBS"
)

type AIJobPublisher interface {
	Publish(ctx context.Context, jobID string) error
	Close() error
}

type AIJobConsumer interface {
	Subscribe(ctx context.Context, group string, handler func(ctx context.Context, jobID string) error) error
	Close() error
}
