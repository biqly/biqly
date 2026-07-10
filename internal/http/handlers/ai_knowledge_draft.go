package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
)

// draftKnowledgeRequest is the body for POST /api/ai/knowledge/draft: an
// AI-assisted authoring step ("write me a nice md on how this calculation
// works") that produces a reviewable markdown draft for the given folder. It
// uses the same describe-purpose provider as metadata description generation.
type draftKnowledgeRequest struct {
	DatasourceID string `json:"datasource_id"`
	Folder       string `json:"folder"`
	Prompt       string `json:"prompt"`
}

type draftKnowledgeResponse struct {
	Path      string `json:"path"`
	ContentMD string `json:"content_md"`
}

// DraftKnowledgeFile generates a markdown draft (with folder-appropriate YAML
// frontmatter) from a natural-language request. Nothing is persisted — the
// user reviews and saves through the normal knowledge Create endpoint.
func (h *AIKnowledgeHandler) DraftKnowledgeFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, ok := decodeJSON[draftKnowledgeRequest](w, r)
	if !ok {
		return
	}
	if req.DatasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}
	folder := strings.TrimSpace(req.Folder)

	workspaceID := bimw.WorkspaceID(ctx)
	if h.deps.SpendLimiter != nil {
		if err := h.deps.SpendLimiter.Check(ctx, workspaceID); err != nil {
			writeError(w, http.StatusTooManyRequests, "workspace AI token budget exceeded for today")
			return
		}
	}

	gen, err := h.deps.AIClient.Generate(ctx, buildKnowledgeDraftPrompt(folder, prompt))
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to draft knowledge file", err,
			"folder", folder, "datasource_id", req.DatasourceID)
		return
	}
	if h.deps.SpendLimiter != nil && gen.Usage != nil {
		h.deps.SpendLimiter.Record(ctx, workspaceID, gen.Usage.Total)
	}

	content := stripMarkdownFence(gen.Content)
	fm, body := parseKnowledgeMarkdown(content)
	title := knowledgeTitle(fm, body, "draft.md")
	writeJSON(w, http.StatusOK, draftKnowledgeResponse{
		Path:      suggestKnowledgePath(folder, title),
		ContentMD: content,
	})
}

func suggestKnowledgePath(folder, title string) string {
	slug := knowledgeSlug(title)
	if folder == "" {
		return slug + ".md"
	}
	return folder + "/" + slug + ".md"
}

// stripMarkdownFence unwraps a full response wrapped in a single fenced code
// block (```markdown … ```), which chat models often add around file content.
func stripMarkdownFence(content string) string {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	firstNewline := strings.Index(trimmed, "\n")
	if firstNewline < 0 || !strings.HasSuffix(trimmed, "```") {
		return trimmed
	}
	inner := trimmed[firstNewline+1 : len(trimmed)-len("```")]
	return strings.TrimSpace(inner)
}

func buildKnowledgeDraftPrompt(folder, userPrompt string) string {
	var sb strings.Builder
	sb.WriteString("You are a data-team documentation assistant writing one markdown file for an AI knowledge base. ")
	sb.WriteString("The file grounds a text-to-SQL agent: it must be precise, self-contained, and business-friendly.\n\n")
	sb.WriteString("## Rules\n")
	sb.WriteString("- Output ONLY the markdown file content. No surrounding explanation, no code fence around the whole file.\n")
	sb.WriteString("- Start with a YAML frontmatter block delimited by `---` lines.\n")
	sb.WriteString("- Always include a `description:` field in the frontmatter — one sentence telling the agent when to use this file.\n")
	sb.WriteString("- Write the body in the same language as the user's request.\n")

	switch folder {
	case knowledgeFolderInstructions:
		sb.WriteString("- This is an instructions/ file: a rule the agent must follow when generating SQL. Frontmatter: `type: instruction`, `title:`. Body: the rule, with a short `## Usage notes` section.\n")
	case knowledgeFolderGlossary:
		sb.WriteString("- This is a glossary/ file: one canonical business term. Frontmatter: `type: glossary`, `term:`, `aliases: [..]`, optional `maps_to_type:`/`maps_to_name:`. Body: the definition and `## Usage notes`.\n")
	case knowledgeFolderSQLPairs:
		sb.WriteString("- This is a sql-pairs/ file: one worked question-to-SQL example. Frontmatter: `type: sql-pair`, `question:` (the natural-language question). Body: a short explanation and the SQL in a ```sql fenced block.\n")
	case knowledgeFolderMetrics:
		sb.WriteString("- This is a metrics/ file: one metric definition. Frontmatter: `type: metric`, `title:`. Body: definition, unit, grain, the calculation steps, and `## Usage notes`.\n")
	default:
		sb.WriteString("- Frontmatter: `title:` plus any fields that help routing. Body: a clear explanation with headings.\n")
	}

	_, _ = fmt.Fprintf(&sb, "\n## User request\n%s\n", userPrompt)
	return sb.String()
}

