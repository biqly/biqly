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

// Publisher publishes a byte payload to an explicit subject, keyed for
// dedup/idempotency by the caller. Unlike AIJobPublisher — whose subject is
// fixed at construction time for the legacy AI job pipeline — Publisher
// lets a caller route different jobs to different subjects (e.g. the
// agentic runtime's shadow/beta rollout in internal/http/handlers/ai_job_service.go).
type Publisher interface {
	Publish(ctx context.Context, subject, key string, payload []byte) error
}

// Consumer subscribes to an explicit subject/group, receiving raw payload
// bytes. The subject-aware counterpart to AIJobConsumer.
type Consumer interface {
	Subscribe(ctx context.Context, subject, group string, handler func(context.Context, []byte) error) error
}
