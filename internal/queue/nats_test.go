package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockMsg is a minimal jetstream.Msg used to exercise handleAIJobFailure's
// redelivery (Nak) path without a live NATS server. The DLQ/Term path requires
// a real JetStream context and is covered by integration tests.
type mockMsg struct {
	meta     *jetstream.MsgMetadata
	metaErr  error
	nakCalls int
}

func (m *mockMsg) Metadata() (*jetstream.MsgMetadata, error) { return m.meta, m.metaErr }
func (*mockMsg) Data() []byte                                { return []byte("job-1") }
func (*mockMsg) Headers() nats.Header                        { return nil }
func (*mockMsg) Subject() string                             { return "" }
func (*mockMsg) Reply() string                               { return "" }
func (*mockMsg) Ack() error                                  { return nil }
func (*mockMsg) DoubleAck(context.Context) error             { return nil }
func (m *mockMsg) Nak() error                                { m.nakCalls++; return nil }
func (*mockMsg) NakWithDelay(time.Duration) error            { return nil }
func (*mockMsg) InProgress() error                           { return nil }
func (*mockMsg) Term() error                                 { return nil }
func (*mockMsg) TermWithReason(string) error                 { return nil }

func TestHandleAIJobFailureNaksBelowMaxDeliver(t *testing.T) {
	q := &NATSQueue{}
	msg := &mockMsg{meta: &jetstream.MsgMetadata{NumDelivered: 1}}
	q.handleAIJobFailure(context.Background(), msg, "job-1")
	assert.Equal(t, 1, msg.nakCalls, "should redeliver (Nak) while below maxDeliver")
}

func TestHandleAIJobFailureNaksOnMetadataError(t *testing.T) {
	q := &NATSQueue{}
	msg := &mockMsg{metaErr: errors.New("no metadata")}
	q.handleAIJobFailure(context.Background(), msg, "job-1")
	assert.Equal(t, 1, msg.nakCalls, "metadata error should fall back to Nak")
}

func TestNATSQueueCloseNilConn(t *testing.T) {
	// A zero-value NATSQueue (nil underlying connection) closes cleanly.
	assert.NoError(t, (&NATSQueue{}).Close())
}

type mockJetStream struct {
	publish        func(ctx context.Context, subject string, payload []byte, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error)
	createConsumer func(ctx context.Context, stream string, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error)
}

func (m *mockJetStream) Publish(ctx context.Context, subject string, payload []byte, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
	if m.publish != nil {
		return m.publish(ctx, subject, payload, opts...)
	}
	return &jetstream.PubAck{}, nil
}

func (*mockJetStream) CreateOrUpdateStream(context.Context, jetstream.StreamConfig) (jetstream.Stream, error) {
	return nil, errors.New("not implemented")
}

func (m *mockJetStream) CreateOrUpdateConsumer(ctx context.Context, stream string, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error) {
	if m.createConsumer != nil {
		return m.createConsumer(ctx, stream, cfg)
	}
	return nil, errors.New("not implemented")
}

type mockMsgWithNakErr struct {
	mockMsg
	nakErr error
}

func (m *mockMsgWithNakErr) Nak() error {
	m.nakCalls++
	return m.nakErr
}

type mockMsgAtMaxDeliver struct {
	mockMsg
	termCalls int
}

func (m *mockMsgAtMaxDeliver) Term() error {
	m.termCalls++
	return nil
}

func TestNATSQueuePublishSuccess(t *testing.T) {
	var publishedSubject string
	q := &NATSQueue{
		js: &mockJetStream{
			publish: func(_ context.Context, subject string, payload []byte, _ ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
				publishedSubject = subject
				assert.Equal(t, []byte("job-42"), payload)
				return &jetstream.PubAck{}, nil
			},
		},
		subj: AIJobSubject,
	}

	require.NoError(t, q.Publish(context.Background(), "job-42"))
	assert.Equal(t, AIJobSubject, publishedSubject)
}

func TestNATSQueuePublishError(t *testing.T) {
	q := &NATSQueue{
		js: &mockJetStream{
			publish: func(context.Context, string, []byte, ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
				return nil, errors.New("publish failed")
			},
		},
		subj: AIJobSubject,
	}

	err := q.Publish(context.Background(), "job-42")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "publish ai job")
}

func TestHandleAIJobFailureMovesToDLQAtMaxDeliver(t *testing.T) {
	var dlqSubject string
	q := &NATSQueue{
		js: &mockJetStream{
			publish: func(_ context.Context, subject string, payload []byte, _ ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
				dlqSubject = subject
				assert.Equal(t, []byte("job-1"), payload)
				return &jetstream.PubAck{}, nil
			},
		},
	}
	msg := &mockMsgAtMaxDeliver{mockMsg: mockMsg{meta: &jetstream.MsgMetadata{NumDelivered: 3}}}
	q.handleAIJobFailure(context.Background(), msg, "job-1")
	assert.Equal(t, AIJobDLQSubject, dlqSubject)
	assert.Equal(t, 0, msg.nakCalls)
	assert.Equal(t, 1, msg.termCalls)
}

