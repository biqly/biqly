package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/biqly/biqly/internal/platform/observability"
)

const aiJobMaxDeliver = 3

type NATSConfig struct {
	URL     string
	Stream  string
	Subject string
}

// jetStreamClient is the subset of jetstream.JetStream used by NATSQueue.
// It exists so unit tests can inject mocks without a live NATS server.
type jetStreamClient interface {
	CreateOrUpdateStream(ctx context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error)
	CreateOrUpdateConsumer(ctx context.Context, stream string, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error)
	Publish(ctx context.Context, subject string, payload []byte, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error)
	Stream(ctx context.Context, name string) (jetstream.Stream, error)
}

type NATSQueue struct {
	nc     *nats.Conn
	js     jetStreamClient
	stream string
	subj   string
}

func normalizeNATSConfig(cfg NATSConfig) NATSConfig {
	if cfg.Stream == "" {
		cfg.Stream = AIJobStream
	}
	if cfg.Subject == "" {
		cfg.Subject = AIJobSubject
	}
	return cfg
}

// mergeStreamSubjects appends existing subjects not already present in desired,
// preserving the order of desired first then existing. This ensures every caller's
// CreateOrUpdateStream preserves subjects registered by other services sharing
// the same JetStream stream (e.g. legacy AI pipeline + agentic runner).
func mergeStreamSubjects(existing, desired []string) []string {
	seen := make(map[string]bool, len(desired))
	merged := make([]string, 0, len(desired))
	for _, s := range desired {
		seen[s] = true
		merged = append(merged, s)
	}
	for _, s := range existing {
		if !seen[s] {
			merged = append(merged, s)
			seen[s] = true
		}
	}
	return merged
}

func ConnectNATS(cfg NATSConfig) (*NATSQueue, error) {
	if cfg.URL == "" {
		return nil, errors.New("nats url is empty")
	}
	cfg = normalizeNATSConfig(cfg)
	nc, err := nats.Connect(cfg.URL, nats.Timeout(10*time.Second))
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	subjects := []string{cfg.Subject, AIJobDLQSubject}

	// Merge with existing stream subjects instead of replacing them.
	// Multiple services (legacy AI pipeline + agentic runner) share the
	// same JetStream stream. Every caller's CreateOrUpdateStream must
	// preserve subjects registered by other callers, or the first caller
	// to restart after another's connect silently drops the other's
	// subjects — breaking that service's job publishing.
	if existing, sErr := js.Stream(ctx, cfg.Stream); sErr == nil {
		info, iErr := existing.Info(ctx)
		if iErr == nil {
			subjects = mergeStreamSubjects(info.Config.Subjects, subjects)
		}
	}

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      cfg.Stream,
		Subjects:  subjects,
		Retention: jetstream.WorkQueuePolicy,
		MaxAge:    7 * 24 * time.Hour,
	})
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("create stream: %w", err)
	}
	return &NATSQueue{nc: nc, js: js, stream: cfg.Stream, subj: cfg.Subject}, nil
}

func (q *NATSQueue) Publish(ctx context.Context, jobID string) (err error) {
	start := time.Now()
	defer func() {
		observability.Default().RecordNATSPublish(time.Since(start), err == nil)
	}()
	ctx, span := otel.Tracer("biqly/queue").Start(ctx, "nats.publish "+q.subj,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("peer.service", "biqly-nats"),
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", q.subj),
		))
	defer span.End()
	if _, err = q.js.Publish(ctx, q.subj, []byte(jobID)); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("publish ai job: %w", err)
	}
	return nil
}

func (q *NATSQueue) Subscribe(ctx context.Context, group string, handler func(ctx context.Context, jobID string) error) error {
	if group == "" {
		group = "biqly-ai-workers"
	}
	cons, err := q.js.CreateOrUpdateConsumer(ctx, q.stream, jetstream.ConsumerConfig{
		Durable:       group,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: q.subj,
		AckWait:       30 * time.Minute,
		MaxDeliver:    aiJobMaxDeliver,
	})
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
				info, err := cons.Info(cctx)
				cancel()
				if err == nil {
					observability.Default().RecordNATSConsumerPending(info.NumPending)
				}
			}
		}
	}()
	_, err = cons.Consume(func(msg jetstream.Msg) {
		jobID := string(msg.Data())
		hctx, cancel := context.WithTimeout(ctx, 35*time.Minute)
		defer cancel()
		var consumeErr error
		defer func() {
			observability.Default().RecordNATSConsume(consumeErr == nil)
		}()
		if err := handler(hctx, jobID); err != nil {
			consumeErr = err
			q.handleAIJobFailure(ctx, msg, jobID)
			return
		}
		if ackErr := msg.Ack(); ackErr != nil {
			slog.Warn("ack ai job message", "job_id", jobID, "error", ackErr)
		}
	})
	return err
}

