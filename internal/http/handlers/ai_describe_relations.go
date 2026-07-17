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
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
)

// describeRelationsRequest narrows which of a datasource's FK relations get
// AI-described. An empty body describes every relation without a description.
// relation_ids restricts to specific relations (per-row describe); when
// skip_existing is true relations that already carry a description are left
// untouched (bulk "fill in the gaps"). locale selects the additional localized
// output; the English description is always written to relations.description
// and the localized value overlays via entity_translations.
type describeRelationsRequest struct {
	RelationIDs  []string `json:"relation_ids"`
	SkipExisting bool     `json:"skip_existing"`
	Locale       string   `json:"locale"`
}

type describeRelationsResponse struct {
	Updated int `json:"updated"`
}

// relationDescription is one entry of the LLM's JSON output. Localized carries
// the requested-locale sentence and is empty when the run locale is English.
type relationDescription struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Localized   string `json:"localized"`
}

func decodeDescribeRelationsRequest(r *http.Request) (describeRelationsRequest, error) {
	var req describeRelationsRequest
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

// selectRelations filters a datasource's relations to those matching the
// request's relation_ids / skip_existing constraints.
func selectRelations(rels []pkgmetadata.RelationDetail, req describeRelationsRequest) []pkgmetadata.RelationDetail {
	wanted := make(map[string]struct{}, len(req.RelationIDs))
	for _, id := range req.RelationIDs {
		wanted[id] = struct{}{}
	}
	out := make([]pkgmetadata.RelationDetail, 0, len(rels))
	for _, rel := range rels {
		if len(wanted) > 0 {
			if _, ok := wanted[rel.ID]; !ok {
				continue
			}
		}
		if req.SkipExisting && strings.TrimSpace(rel.Description) != "" {
			continue
		}
		out = append(out, rel)
	}
	return out
}

// DescribeRelations generates business descriptions for a datasource's FK
// relations (metadata-level — no semantic model required) and persists them on
// the relations themselves: English into relations.description and, when the
// run locale isn't English, the localized sentence into entity_translations.
// Powers the Metadata Relationships panel's per-row describe and the AI
// metadata generator's relations scope.
func (h *AIHandler) DescribeRelations(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	if h.deps.MetaRepo == nil || h.deps.AIClient == nil {
		writeError(w, http.StatusServiceUnavailable, "describe service is not configured")
		return
	}
	req, err := decodeDescribeRelationsRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	locale := i18n.ParseLocale(req.Locale)
	if req.Locale == "" {
		locale = i18n.FromContext(ctx)
	}

	all, err := h.deps.MetaRepo.ListRelationDetails(ctx, id)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list relations", err, "datasource_id", id)
		return
	}
	targets := selectRelations(all, req)
	if len(targets) == 0 {
		writeJSON(w, http.StatusOK, describeRelationsResponse{Updated: 0})
		return
	}

	updated, err := h.describeRelationsCore(ctx, targets, locale, nil)
	if err != nil {
		writeInternalError(ctx, w, http.StatusBadGateway, "failed to describe relations", err,
			"datasource_id", id, "updated_before_failure", updated)
		return
	}
	writeJSON(w, http.StatusOK, describeRelationsResponse{Updated: updated})
}

// describeRelationsChunkSize bounds how many relations go into one LLM prompt;
// small enough that a chunk completes well inside client/gateway timeouts even
// on slow models.
const describeRelationsChunkSize = 8

// describeRelationsCore runs the chunked describe loop: one LLM prompt per
// chunk, persisting after each so completed chunks survive an interruption (a
// single prompt for dozens of relations takes minutes on slow models, and a
// canceled request would otherwise lose everything). report, when set, gets a
// progress update after every chunk — used by the describe_relations job so
// the AI jobs strip shows live progress.
func (h *AIHandler) describeRelationsCore(
	ctx context.Context,
	targets []pkgmetadata.RelationDetail,
	locale i18n.Locale,
	report AIJobProgressFunc,
) (int, error) {
	workspaceID := bimw.WorkspaceID(ctx)
	if h.deps.SpendLimiter != nil {
		if err := h.deps.SpendLimiter.Check(ctx, workspaceID); err != nil {
			return 0, err
		}
	}

	updated := 0
	for start := 0; start < len(targets); start += describeRelationsChunkSize {
		if err := ctx.Err(); err != nil {
			// Caller gone / job cancelled; already-persisted chunks are kept.
			return updated, err
		}
		chunk := targets[start:min(start+describeRelationsChunkSize, len(targets))]
		if report != nil {
			report(AIJobProgress{
				Phase:    "generating",
				Message:  fmt.Sprintf("describing relations %d–%d of %d", start+1, start+len(chunk), len(targets)),
				Progress: 10 + 85*start/len(targets),
				Status:   metadata.AIJobStatusRunning,
			})
		}

		gen, err := h.deps.AIClient.Generate(ctx, buildDescribeRelationsPrompt(chunk, locale))
		if err != nil {
			return updated, fmt.Errorf("describe relations chunk: %w", err)
		}
		if h.deps.SpendLimiter != nil && gen.Usage != nil {
			h.deps.SpendLimiter.Record(ctx, workspaceID, gen.Usage.Total)
		}

		descriptions, err := parseDescribeRelationsResponse(gen.Content)
		if err != nil {
			return updated, err
		}

		n, err := h.persistRelationDescriptions(ctx, chunk, descriptions, locale)
		updated += n
		if err != nil {
			return updated, fmt.Errorf("store relation description: %w", err)
		}
	}
	return updated, nil
}

