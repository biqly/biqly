package ai

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	promptpkg "github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/query"
)

const (
	// answerMaxResultRows caps how many result rows are rendered into the answer
	// prompt so the completion stays cheap and small.
	answerMaxResultRows = 20
	// answerMaxCellRunes truncates individual cell values in the prompt rendering.
	answerMaxCellRunes = 80
	// answerPromptMaxRunes bounds the whole synthesized prompt.
	answerPromptMaxRunes = 4000
)

// AnswerEnabled reports whether post-execution natural-language answer synthesis
// is turned on (BI_AI_ANSWER_ENABLED). Callers can use it to skip the extra work
// (e.g. a spend-limit check) when the feature is disabled.
func (s *Service) AnswerEnabled() bool {
	return s.answerEnabled
}

// SynthesizeAnswer produces a 1-2 sentence natural-language answer to the user's
// question, stating the key number(s) from the executed result, in the user's
// locale. It is a separate, lightweight, post-result LLM call — it never touches
// or mutates the fingerprinted LogicalQuery JSON. It returns "" (never an error)
// when the feature is disabled, there is no result to summarize, or the provider
// call fails, so answer synthesis can never fail the underlying query.
func (s *Service) SynthesizeAnswer(ctx context.Context, question, locale string, result *query.Result) string {
	if !s.answerEnabled || result == nil || len(result.Rows) == 0 {
		return ""
	}
	prompt := buildAnswerPrompt(question, locale, result)
	if prompt == "" {
		return ""
	}
	gen, err := s.client.Generate(ctx, prompt)
	if err != nil {
		slog.DebugContext(ctx, "answer synthesis failed", "error", err)
		return ""
	}
	return strings.TrimSpace(gen.Content)
}

// buildAnswerPrompt renders a compact, bounded prompt: the question, the target
// locale, a small rendering of the result (column names + up to N rows), and
// strict instructions to answer in plain text without inventing data.
func buildAnswerPrompt(question, locale string, result *query.Result) string {
	if locale == "" {
		locale = "the user's language"
	}
	var sb strings.Builder
	sb.Grow(1024)
	sb.WriteString("You are a Business Intelligence assistant. Answer the user's question in ")
	sb.WriteString(locale)
	sb.WriteString(" using ONLY the query result below.\n\n")
	sb.WriteString("## Question\n")
	sb.WriteString(question)
	sb.WriteString("\n\n## Query Result\n")
	sb.WriteString(renderResultForAnswer(result))
	sb.WriteString("\n## Instructions\n")
	sb.WriteString("- Reply with 1-2 short sentences in plain text (no markdown, no tables, no SQL, no code fences).\n")
	sb.WriteString("- State the key number(s) from the result directly; you may add at most one brief observation.\n")
	sb.WriteString("- Use ONLY the data shown above. Do NOT invent, estimate, or extrapolate any values.\n")
	sb.WriteString("- Write the answer in ")
	sb.WriteString(locale)
	sb.WriteString(".\n")

	return promptpkg.TruncateRunes(sb.String(), answerPromptMaxRunes)
}

// renderResultForAnswer produces a compact textual rendering of the result:
// column names, up to answerMaxResultRows pipe-delimited rows, and the total count.
func renderResultForAnswer(result *query.Result) string {
	var sb strings.Builder
	names := make([]string, 0, len(result.Columns))
	for _, c := range result.Columns {
		names = append(names, c.Name)
	}
	sb.WriteString("Columns: ")
	sb.WriteString(strings.Join(names, " | "))
	sb.WriteByte('\n')

	limit := min(len(result.Rows), answerMaxResultRows)
	for i := range limit {
		row := result.Rows[i]
		cells := make([]string, 0, len(row))
		for _, v := range row {
			cells = append(cells, promptpkg.TruncateRunes(formatAnswerCell(v), answerMaxCellRunes))
		}
		sb.WriteString(strings.Join(cells, " | "))
		sb.WriteByte('\n')
	}
	if total := max(result.Stats.RowCount, len(result.Rows)); total > limit {
		fmt.Fprintf(&sb, "(showing %d of %d rows)\n", limit, total)
	}
	return sb.String()
}

func formatAnswerCell(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