func (q *NATSQueue) handleAIJobFailure(ctx context.Context, msg jetstream.Msg, jobID string) {
	meta, metaErr := msg.Metadata()
	if metaErr != nil || meta.NumDelivered < aiJobMaxDeliver {
		if nakErr := msg.Nak(); nakErr != nil {
			slog.Warn("nack ai job message", "job_id", jobID, "error", nakErr)
		}
		return
	}
	if _, dlqErr := q.js.Publish(ctx, AIJobDLQSubject, msg.Data()); dlqErr != nil {
		// Do NOT Term() when the DLQ write failed — that would drop the message
		// with no dead-letter copy anywhere. Nak so JetStream redelivers and we
		// get another chance to DLQ it.
		slog.Error("publish ai job to dlq, will retry", "job_id", jobID, "error", dlqErr)
		if nakErr := msg.Nak(); nakErr != nil {
			slog.Warn("nack ai job message after failed dlq publish", "job_id", jobID, "error", nakErr)
		}
		return
	}
	slog.Warn("ai job moved to dlq after max deliveries", "job_id", jobID, "deliveries", meta.NumDelivered)
	observability.Default().RecordNATSDLQMove()
	if termErr := msg.Term(); termErr != nil {
		slog.Warn("term ai job message after dlq", "job_id", jobID, "error", termErr)
	}
}

func (q *NATSQueue) Close() error {
	if q.nc != nil {
		q.nc.Close()
	}
	return nil
}

// SubjectQueue returns a subject-aware Publisher/Consumer backed by the same
// JetStream connection and stream as q, for callers (e.g. the agentic
// runtime's job router) that need to address a subject other than the one
// this NATSQueue was constructed with. It never touches q's existing
// fixed-subject Publish/Subscribe, which the legacy AI job pipeline keeps
// using unchanged.
func (q *NATSQueue) SubjectQueue() *NATSSubjectQueue {
	return &NATSSubjectQueue{js: q.js, stream: q.stream}
}

// NATSSubjectQueue implements Publisher and Consumer against an explicit
// subject per call, reusing one JetStream connection/stream across subjects.
type NATSSubjectQueue struct {
	js     jetStreamClient
	stream string
}

// Publish implements Publisher. key becomes the JetStream message ID for
// producer-side dedup; it should be stable across redelivery attempts by
// the caller (e.g. the job id).
func (q *NATSSubjectQueue) Publish(ctx context.Context, subject, key string, payload []byte) error {
	opts := []jetstream.PublishOpt{}
	if key != "" {
		opts = append(opts, jetstream.WithMsgID(key))
	}
	if _, err := q.js.Publish(ctx, subject, payload, opts...); err != nil {
		return fmt.Errorf("publish to subject %s: %w", subject, err)
	}
	return nil
}

// Subscribe implements Consumer: an explicit-subject, at-least-once,
// explicit-ack durable consumer. Redelivery/DLQ handling belongs to the
// caller's handler — unlike NATSQueue.Subscribe, this generic subject
// consumer has no fixed notion of "the AI job DLQ subject".
func (q *NATSSubjectQueue) Subscribe(ctx context.Context, subject, group string, handler func(context.Context, []byte) error) error {
	if group == "" {
		return errors.New("subscribe: group is required")
	}
	cons, err := q.js.CreateOrUpdateConsumer(ctx, q.stream, jetstream.ConsumerConfig{
		Durable:       group,
		AckPolicy:     jetstream.AckExplicitPolicy,
		FilterSubject: subject,
		AckWait:       30 * time.Minute,
		MaxDeliver:    aiJobMaxDeliver,
	})
	if err != nil {
		return fmt.Errorf("create consumer for subject %s: %w", subject, err)
	}
	_, err = cons.Consume(func(msg jetstream.Msg) {
		hctx, cancel := context.WithTimeout(ctx, 35*time.Minute)
		defer cancel()
		if err := handler(hctx, msg.Data()); err != nil {
			if nakErr := msg.Nak(); nakErr != nil {
				slog.Warn("nack subject message", "subject", subject, "error", nakErr)
			}
			return
		}
		if ackErr := msg.Ack(); ackErr != nil {
			slog.Warn("ack subject message", "subject", subject, "error", ackErr)
		}
	})
	return err
}
