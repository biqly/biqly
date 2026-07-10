package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/ai/jsonextract"
	bimw "github.com/biqly/biqly/internal/http/middleware"
)

// suggestTitleRequest is the body for POST /api/ai/conversations/suggest-title:
// a lightweight LLM call that names a chat thread from its first exchange, so
// the sidebar shows a meaningful title instead of the truncated question.
type suggestTitleRequest struct {
	Question string `json:"question"`
	Answer   string `json:"answer,omitempty"`
}

type suggestTitleResponse struct {
	Title string `json:"title"`
}

const maxSuggestedTitleRunes = 60

// SuggestConversationTitle asks the describe-purpose provider for a short
// conversation title in the question's own language. Nothing is persisted —
// the client renames the conversation through the normal snapshot save.
func (h *AIHandler) SuggestConversationTitle(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[suggestTitleRequest](w, r)
	if !ok {
		return
	}
	question := strings.TrimSpace(req.Question)
	if question == "" {
		writeError(w, http.StatusBadRequest, "question is required")
		return
	}
	ctx := r.Context()
	if h.deps.AIClient == nil {
		writeError(w, http.StatusServiceUnavailable, "describe service is not configured")
		return
	}

	workspaceID := bimw.WorkspaceID(ctx)
	if h.deps.SpendLimiter != nil {
		if err := h.deps.SpendLimiter.Check(ctx, workspaceID); err != nil {
			writeError(w, http.StatusTooManyRequests, "workspace AI token budget exceeded for today")
			return
		}
	}

	gen, err := h.deps.AIClient.Generate(ctx, buildSuggestTitlePrompt(question, req.Answer))
	if err != nil {
		writeInternalError(ctx, w, http.StatusBadGateway, "failed to suggest title", err)
		return
	}
	if h.deps.SpendLimiter != nil && gen.Usage != nil {
		h.deps.SpendLimiter.Record(ctx, workspaceID, gen.Usage.Total)
	}

	title := parseSuggestedTitle(gen.Content)
	if title == "" {
		writeError(w, http.StatusUnprocessableEntity, "no title produced")
		return
	}
	writeJSON(w, http.StatusOK, suggestTitleResponse{Title: title})
}

func buildSuggestTitlePrompt(question, answer string) string {
	var sb strings.Builder
	sb.WriteString("Name this analytics chat thread. Output ONLY valid JSON: {\"title\": \"...\"}.\n")
	sb.WriteString("Rules: 3-6 words, no quotes/emoji/trailing punctuation, SAME LANGUAGE as the question, describe the analytical topic (not the answer value).\n\n")
	_, _ = fmt.Fprintf(&sb, "Question: %s\n", question)
	if trimmed := strings.TrimSpace(answer); trimmed != "" {
		const maxAnswerRunes = 300
		if utf8.RuneCountInString(trimmed) > maxAnswerRunes {
			trimmed = string([]rune(trimmed)[:maxAnswerRunes])
		}
		_, _ = fmt.Fprintf(&sb, "Answer: %s\n", trimmed)
	}
	return sb.String()
}

func parseSuggestedTitle(raw string) string {
	cleaned := jsonextract.TrimToJSONObject(raw)
	var payload struct {
		Title string `json:"title"`
	}
	if err := sonic.ConfigStd.Unmarshal([]byte(cleaned), &payload); err != nil {
		return ""
	}
	title := strings.TrimSpace(strings.Trim(payload.Title, `"'“”`))
	if title == "" {
		return ""
	}
	if utf8.RuneCountInString(title) > maxSuggestedTitleRunes {
		title = strings.TrimSpace(string([]rune(title)[:maxSuggestedTitleRunes]))
	}
	return title
}
