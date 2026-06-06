package queue

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
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
	q.handleAIJobFailure(context.Background(), msg, "job-1", 3)
	assert.Equal(t, 1, msg.nakCalls, "should redeliver (Nak) while below maxDeliver")
}

func TestHandleAIJobFailureNaksOnMetadataError(t *testing.T) {
	q := &NATSQueue{}
	msg := &mockMsg{metaErr: errors.New("no metadata")}
	q.handleAIJobFailure(context.Background(), msg, "job-1", 3)
	assert.Equal(t, 1, msg.nakCalls, "metadata error should fall back to Nak")
}

func TestNATSQueueCloseNilConn(t *testing.T) {
	// A zero-value NATSQueue (nil underlying connection) closes cleanly.
	assert.NoError(t, (&NATSQueue{}).Close())
}
