package provider

import (
	"errors"
	"testing"
)

// TestCheckProviderBaseURL verifies the deployment-mode-aware SSRF guard for
// admin-configured provider base URLs (S12): cloud mode rejects private/internal
// targets, private mode allows anything, airgapped mode allows only private.
func TestCheckProviderBaseURL(t *testing.T) {
	t.Cleanup(func() {
		SetCloud(false)
		SetAirgapped(false)
	})

	t.Run("cloud mode rejects private/metadata hosts", func(t *testing.T) {
		SetAirgapped(false)
		SetCloud(true)
		for _, u := range []string{
			"http://169.254.169.254/latest/meta-data/",
			"http://localhost:8080",
			"http://10.0.0.5:11434",
			"http://ollama.biqly.svc.cluster.local",
		} {
			if err := CheckProviderBaseURL(u); !errors.Is(err, ErrEgressBlocked) {
				t.Errorf("cloud mode should block %q, got %v", u, err)
			}
		}
	})

	t.Run("cloud mode allows public hosts", func(t *testing.T) {
		SetAirgapped(false)
		SetCloud(true)
		if err := CheckProviderBaseURL("https://api.openai.com/v1"); err != nil {
			t.Errorf("cloud mode should allow a public provider, got %v", err)
		}
	})

	t.Run("private mode allows in-cluster hosts", func(t *testing.T) {
		SetAirgapped(false)
		SetCloud(false)
		if err := CheckProviderBaseURL("http://ollama.biqly.svc.cluster.local:11434"); err != nil {
			t.Errorf("private mode should allow a self-hosted provider, got %v", err)
		}
	})

	t.Run("airgapped mode allows only private hosts", func(t *testing.T) {
		SetCloud(false)
		SetAirgapped(true)
		if err := CheckProviderBaseURL("http://ollama.biqly.svc.cluster.local:11434"); err != nil {
			t.Errorf("airgapped mode should allow an in-cluster provider, got %v", err)
		}
		if err := CheckProviderBaseURL("https://api.openai.com/v1"); !errors.Is(err, ErrEgressBlocked) {
			t.Error("airgapped mode should block a public provider")
		}
	})
}
