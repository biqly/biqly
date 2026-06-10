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
}

type NATSQueue struct {
	nc     *nats.Conn
	js     jetStreamClient
	stream string
	subj   string
}

func ConnectNATS(cfg NATSConfig) (*NATSQueue, error) {
	if cfg.URL == "" {
		return nil, errors.New("nats url is empty")
	}
	if cfg.Stream == "" {
		cfg.Stream = AIJobStream
	}
	if cfg.Subject == "" {
		cfg.Subject = AIJobSubject
	}
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
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      cfg.Stream,
		Subjects:  []string{cfg.Subject, AIJobDLQSubject},
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
		slog.Error("publish ai job to dlq", "job_id", jobID, "error", dlqErr)
	} else {
		slog.Warn("ai job moved to dlq after max deliveries", "job_id", jobID, "deliveries", meta.NumDelivered)
		observability.Default().RecordNATSDLQMove()
	}
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
