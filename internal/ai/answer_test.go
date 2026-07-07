package ai

import (
	"context"
	stderrors "errors"
	"strings"
	"testing"

	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/query"
)

type stubAnswerProvider struct {
	content    string
	err        error
	calls      int
	lastPrompt string
}

func (p *stubAnswerProvider) Generate(_ context.Context, prompt string) (providerpkg.GenerationResult, error) {
	p.calls++
	p.lastPrompt = prompt
	if p.err != nil {
		return providerpkg.GenerationResult{}, p.err
	}
	return providerpkg.GenerationResult{Content: p.content}, nil
}

func (p *stubAnswerProvider) GenerateAt(ctx context.Context, prompt string, _ float64) (providerpkg.GenerationResult, error) {
	return p.Generate(ctx, prompt)
}

func sampleResult() *query.Result {
	return &query.Result{
		Columns: []query.ResultColumn{{Name: "tweet_count"}},
		Rows:    [][]any{{5658}},
		Stats:   query.Stats{RowCount: 1},
	}
}

func TestSynthesizeAnswerReturnsTextOnSuccess(t *testing.T) {
	provider := &stubAnswerProvider{content: "  Geçen hafta 5.658 tweet atılmıştır.\n"}
	svc := &Service{client: provider, answerEnabled: true}

	got := svc.SynthesizeAnswer(context.Background(), "geçen hafta kaç tweet atıldı", "tr", sampleResult())
	if got != "Geçen hafta 5.658 tweet atılmıştır." {
		t.Fatalf("SynthesizeAnswer = %q, want trimmed answer", got)
	}
	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}
	if !strings.Contains(provider.lastPrompt, "5658") {
		t.Errorf("prompt missing result value; got:\n%s", provider.lastPrompt)
	}
	if !strings.Contains(provider.lastPrompt, "geçen hafta kaç tweet atıldı") {
		t.Errorf("prompt missing question; got:\n%s", provider.lastPrompt)
	}
}

func TestSynthesizeAnswerReturnsEmptyOnProviderError(t *testing.T) {
	provider := &stubAnswerProvider{err: stderrors.New("boom")}
	svc := &Service{client: provider, answerEnabled: true}

	if got := svc.SynthesizeAnswer(context.Background(), "q", "en", sampleResult()); got != "" {
		t.Fatalf("SynthesizeAnswer = %q, want empty on provider error", got)
	}
}

func TestSynthesizeAnswerSkippedWhenDisabledOrNoData(t *testing.T) {
	provider := &stubAnswerProvider{content: "should not be used"}
	disabled := &Service{client: provider, answerEnabled: false}
	if got := disabled.SynthesizeAnswer(context.Background(), "q", "en", sampleResult()); got != "" {
		t.Fatalf("disabled SynthesizeAnswer = %q, want empty", got)
	}

	enabled := &Service{client: provider, answerEnabled: true}
	if got := enabled.SynthesizeAnswer(context.Background(), "q", "en", nil); got != "" {
		t.Fatalf("nil-result SynthesizeAnswer = %q, want empty", got)
	}
	empty := &query.Result{Columns: []query.ResultColumn{{Name: "c"}}, Rows: [][]any{}}
	if got := enabled.SynthesizeAnswer(context.Background(), "q", "en", empty); got != "" {
		t.Fatalf("empty-rows SynthesizeAnswer = %q, want empty", got)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0 (no LLM call when gated/empty)", provider.calls)
	}
}

func TestRenderResultForAnswerFormatsStringNumerics(t *testing.T) {
	t.Parallel()
	// SQL numeric/decimal columns (e.g. a percent_change formula) arrive as
	// strings; the answer prompt must apply the same percent/number formatting the
	// table does instead of leaking the raw digits.
	result := &query.Result{
		Columns: []query.ResultColumn{
			{Name: "fark", Format: query.FormatNumber},
			{Name: "degisim_orani", Format: query.FormatPercent},
		},
		Rows:  [][]any{{"-227", "-27.3164861612515042"}},
		Stats: query.Stats{RowCount: 1},
	}
	got := renderResultForAnswer(result, "tr")
	if strings.Contains(got, "-27.3164861612515042") {
		t.Fatalf("rendered raw percent digits; got:\n%s", got)
	}
	if !strings.Contains(got, "-27,3%") {
		t.Fatalf("percent not formatted to 1 decimal with %% sign; got:\n%s", got)
	}
	if !strings.Contains(got, "-227") {
		t.Fatalf("number cell missing; got:\n%s", got)
	}
}

func TestSanitizeAnswerText(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, in, want string
	}{
		{"plain", "Dün 589 tweet atılmıştır.", "Dün 589 tweet atılmıştır."},
		{"json answer key", `{"answer": "Dün 589 tweet atılmıştır."}`, "Dün 589 tweet atılmıştır."},
		{"json other key", `{"cevap": "589 tweet."}`, "589 tweet."},
		{"fenced json", "```json\n{\"answer\": \"589 tweet.\"}\n```", "589 tweet."},
		{"bare json string", `"589 tweet."`, "589 tweet."},
		{"unparseable brace", "{not json", "{not json"},
		{"empty", "  ", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeAnswerText(tc.in); got != tc.want {
				t.Fatalf("sanitizeAnswerText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
