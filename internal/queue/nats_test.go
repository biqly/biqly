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
	publish func(ctx context.Context, subject string, payload []byte, opts ...jetstream.PublishOpt) (*jetstream.PubAck, error)
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

func (*mockJetStream) CreateOrUpdateConsumer(context.Context, string, jetstream.ConsumerConfig) (jetstream.Consumer, error) {
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
