package handlers

import (
	"net/http"
	"strings"
	"unicode"

	"github.com/biqly/biqly/internal/ai"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/query"
)

// draftSavedQueryRequest is the body for POST /api/ai/skills/draft: an
// AI-assisted authoring step that turns a natural-language description into a
// draft Saved Query the user reviews before saving. It reuses the same
// text-to-SQL generation path as the normal AI query — only the persisted
// execution is skipped.
type draftSavedQueryRequest struct {
	DatasourceID string `json:"datasource_id"`
	ModelID      string `json:"model_id,omitempty"`
	Question     string `json:"question"`
}

// draftSavedQueryResponse is a NON-persisted Saved Query draft. When the
// generator produces a LogicalQuery it carries the suggested name/description
// plus the query itself; otherwise NeedsClarification/Error explains why so the
// client can surface it inline without fabricating a query.
type draftSavedQueryResponse struct {
	Name               string              `json:"name,omitempty"`
	Description        string              `json:"description,omitempty"`
	Question           string              `json:"question"`
	LogicalQuery       *query.LogicalQuery `json:"logical_query,omitempty"`
	Parameters         []SkillParameter    `json:"parameters"`
	NeedsClarification bool                `json:"needs_clarification,omitempty"`
	Message            string              `json:"message,omitempty"`
	Error              string              `json:"error,omitempty"`
}

// DraftSavedQuery drafts a Saved Query from a natural-language question by
// running the existing NL→LogicalQuery generation (h.service.ProcessQuestion via
// processAIQuestion) and returning the generated LogicalQuery plus a suggested
// name/description. Nothing is persisted — the user reviews and saves through
// the normal skills Create endpoint. Auth, per-datasource access, and the spend
// limiter are enforced exactly like the normal AI query path.
func (h *AIHandler) DraftSavedQuery(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[draftSavedQueryRequest](w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	question := strings.TrimSpace(req.Question)

	// routeAIQueryRequest validates required fields, loads the semantic model /
	// table routing, and (if needed) writes a table-scope clarification itself.
	routed, pc, model, routeResult, ok := h.routeAIQueryRequest(ctx, w, aiQueryRequest{
		DatasourceID: req.DatasourceID,
		ModelID:      req.ModelID,
		Question:     question,
	})
	if !ok {
		return
	}

	workspaceID := bimw.WorkspaceID(ctx)
	if h.deps.SpendLimiter != nil {
		if err := h.deps.SpendLimiter.Check(ctx, workspaceID); err != nil {
			writeError(w, http.StatusTooManyRequests, "workspace AI token budget exceeded for today")
			return
		}
	}

	resp, err := h.processAIQuestion(ctx, pc, routed, model, routeResult)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to draft saved query", err,
			"question", question,
			"model_id", req.ModelID,
			"datasource_id", req.DatasourceID,
		)
		return
	}
	if h.deps.SpendLimiter != nil && resp != nil && resp.Metadata != nil && resp.Metadata.TokenUsage != nil {
		h.deps.SpendLimiter.Record(ctx, workspaceID, resp.Metadata.TokenUsage.Total)
	}

	draft, status := buildSavedQueryDraft(question, resp)
	writeJSON(w, status, draft)
}

// buildSavedQueryDraft maps a generation response to a draft payload without
// fabricating a query: clarification and empty-query cases return an explanatory
// message/error instead of a LogicalQuery. Kept pure so the branch logic is
// unit-testable without driving the LLM.
func buildSavedQueryDraft(question string, resp *ai.Response) (draftSavedQueryResponse, int) {
	if resp != nil && resp.Clarification != nil && resp.Clarification.NeedsClarification {
		msg := strings.TrimSpace(resp.Clarification.ClarificationQuestion)
		if msg == "" {
			msg = "the question needs clarification before a query can be drafted"
		}
		return draftSavedQueryResponse{
			Question:           question,
			Parameters:         []SkillParameter{},
			NeedsClarification: true,
			Message:            msg,
		}, http.StatusOK
	}

	var lq *query.LogicalQuery
	if resp != nil && resp.Result != nil {
		lq = resp.Result.LogicalQuery
	}
	if lq == nil {
		msg := "could not draft a query for this question; try rephrasing it"
		if resp != nil && resp.Result != nil && len(resp.Result.Warnings) > 0 {
			if warn := strings.TrimSpace(resp.Result.Warnings[0]); warn != "" {
				msg = warn
			}
		}
		return draftSavedQueryResponse{
			Question:   question,
			Parameters: []SkillParameter{},
			Error:      msg,
		}, http.StatusUnprocessableEntity
	}

	return draftSavedQueryResponse{
		Name:         suggestSavedQueryName(question),
		Description:  question,
		Question:     question,
		LogicalQuery: lq,
		// SP2 does not auto-detect parameters; the user adds slots in the modal.
		Parameters: []SkillParameter{},
	}, http.StatusOK
}

// draftNameMaxWords / draftNameMaxRunes bound the suggested title so it stays a
// short label rather than a full sentence.
const (
	draftNameMaxWords = 10
	draftNameMaxRunes = 80
)

// suggestSavedQueryName derives a short, deterministic title from the question:
// the first few words, trailing punctuation stripped, first letter capitalized.
// No extra LLM call — the user can rename in the modal.
func suggestSavedQueryName(question string) string {
	trimmed := strings.TrimSpace(question)
	if trimmed == "" {
		return ""
	}
	words := strings.Fields(trimmed)
	if len(words) > draftNameMaxWords {
		words = words[:draftNameMaxWords]
	}
	name := strings.Join(words, " ")
	name = strings.TrimRight(name, "?.!,;:")
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	runes := []rune(name)
	runes[0] = unicode.ToUpper(runes[0])
	if len(runes) > draftNameMaxRunes {
		runes = runes[:draftNameMaxRunes]
		runes = []rune(strings.TrimSpace(string(runes)))
	}
	return string(runes)
}
