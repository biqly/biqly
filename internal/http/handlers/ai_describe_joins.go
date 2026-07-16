package handlers

import (
	"context"
	"errors"
	"fmt"
	"io"
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

// describeJoinsRequest optionally narrows which joins get described. An empty
// body describes every active join (backward compatible). join_ids restricts
// to specific joins (per-row / auto-describe-on-create); only_missing skips
// joins that already carry a description (bulk "fill in the gaps").
type describeJoinsRequest struct {
	JoinIDs     []string `json:"join_ids"`
	OnlyMissing bool     `json:"only_missing"`
}

// joinDescription is one entry of the LLM's JSON output.
type joinDescription struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// selectJoins filters a model's joins to the active ones that match the
// request's join_ids / only_missing constraints.
func selectJoins(joins []pkgsemantic.Join, req describeJoinsRequest) []pkgsemantic.Join {
	wanted := make(map[string]struct{}, len(req.JoinIDs))
	for _, id := range req.JoinIDs {
		wanted[id] = struct{}{}
	}
	out := make([]pkgsemantic.Join, 0, len(joins))
	for _, join := range joins {
		if !join.IsActive {
			continue
		}
		if len(wanted) > 0 {
			if _, ok := wanted[join.ID]; !ok {
				continue
			}
		}
		if req.OnlyMissing && strings.TrimSpace(join.Description) != "" {
			continue
		}
		out = append(out, join)
	}
	return out
}

// decodeDescribeJoinsRequest reads the optional filter body; a missing or
// empty body is valid and means "all active joins".
func decodeDescribeJoinsRequest(r *http.Request) (describeJoinsRequest, error) {
	var req describeJoinsRequest
	if r.Body == nil || r.ContentLength == 0 {
		return req, nil
	}
	if err := sonic.ConfigStd.NewDecoder(r.Body).Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			return req, nil
		}
		return req, err
	}
	return req, nil
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
	req, err := decodeDescribeJoinsRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	model, err := h.deps.SemanticRepo.GetFullModel(ctx, id)
	if err != nil {
		writeInternalError(ctx, w, http.StatusNotFound, "model not found", err)
		return
	}
	targets := selectJoins(model.Joins, req)
	joinModelID := make(map[string]string, len(targets))
	for _, join := range targets {
		joinModelID[join.ID] = id
	}

	updated, status, err := h.describeAndPersistJoins(ctx, targets, joinModelID)
	if err != nil {
		writeInternalError(ctx, w, status, "failed to describe joins", err, "model_id", id)
		return
	}
	writeJSON(w, http.StatusOK, describeJoinsResponse{Updated: updated})
}

// describeAndPersistJoins runs the describe LLM over the given joins and stores
// each returned description on its owning model. Returns the count updated and,
// on error, the HTTP status to surface. A no-op (no targets) returns (0, 200, nil).
func (h *AIHandler) describeAndPersistJoins(
	ctx context.Context,
	targets []pkgsemantic.Join,
	joinModelID map[string]string,
) (int, int, error) {
	if len(targets) == 0 {
		return 0, http.StatusOK, nil
	}

	workspaceID := bimw.WorkspaceID(ctx)
	if h.deps.SpendLimiter != nil {
		if err := h.deps.SpendLimiter.Check(ctx, workspaceID); err != nil {
			return 0, http.StatusTooManyRequests, err
		}
	}

	gen, err := h.deps.AIClient.Generate(ctx, buildDescribeJoinsPrompt(targets))
	if err != nil {
		return 0, http.StatusBadGateway, err
	}
	if h.deps.SpendLimiter != nil && gen.Usage != nil {
		h.deps.SpendLimiter.Record(ctx, workspaceID, gen.Usage.Total)
	}

	descriptions, err := parseDescribeJoinsResponse(gen.Content)
	if err != nil {
		return 0, http.StatusBadGateway, err
	}

	updated := 0
	for _, d := range descriptions {
		desc := strings.TrimSpace(d.Description)
		modelID, ok := joinModelID[d.ID]
		if desc == "" || !ok {
			continue
		}
		if err := h.deps.SemanticRepo.UpdateJoinDescription(ctx, modelID, d.ID, desc); err != nil {
			return updated, http.StatusInternalServerError, err
		}
		updated++
	}
	return updated, http.StatusOK, nil
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
