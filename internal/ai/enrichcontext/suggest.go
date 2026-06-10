package enrichcontext

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/ai/jsonextract"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/bytedance/sonic"
)

const maxSuggestGaps = 40

type suggestLLMResponse struct {
	Suggestions []Suggestion `json:"suggestions"`
}

func suggestForGaps(ctx context.Context, client providerpkg.Provider, model *semantic.SemanticModel, gaps []Gap, sampleSummary string) ([]Suggestion, error) {
	if client == nil || len(gaps) == 0 {
		return nil, nil
	}
	applyable := make([]Gap, 0, len(gaps))
	for _, g := range gaps {
		if g.Applyable {
			applyable = append(applyable, g)
		}
	}
	if len(applyable) == 0 {
		return nil, nil
	}
	if len(applyable) > maxSuggestGaps {
		applyable = applyable[:maxSuggestGaps]
	}

	var b strings.Builder
	b.WriteString("You are a data catalog assistant. Propose concise business descriptions in Turkish when the model is Turkish-facing; otherwise English.\n")
	b.WriteString("Return ONLY JSON: {\"suggestions\":[{\"gap_id\":\"...\",\"text\":\"...\"}]}\n")
	if model != nil {
		fmt.Fprintf(&b, "Semantic model: %s\n", model.Name)
		if model.Description != nil && strings.TrimSpace(*model.Description) != "" {
			fmt.Fprintf(&b, "Model description: %s\n", strings.TrimSpace(*model.Description))
		}
	}
	if sampleSummary != "" {
		fmt.Fprintf(&b, "Sample data summary:\n%s\n", sampleSummary)
	}
	b.WriteString("\nGaps:\n")
	for _, g := range applyable {
		fmt.Fprintf(&b, "- id=%s kind=%s summary=%q detail=%q\n", g.ID, g.Kind, g.Summary, g.Detail)
	}

	start := time.Now()
	gen, err := client.Generate(ctx, b.String())
	observability.Default().RecordEnrichContextSuggestLatency(time.Since(start))
	if err != nil {
		return nil, fmt.Errorf("ai suggest: %w", err)
	}
	raw := jsonextract.TrimToJSONObject(gen.Content)
	var parsed suggestLLMResponse
	if err := sonic.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("ai suggest parse: %w", err)
	}
	allowed := make(map[string]struct{}, len(applyable))
	for _, g := range applyable {
		allowed[g.ID] = struct{}{}
	}
	out := make([]Suggestion, 0, len(parsed.Suggestions))
	for _, s := range parsed.Suggestions {
		if _, ok := allowed[s.GapID]; !ok {
			continue
		}
		if strings.TrimSpace(s.Text) == "" {
			continue
		}
		out = append(out, Suggestion{GapID: s.GapID, Text: strings.TrimSpace(s.Text)})
	}
	observability.Default().RecordEnrichContextSuggestionsGenerated(len(out))
	return out, nil
}
