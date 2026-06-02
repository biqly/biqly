package provider

import "testing"

func TestHTTPProviderURL(t *testing.T) {
	cases := []struct {
		name string
		base string
		path string
		want string
	}{
		{"base without path", "https://api.openai.com/v1", "/embeddings", "https://api.openai.com/v1/embeddings"},
		{"trailing slash", "https://api.openai.com/v1/", "/embeddings", "https://api.openai.com/v1/embeddings"},
		{"base already includes path", "https://openrouter.ai/api/v1/embeddings", "/embeddings", "https://openrouter.ai/api/v1/embeddings"},
		{"chat base includes path", "https://x/v1/chat/completions", "/chat/completions", "https://x/v1/chat/completions"},
		{"chat base without path", "https://x/v1", "/chat/completions", "https://x/v1/chat/completions"},
		{"path missing leading slash", "https://x/v1", "embeddings", "https://x/v1/embeddings"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := httpProvider{baseURL: tc.base}
			if got := p.url(tc.path); got != tc.want {
				t.Fatalf("url(%q) with base %q = %q, want %q", tc.path, tc.base, got, tc.want)
			}
		})
	}
}