func TestHandleAIJobFailureDLQPublishError(t *testing.T) {
	q := &NATSQueue{
		js: &mockJetStream{
			publish: func(context.Context, string, []byte, ...jetstream.PublishOpt) (*jetstream.PubAck, error) {
				return nil, errors.New("dlq unavailable")
			},
		},
	}
	msg := &mockMsgAtMaxDeliver{mockMsg: mockMsg{meta: &jetstream.MsgMetadata{NumDelivered: 3}}}
	q.handleAIJobFailure(context.Background(), msg, "job-1")
	assert.Equal(t, 0, msg.nakCalls)
	assert.Equal(t, 1, msg.termCalls)
}

func TestHandleAIJobFailureNakError(t *testing.T) {
	q := &NATSQueue{}
	msg := &mockMsgWithNakErr{
		mockMsg: mockMsg{meta: &jetstream.MsgMetadata{NumDelivered: 1}},
		nakErr:  errors.New("nak failed"),
	}
	q.handleAIJobFailure(context.Background(), msg, "job-1")
	assert.Equal(t, 1, msg.nakCalls)
}

type mockConsumer struct {
	jetstream.Consumer
	consumeFunc func(jetstream.MessageHandler, ...jetstream.PullConsumeOpt) (jetstream.ConsumeContext, error)
}

func (*mockConsumer) Info(context.Context) (*jetstream.ConsumerInfo, error) {
	return &jetstream.ConsumerInfo{NumPending: 5}, nil
}

func (m *mockConsumer) Consume(handler jetstream.MessageHandler, opts ...jetstream.PullConsumeOpt) (jetstream.ConsumeContext, error) {
	if m.consumeFunc != nil {
		return m.consumeFunc(handler, opts...)
	}
	return &mockConsumeContext{}, nil
}

type mockConsumeContext struct{}

func (*mockConsumeContext) Stop()                   {}
func (*mockConsumeContext) Drain()                  {}
func (*mockConsumeContext) Closed() <-chan struct{} { return nil }

func TestNATSQueueSubscribeSuccess(t *testing.T) {
	mockCons := &mockConsumer{
		consumeFunc: func(handler jetstream.MessageHandler, _ ...jetstream.PullConsumeOpt) (jetstream.ConsumeContext, error) {
			msg := &mockMsg{meta: &jetstream.MsgMetadata{NumDelivered: 1}}
			handler(msg)
			return &mockConsumeContext{}, nil
		},
	}

	q := &NATSQueue{
		js: &mockJetStream{
			createConsumer: func(_ context.Context, stream string, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error) {
				assert.Equal(t, "test-stream", stream)
				assert.Equal(t, "test-subj", cfg.FilterSubject)
				return mockCons, nil
			},
		},
		stream: "test-stream",
		subj:   "test-subj",
	}

	handlerCalled := false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := q.Subscribe(ctx, "test-group", func(_ context.Context, jobID string) error {
		handlerCalled = true
		assert.Equal(t, "job-1", jobID)
		return nil
	})

	require.NoError(t, err)
	assert.True(t, handlerCalled)
}

func TestNATSQueueSubscribeCreateConsumerError(t *testing.T) {
	q := &NATSQueue{
		js: &mockJetStream{
			createConsumer: func(_ context.Context, _ string, _ jetstream.ConsumerConfig) (jetstream.Consumer, error) {
				return nil, errors.New("create consumer failed")
			},
		},
		stream: "test-stream",
		subj:   "test-subj",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := q.Subscribe(ctx, "test-group", func(_ context.Context, _ string) error {
		return nil
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create consumer failed")
}

func TestNATSQueueSubscribeHandlerError(t *testing.T) {
	mockCons := &mockConsumer{
		consumeFunc: func(handler jetstream.MessageHandler, _ ...jetstream.PullConsumeOpt) (jetstream.ConsumeContext, error) {
			msg := &mockMsg{meta: &jetstream.MsgMetadata{NumDelivered: 1}}
			handler(msg)
			return &mockConsumeContext{}, nil
		},
	}

	q := &NATSQueue{
		js: &mockJetStream{
			createConsumer: func(_ context.Context, _ string, _ jetstream.ConsumerConfig) (jetstream.Consumer, error) {
				return mockCons, nil
			},
		},
		stream: "test-stream",
		subj:   "test-subj",
	}

	handlerCalled := false
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := q.Subscribe(ctx, "test-group", func(_ context.Context, _ string) error {
		handlerCalled = true
		return errors.New("handler error")
	})

	require.NoError(t, err)
	assert.True(t, handlerCalled)
}

func TestConnectNATS_Errors(t *testing.T) {
	// Empty URL error
	q, err := ConnectNATS(NATSConfig{URL: ""})
	assert.Nil(t, q)
	assert.EqualError(t, err, "nats url is empty")

	// Invalid URL connection error
	q2, err2 := ConnectNATS(NATSConfig{URL: "nats://127.0.0.1:23456"})
	assert.Nil(t, q2)
	assert.Error(t, err2)
	assert.Contains(t, err2.Error(), "nats connect")
}
