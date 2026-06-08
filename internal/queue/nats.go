package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
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

func (q *NATSQueue) Publish(ctx context.Context, jobID string) error {
	_, err := q.js.Publish(ctx, q.subj, []byte(jobID))
	if err != nil {
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
	_, err = cons.Consume(func(msg jetstream.Msg) {
		jobID := string(msg.Data())
		hctx, cancel := context.WithTimeout(ctx, 35*time.Minute)
		defer cancel()
		if err := handler(hctx, jobID); err != nil {
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
