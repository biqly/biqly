package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

const defaultRemoteModelsTimeout = 20 * time.Second

// RemoteModelOption is a model id returned by a provider's remote catalog API.
type RemoteModelOption struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by,omitempty"`
}

type remoteModelsEnvelope struct {
	Data []remoteModelEntry `json:"data"`
}

type remoteModelEntry struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by"`
	Type    string `json:"type"`
}

// ListRemoteModelsFromEndpoint calls the provider's model catalog API.
// OpenAI-compatible providers use GET {base_url}/models with Bearer auth.
// Anthropic uses GET {base_url}/models with x-api-key and anthropic-version.
func ListRemoteModelsFromEndpoint(ctx context.Context, providerType, baseURL, apiKey string, timeout time.Duration) ([]RemoteModelOption, error) {
	if timeout <= 0 {
		timeout = defaultRemoteModelsTimeout
	}
	pt := strings.ToLower(strings.TrimSpace(providerType))
	switch pt {
	case "openai", "openai-compatible", "":
		return listOpenAICompatibleModels(ctx, baseURL, apiKey, timeout)
	case "anthropic":
		return listAnthropicModels(ctx, baseURL, apiKey, timeout)
	default:
		return nil, fmt.Errorf("listing remote models is not supported for provider type %q", providerType)
	}
}

func listOpenAICompatibleModels(ctx context.Context, baseURL, apiKey string, timeout time.Duration) ([]RemoteModelOption, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return nil, errors.New("base URL is required to list models")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", http.NoBody)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	return doRemoteModelsRequest(req, timeout)
}

func listAnthropicModels(ctx context.Context, baseURL, apiKey string, timeout time.Duration) ([]RemoteModelOption, error) {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "https://api.anthropic.com/v1"
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("API key is required to list models")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/models", http.NoBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Accept", "application/json")
	return doRemoteModelsRequest(req, timeout)
}

func doRemoteModelsRequest(req *http.Request, timeout time.Duration) ([]RemoteModelOption, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 300 {
			msg = msg[:300] + "…"
		}
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("remote API returned %d: %s", resp.StatusCode, msg)
	}

	var env remoteModelsEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}
	if len(env.Data) == 0 {
		return []RemoteModelOption{}, nil
	}

	out := make([]RemoteModelOption, 0, len(env.Data))
	seen := make(map[string]struct{}, len(env.Data))
	for _, row := range env.Data {
		id := strings.TrimSpace(row.ID)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, RemoteModelOption{ID: id, OwnedBy: strings.TrimSpace(row.OwnedBy)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