// Backfill seeds an empty knowledge base from the existing structured records
// (instructions, glossary terms, sql-pair examples) so teams don't start from
// a blank tree. It refuses to run when files already exist.
func (h *AIKnowledgeHandler) Backfill(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, ok := decodeJSON[struct {
		DatasourceID string `json:"datasource_id"`
	}](w, r)
	if !ok {
		return
	}
	if req.DatasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return
	}
	existing, err := h.deps.MetaRepo.ListKnowledgeFiles(ctx, req.DatasourceID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to check knowledge files", err)
		return
	}
	if len(existing) > 0 {
		writeError(w, http.StatusConflict, "knowledge base already has files")
		return
	}
	createdBy := bimw.UserID(ctx)
	usedPaths := map[string]bool{}
	created := 0

	created += h.backfillInstructions(ctx, req.DatasourceID, createdBy, usedPaths)
	created += h.backfillGlossary(ctx, req.DatasourceID, createdBy, usedPaths)
	created += h.backfillExamples(ctx, req.DatasourceID, createdBy, usedPaths)

	writeJSON(w, http.StatusOK, map[string]int{"created": created})
}

func uniqueKnowledgePath(used map[string]bool, folder, title string) string {
	base := suggestKnowledgePath(folder, title)
	path := base
	for i := 2; used[path]; i++ {
		path = strings.TrimSuffix(base, ".md") + fmt.Sprintf("-%d.md", i)
	}
	used[path] = true
	return path
}

func (h *AIKnowledgeHandler) backfillInstructions(ctx context.Context, datasourceID, createdBy string, used map[string]bool) int {
	rows, err := h.deps.MetaRepo.ListInstructions(ctx, datasourceID)
	if err != nil {
		return 0
	}
	created := 0
	for _, row := range rows {
		if !row.IsActive {
			continue
		}
		content := fmt.Sprintf("---\ntype: instruction\ntitle: %q\ndescription: %q\n---\n\n# %s\n\n%s\n",
			row.Title, row.Title, row.Title, row.BodyMD)
		fm, body := parseKnowledgeMarkdown(content)
		fileID, err := h.deps.MetaRepo.InsertKnowledgeFile(ctx, metadata.KnowledgeFileInsert{
			DatasourceID: datasourceID,
			Path:         uniqueKnowledgePath(used, knowledgeFolderInstructions, row.Title),
			Title:        knowledgeTitle(fm, body, row.Title+".md"),
			ContentMD:    content,
			Frontmatter:  marshalKnowledgeFrontmatter(fm),
			Status:       "published",
			CreatedBy:    createdBy,
		})
		if err != nil {
			continue
		}
		_ = h.deps.MetaRepo.LinkInstructionKnowledgeFile(ctx, row.ID, fileID)
		created++
	}
	return created
}

func (h *AIKnowledgeHandler) backfillGlossary(ctx context.Context, datasourceID, createdBy string, used map[string]bool) int {
	rows, err := h.deps.MetaRepo.ListBusinessGlossary(ctx, datasourceID, "")
	if err != nil {
		return 0
	}
	created := 0
	for _, row := range rows {
		var fmAliases string
		if len(row.Aliases) > 0 {
			fmAliases = fmt.Sprintf("aliases: [%q]\n", strings.Join(row.Aliases, `", "`))
		}
		content := fmt.Sprintf("---\ntype: glossary\nterm: %q\n%smaps_to_type: %q\nmaps_to_name: %q\ndescription: %q\n---\n\n# %s\n\n%s\n",
			row.Term, fmAliases, row.MapsToType, row.MapsToName, row.Definition, row.Term, row.Definition)
		fm, body := parseKnowledgeMarkdown(content)
		fileID, err := h.deps.MetaRepo.InsertKnowledgeFile(ctx, metadata.KnowledgeFileInsert{
			DatasourceID: datasourceID,
			Path:         uniqueKnowledgePath(used, knowledgeFolderGlossary, row.Term),
			Title:        knowledgeTitle(fm, body, row.Term+".md"),
			ContentMD:    content,
			Frontmatter:  marshalKnowledgeFrontmatter(fm),
			Status:       "published",
			CreatedBy:    createdBy,
		})
		if err != nil {
			continue
		}
		_ = h.deps.MetaRepo.LinkGlossaryKnowledgeFile(ctx, row.ID, fileID)
		created++
	}
	return created
}

func (h *AIKnowledgeHandler) backfillExamples(ctx context.Context, datasourceID, createdBy string, used map[string]bool) int {
	rows, err := h.deps.MetaRepo.ListSavedQueries(ctx, metadata.SavedQueryFilter{
		DatasourceID: datasourceID,
		Source:       "example",
	})
	if err != nil {
		return 0
	}
	created := 0
	for _, row := range rows {
		if !row.IsActive || strings.TrimSpace(row.SQLQuery) == "" {
			continue
		}
		name := row.Name
		if name == "" {
			name = row.Question
		}
		content := fmt.Sprintf("---\ntype: sql-pair\ntitle: %q\nquestion: %q\ndescription: %q\n---\n\n# %s\n\n```sql\n%s\n```\n",
			name, row.Question, row.Description, name, row.SQLQuery)
		fm, body := parseKnowledgeMarkdown(content)
		fileID, err := h.deps.MetaRepo.InsertKnowledgeFile(ctx, metadata.KnowledgeFileInsert{
			DatasourceID: datasourceID,
			Path:         uniqueKnowledgePath(used, knowledgeFolderSQLPairs, name),
			Title:        knowledgeTitle(fm, body, name+".md"),
			ContentMD:    content,
			Frontmatter:  marshalKnowledgeFrontmatter(fm),
			Status:       "published",
			CreatedBy:    createdBy,
		})
		if err != nil {
			continue
		}
		_ = h.deps.MetaRepo.LinkSavedQueryKnowledgeFile(ctx, row.ID, fileID)
		created++
	}
	return created
}
