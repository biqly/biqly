package queue

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLocalAIJobQueue_BufferSizes(t *testing.T) {
	t.Run("default buffer size", func(t *testing.T) {
		q := NewLocalAIJobQueue(0)
		assert.Equal(t, 64, cap(q.ch))
		assert.NoError(t, q.Close())
	})

	t.Run("negative buffer size fallback", func(t *testing.T) {
		q := NewLocalAIJobQueue(-10)
		assert.Equal(t, 64, cap(q.ch))
		assert.NoError(t, q.Close())
	})

	t.Run("custom positive buffer size", func(t *testing.T) {
		q := NewLocalAIJobQueue(128)
		assert.Equal(t, 128, cap(q.ch))
		assert.NoError(t, q.Close())
	})
}

func TestLocalAIJobQueue_PublishAndSubscribe(t *testing.T) {
	q := NewLocalAIJobQueue(10)

	t.Run("publish single job", func(t *testing.T) {
		ctx := context.Background()
		err := q.Publish(ctx, "job_123")
		assert.NoError(t, err)
	})

	t.Run("subscribe processes published job", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		var processed []string
		var mu sync.Mutex

		handler := func(_ context.Context, jobID string) error {
			mu.Lock()
			processed = append(processed, jobID)
			mu.Unlock()
			cancel() // Stop the subscription once we process the job
			return nil
		}

		err := q.Subscribe(ctx, "worker-group", handler)
		assert.ErrorIs(t, err, context.Canceled)

		mu.Lock()
		defer mu.Unlock()
		assert.Equal(t, []string{"job_123"}, processed)
	})
}

func TestLocalAIJobQueue_SubscribeWithCancellation(t *testing.T) {
	q := NewLocalAIJobQueue(10)
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)

	var subErr error
	go func() {
		defer wg.Done()
		subErr = q.Subscribe(ctx, "worker-group", func(_ context.Context, _ string) error {
			return nil
		})
	}()

	// sleep a little bit and then cancel context
	time.Sleep(10 * time.Millisecond)
	cancel()
	wg.Wait()

	assert.ErrorIs(t, subErr, context.Canceled)
	assert.NoError(t, q.Close())
}

func TestLocalAIJobQueue_CloseBehavior(t *testing.T) {
	q := NewLocalAIJobQueue(5)

	err := q.Publish(context.Background(), "job_1")
	assert.NoError(t, err)

	err = q.Close()
	assert.NoError(t, err)

	// Publish after close should return error or context canceled
	err = q.Publish(context.Background(), "job_2")
	assert.Error(t, err)

	// Subscribe on closed queue should safely consume remaining then return nil
	processed := []string{}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err = q.Subscribe(ctx, "worker-group", func(_ context.Context, jobID string) error {
		processed = append(processed, jobID)
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, []string{"job_1"}, processed)
}

func TestLocalAIJobQueue_HandlerErrorDoesNotStopConsumer(t *testing.T) {
	q := NewLocalAIJobQueue(5)
	assert.NoError(t, q.Publish(context.Background(), "job_fail"))
	assert.NoError(t, q.Publish(context.Background(), "job_ok"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var processed []string
	err := q.Subscribe(ctx, "worker-group", func(_ context.Context, jobID string) error {
		processed = append(processed, jobID)
		if jobID == "job_fail" {
			return errors.New("something went wrong in worker")
		}
		// Cancel only after the job that follows the failing one is handled,
		// proving a handler error did not tear down the consumer.
		cancel()
		return nil
	})

	// Subscribe returns because the context was cancelled, not because of the
	// handler error: a failing job is logged and consumption continues.
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, []string{"job_fail", "job_ok"}, processed)
	assert.NoError(t, q.Close())
}

func TestConnectNATS_EmptyURLError(t *testing.T) {
	cfg := NATSConfig{
		URL: "",
	}
	client, err := ConnectNATS(cfg)
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "nats url is empty")
}

func TestConnectNATS_ConnectErrorAppliesDefaults(t *testing.T) {
	client, err := ConnectNATS(NATSConfig{URL: "nats://127.0.0.1:1"})
	assert.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "nats connect")
}

func TestLocalAIJobQueue_DoubleCloseIsIdempotent(t *testing.T) {
	q := NewLocalAIJobQueue(4)
	assert.NoError(t, q.Close())
	assert.NoError(t, q.Close())
}

func TestLocalAIJobQueue_ConcurrencyAndDeadlockFix(t *testing.T) {
	q := NewLocalAIJobQueue(1)

	// Publish first job to fill the buffer
	assert.NoError(t, q.Publish(context.Background(), "job_1"))

	// Publish second job with a timeout context; it should block and then return timeout/deadline exceeded
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := q.Publish(ctx, "job_2")
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// Verify we can still close the queue safely (proves mutex is not locked during block)
	assert.NoError(t, q.Close())
}
