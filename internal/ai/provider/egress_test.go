package provider

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckEgressDisabledAllowsEverything(t *testing.T) {
	SetAirgapped(false)
	assert.NoError(t, CheckEgress("https://api.openai.com/v1/chat/completions"))
}

func TestCheckEgressAirgapped(t *testing.T) {
	SetAirgapped(true)
	t.Cleanup(func() { SetAirgapped(false) })

	allowed := []string{
		"http://localhost:11434/v1/chat/completions",
		"http://127.0.0.1:8080/v1",
		"http://10.1.2.3:8000/v1",
		"http://192.168.1.10/v1",
		"http://ollama:11434/v1",
		"http://ollama.ai.svc/v1",
		"http://ollama.ai.svc.cluster.local:11434/v1",
		"http://models.internal/v1",
	}
	for _, u := range allowed {
		assert.NoError(t, CheckEgress(u), u)
	}

	blocked := []string{
		"https://api.openai.com/v1/chat/completions",
		"https://api.anthropic.com/v1/messages",
		"http://8.8.8.8/v1",
		"https://example.com/v1",
	}
	for _, u := range blocked {
		err := CheckEgress(u)
		require.Error(t, err, u)
		assert.True(t, errors.Is(err, ErrEgressBlocked), u)
	}
}
