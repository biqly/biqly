package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/ai/jsonextract"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

// describeJoinsResponse reports how many join descriptions were generated and
// persisted by POST /api/ai/semantic/models/{id}/describe-joins.
type describeJoinsResponse struct {
	Updated int `json:"updated"`
}

// joinDescription is one entry of the LLM's JSON output.
type joinDescription struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// DescribeJoins generates a one-sentence business description for every
// active join of a semantic model using the describe-purpose provider (the
// same model that writes metadata descriptions) and persists them on the
// joins. The modeling canvas shows the description in the relationship
// tooltip. Follows the TranslateSemanticModel pattern: model-scoped, auth via
// the standard chain, spend limiter enforced.
func (h *AIHandler) DescribeJoins(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	if h.deps.SemanticRepo == nil || h.deps.AIClient == nil {
		writeError(w, http.StatusServiceUnavailable, "describe service is not configured")
		return
	}

	model, err := h.deps.SemanticRepo.GetFullModel(ctx, id)
	if err != nil {
		writeInternalError(ctx, w, http.StatusNotFound, "model not found", err)
		return
	}
	active := make([]pkgsemantic.Join, 0, len(model.Joins))
	for _, join := range model.Joins {
		if join.IsActive {
			active = append(active, join)
		}
	}
	if len(active) == 0 {
		writeJSON(w, http.StatusOK, describeJoinsResponse{Updated: 0})
		return
	}

	workspaceID := bimw.WorkspaceID(ctx)
	if h.deps.SpendLimiter != nil {
		if err := h.deps.SpendLimiter.Check(ctx, workspaceID); err != nil {
			writeError(w, http.StatusTooManyRequests, "workspace AI token budget exceeded for today")
			return
		}
	}

	gen, err := h.deps.AIClient.Generate(ctx, buildDescribeJoinsPrompt(active))
	if err != nil {
		writeInternalError(ctx, w, http.StatusBadGateway, "failed to describe joins", err, "model_id", id)
		return
	}
	if h.deps.SpendLimiter != nil && gen.Usage != nil {
		h.deps.SpendLimiter.Record(ctx, workspaceID, gen.Usage.Total)
	}

	descriptions, err := parseDescribeJoinsResponse(gen.Content)
	if err != nil {
		writeInternalError(ctx, w, http.StatusBadGateway, "failed to parse join descriptions", err, "model_id", id)
		return
	}

	byID := make(map[string]pkgsemantic.Join, len(active))
	for _, join := range active {
		byID[join.ID] = join
	}
	updated := 0
	for _, d := range descriptions {
		desc := strings.TrimSpace(d.Description)
		if desc == "" {
			continue
		}
		if _, ok := byID[d.ID]; !ok {
			continue
		}
		if err := h.deps.SemanticRepo.UpdateJoinDescription(ctx, id, d.ID, desc); err != nil {
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to store join description", err, "join_id", d.ID)
			return
		}
		updated++
	}
	writeJSON(w, http.StatusOK, describeJoinsResponse{Updated: updated})
}

func buildDescribeJoinsPrompt(joins []pkgsemantic.Join) string {
	var sb strings.Builder
	sb.WriteString("You are a data documentation assistant. For each relationship below, write ONE concise, business-friendly sentence describing what the join semantically links and what analyses it enables.\n\n")
	sb.WriteString("## Rules\n")
	sb.WriteString("- Output ONLY valid JSON matching: {\"joins\": [{\"id\": \"...\", \"description\": \"...\"}]}. No markdown, no explanation.\n")
	sb.WriteString("- Keep each description under 200 characters.\n")
	sb.WriteString("- Mention the linking column and the business meaning (ownership, activity, lookup...), not SQL mechanics.\n\n")
	sb.WriteString("## Relationships\n")
	for _, join := range joins {
		_, _ = fmt.Fprintf(&sb, "- id: %s | %s.%s.%s -> %s.%s.%s | %s\n",
			join.ID,
			join.FromSchema, join.FromTable, join.FromColumn,
			join.ToSchema, join.ToTable, join.ToColumn,
			join.Relationship)
	}
	return sb.String()
}

func parseDescribeJoinsResponse(raw string) ([]joinDescription, error) {
	cleaned := jsonextract.TrimToJSONObject(raw)
	var payload struct {
		Joins []joinDescription `json:"joins"`
	}
	if err := sonic.ConfigStd.Unmarshal([]byte(cleaned), &payload); err != nil {
		return nil, fmt.Errorf("invalid JSON from AI: %w", err)
	}
	return payload.Joins, nil
}
