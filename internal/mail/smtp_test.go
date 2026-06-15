package mail

import (
	"context"
	"errors"
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
