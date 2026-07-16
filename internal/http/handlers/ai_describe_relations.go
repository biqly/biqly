package handlers

import (
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

	workspaceID := bimw.WorkspaceID(ctx)
	if h.deps.SpendLimiter != nil {
		if err := h.deps.SpendLimiter.Check(ctx, workspaceID); err != nil {
			writeError(w, http.StatusTooManyRequests, "workspace AI token budget exceeded for today")
			return
		}
	}

	gen, err := h.deps.AIClient.Generate(ctx, buildDescribeRelationsPrompt(targets, locale))
	if err != nil {
		writeInternalError(ctx, w, http.StatusBadGateway, "failed to describe relations", err, "datasource_id", id)
		return
	}
	if h.deps.SpendLimiter != nil && gen.Usage != nil {
		h.deps.SpendLimiter.Record(ctx, workspaceID, gen.Usage.Total)
	}

	descriptions, err := parseDescribeRelationsResponse(gen.Content)
	if err != nil {
		writeInternalError(ctx, w, http.StatusBadGateway, "failed to parse relation descriptions", err, "datasource_id", id)
		return
	}

	valid := make(map[string]struct{}, len(targets))
	for _, rel := range targets {
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
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to store relation description", err, "relation_id", d.ID)
			return
		}
		if localized := strings.TrimSpace(d.Localized); localized != "" && locale != i18n.DefaultLocale {
			if err := h.deps.MetaRepo.UpsertTranslation(ctx, metadata.Translation{
				EntityType: metadata.EntityTypeRelation,
				EntityID:   d.ID,
				Lang:       string(locale),
				Field:      metadata.TranslationFieldDescription,
				Value:      localized,
			}); err != nil {
				writeInternalError(ctx, w, http.StatusInternalServerError, "failed to store relation translation", err, "relation_id", d.ID)
				return
			}
		}
		updated++
	}
	writeJSON(w, http.StatusOK, describeRelationsResponse{Updated: updated})
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