// describeRelationsJobRequest is the payload of the async describe_relations
// job kind — the datasource-scoped variant of describeRelationsRequest.
type describeRelationsJobRequest struct {
	DatasourceID string   `json:"datasource_id"`
	RelationIDs  []string `json:"relation_ids"`
	SkipExisting bool     `json:"skip_existing"`
	Locale       string   `json:"locale"`
}

type describeRelationsJobResult struct {
	Updated int `json:"updated"`
	Total   int `json:"total"`
}

// executeDescribeRelationsJob is the async job executor for bulk relation
// describes. Running through the jobs pipeline decouples the work from the
// browser (the direct HTTP path died with "context canceled" whenever the user
// navigated or refreshed while waiting) and surfaces progress in the AI jobs
// strip like table describes.
func (h *AIHandler) executeDescribeRelationsJob(
	ctx context.Context,
	req describeRelationsJobRequest,
	report AIJobProgressFunc,
) (*describeRelationsJobResult, error) {
	if h.deps.MetaRepo == nil || h.deps.AIClient == nil {
		return nil, errors.New("describe service is not configured")
	}
	locale := i18n.ParseLocale(req.Locale)
	if req.Locale == "" {
		locale = i18n.FromContext(ctx)
	}

	all, err := h.deps.MetaRepo.ListRelationDetails(ctx, req.DatasourceID)
	if err != nil {
		return nil, fmt.Errorf("list relations: %w", err)
	}
	targets := selectRelations(all, describeRelationsRequest{
		RelationIDs:  req.RelationIDs,
		SkipExisting: req.SkipExisting,
	})
	if report != nil {
		report(AIJobProgress{Phase: "generating", Message: fmt.Sprintf("describing %d relations", len(targets)), Progress: 10, Status: metadata.AIJobStatusRunning})
	}
	updated, err := h.describeRelationsCore(ctx, targets, locale, report)
	if err != nil {
		return nil, err
	}
	if report != nil {
		report(AIJobProgress{Phase: "applying", Message: "descriptions saved", Progress: 98, Status: metadata.AIJobStatusRunning})
	}
	return &describeRelationsJobResult{Updated: updated, Total: len(targets)}, nil
}

// persistRelationDescriptions stores the parsed descriptions for the chunk's
// relations (English on the row, localized as a translation) and returns how
// many were written.
func (h *AIHandler) persistRelationDescriptions(
	ctx context.Context,
	chunk []pkgmetadata.RelationDetail,
	descriptions []relationDescription,
	locale i18n.Locale,
) (int, error) {
	valid := make(map[string]struct{}, len(chunk))
	for _, rel := range chunk {
		valid[rel.ID] = struct{}{}
	}
	updated := 0
	for _, d := range descriptions {
		desc := strings.TrimSpace(d.Description)
		if desc == "" {
			continue
		}
		if _, ok := valid[d.ID]; !ok {
			continue
		}
		if err := h.deps.MetaRepo.UpdateRelationDescription(ctx, d.ID, desc); err != nil {
			return updated, err
		}
		if localized := strings.TrimSpace(d.Localized); localized != "" && locale != i18n.DefaultLocale {
			if err := h.deps.MetaRepo.UpsertTranslation(ctx, metadata.Translation{
				EntityType: metadata.EntityTypeRelation,
				EntityID:   d.ID,
				Lang:       string(locale),
				Field:      metadata.TranslationFieldDescription,
				Value:      localized,
			}); err != nil {
				return updated, err
			}
		}
		updated++
	}
	return updated, nil
}

func buildDescribeRelationsPrompt(rels []pkgmetadata.RelationDetail, locale i18n.Locale) string {
	var sb strings.Builder
	sb.WriteString("You are a data documentation assistant. For each foreign-key relationship below, write ONE concise, business-friendly sentence describing what the relationship semantically links and what analyses it enables.\n\n")
	sb.WriteString("## Rules\n")
	if locale != i18n.DefaultLocale {
		fmt.Fprintf(&sb, "- Output ONLY valid JSON matching: {\"relations\": [{\"id\": \"...\", \"description\": \"<English sentence>\", \"localized\": \"<the same sentence in %s>\"}]}. No markdown, no explanation.\n", locale)
	} else {
		sb.WriteString("- Output ONLY valid JSON matching: {\"relations\": [{\"id\": \"...\", \"description\": \"<English sentence>\"}]}. No markdown, no explanation.\n")
	}
	sb.WriteString("- Keep each sentence under 200 characters.\n")
	sb.WriteString("- Mention the linking column and the business meaning (ownership, activity, lookup...), not SQL mechanics.\n\n")
	sb.WriteString("## Relationships\n")
	for _, rel := range rels {
		fmt.Fprintf(&sb, "- id: %s | %s.%s.%s -> %s.%s.%s | %s\n",
			rel.ID,
			rel.FromSchema, rel.FromTable, rel.FromColumn,
			rel.ToSchema, rel.ToTable, rel.ToColumn,
			rel.RelationshipType)
	}
	return sb.String()
}

func parseDescribeRelationsResponse(raw string) ([]relationDescription, error) {
	cleaned := jsonextract.TrimToJSONObject(raw)
	var payload struct {
		Relations []relationDescription `json:"relations"`
	}
	if err := sonic.ConfigStd.Unmarshal([]byte(cleaned), &payload); err != nil {
		return nil, fmt.Errorf("invalid JSON from AI: %w", err)
	}
	return payload.Relations, nil
}
