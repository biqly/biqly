package mail

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingBlockListRepo struct {
	err error
}

func (r failingBlockListRepo) IsBlocked(context.Context, string) (bool, error) {
	return false, r.err
}

func (failingBlockListRepo) Block(context.Context, string, string, string) error {
	return nil
}

func (failingBlockListRepo) Unblock(context.Context, string) error {
	return nil
}

func (failingBlockListRepo) List(context.Context, int, int) ([]BlockedEmail, error) {
	return nil, nil
}

func TestSendTemplateFailsClosedWhenBlockListCheckFails(t *testing.T) {
	blockErr := errors.New("block-list backend down")
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, failingBlockListRepo{err: blockErr}, nil)
	require.NoError(t, err)

	err = sender.sendTemplate(context.Background(), "user@example.com", "verification", map[string]any{
		"URL": "https://app.example.com/auth/verify-email?token=x",
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, blockErr)
	assert.ErrorContains(t, err, "email block-list check")
}

func TestSendTemplateSuppressesBlockedRecipient(t *testing.T) {
	blocks := NewMemoryEmailBlockListRepo()
	require.NoError(t, blocks.Block(context.Background(), "user@example.com", "bounce", "test"))
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
	}, blocks, nil)
	require.NoError(t, err)

	err = sender.sendTemplate(context.Background(), "user@example.com", "verification", map[string]any{
		"URL": "https://app.example.com/auth/verify-email?token=x",
	})

	assert.ErrorIs(t, err, ErrEmailBlocked)
}

func TestSendTemplateFallsBackSynchronouslyWhenQueueFull(t *testing.T) {
	registry, err := newEmailTemplateRegistry("en")
	require.NoError(t, err)
	sender := &SMTPEmailSender{
		config: &Config{
			EmailDefaultLocale: "en",
			FrontendBaseURL:    "https://app.example.com",
			SMTPFrom:           "no-reply@example.com",
		},
		registry: registry,
		queue:    make(chan emailJob, 1),
		stop:     make(chan struct{}),
	}
	sender.queue <- emailJob{to: "queued@example.com"}

	err = sender.sendTemplate(context.Background(), "user@example.com", "verification", map[string]any{
		"URL": "https://app.example.com/auth/verify-email?token=x",
	})

	require.Error(t, err)
	assert.ErrorContains(t, err, "SMTP host is not configured")
}

func TestSMTPEmailSenderCloseIsConcurrentSafe(t *testing.T) {
	sender, err := NewSMTPEmailSender(&Config{
		EmailDefaultLocale: "en",
		FrontendBaseURL:    "https://app.example.com",
		EmailQueueSize:     1,
		SMTPFrom:           "no-reply@example.com",
	}, nil, nil)
	require.NoError(t, err)

	start := make(chan struct{})
	var panics atomic.Int64
	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if recover() != nil {
					panics.Add(1)
				}
			}()
			<-start
			sender.Close()
		}()
	}
	close(start)
	wg.Wait()

	assert.Equal(t, int64(0), panics.Load())
	err = sender.sendTemplate(context.Background(), "user@example.com", "verification", map[string]any{
		"URL": "https://app.example.com/auth/verify-email?token=x",
	})
	assert.ErrorContains(t, err, "email sender closed")
}
