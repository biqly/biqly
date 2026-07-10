package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"

	"github.com/biqly/biqly/internal/app"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
)

// AIKnowledgeHandler serves the markdown knowledge base: datasource-scoped
// .md files (with YAML frontmatter) in virtual folders. Publishing a file
// extracts structured records into the existing prompt stores so the
// text-to-SQL pipeline keeps consuming the same loaders; agents additionally
// read published files directly via the knowledge tools.
type AIKnowledgeHandler struct {
	deps *app.AIDeps
}

// NewAIKnowledgeHandler creates an AIKnowledgeHandler.
func NewAIKnowledgeHandler(deps *app.AIDeps) *AIKnowledgeHandler {
	return &AIKnowledgeHandler{deps: deps}
}

type knowledgeFileMeta struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Folder      string `json:"folder"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
	UpdatedAt   string `json:"updated_at"`
}

type knowledgeFileResponse struct {
	knowledgeFileMeta
	ContentMD   string         `json:"content_md"`
	Frontmatter map[string]any `json:"frontmatter,omitempty"`
	CreatedBy   string         `json:"created_by,omitempty"`
	CreatedAt   string         `json:"created_at"`
}

func knowledgeMetaFromRow(row metadata.KnowledgeFileRow) knowledgeFileMeta {
	meta := knowledgeFileMeta{
		ID:        row.ID,
		Path:      row.Path,
		Folder:    row.Folder,
		Title:     row.Title,
		Status:    row.Status,
		UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if len(row.Frontmatter) > 0 {
		var fm map[string]any
		if err := sonic.Unmarshal(row.Frontmatter, &fm); err == nil {
			meta.Description = fmString(fm, "description")
			if meta.Description == "" {
				meta.Description = fmString(fm, "usage_notes")
			}
		}
	}
	return meta
}

func knowledgeResponseFromRow(row metadata.KnowledgeFileRow) knowledgeFileResponse {
	resp := knowledgeFileResponse{
		knowledgeFileMeta: knowledgeMetaFromRow(row),
		ContentMD:         row.ContentMD,
		CreatedBy:         row.CreatedBy,
		CreatedAt:         row.CreatedAt.UTC().Format(time.RFC3339),
	}
	if len(row.Frontmatter) > 0 {
		var fm map[string]any
		if err := sonic.Unmarshal(row.Frontmatter, &fm); err == nil {
			resp.Frontmatter = fm
		}
	}
	return resp
}

// List returns a datasource's knowledge files (tree metadata, no content).
// ?published=true narrows to published files — the agent-facing view.
func (h *AIKnowledgeHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	datasourceID, ok := requireQueryParam(w, r, "datasource_id")
	if !ok {
		return
	}
	var rows []metadata.KnowledgeFileRow
	var err error
	if r.URL.Query().Get("published") == "true" {
		rows, err = h.deps.MetaRepo.ListPublishedKnowledgeFiles(ctx, datasourceID)
	} else {
		rows, err = h.deps.MetaRepo.ListKnowledgeFiles(ctx, datasourceID)
	}
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list knowledge files", err)
		return
	}
	out := make([]knowledgeFileMeta, 0, len(rows))
	for _, row := range rows {
		out = append(out, knowledgeMetaFromRow(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"files": out})
}

// GetByPath returns one PUBLISHED file by datasource + path — the agent
// read_knowledge_file lookup.
func (h *AIKnowledgeHandler) GetByPath(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	datasourceID, ok := requireQueryParam(w, r, "datasource_id")
	if !ok {
		return
	}
	path, ok := requireQueryParam(w, r, "path")
	if !ok {
		return
	}
	row, err := h.deps.MetaRepo.GetKnowledgeFileByPath(ctx, datasourceID, path)
	if err != nil {
		if errors.Is(err, metadata.ErrKnowledgeFileNotFound) {
			writeEntityNotFound(w, "knowledge file")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load knowledge file", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"file": knowledgeResponseFromRow(row)})
}

// Get returns one file with its full markdown content.
func (h *AIKnowledgeHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	row, err := h.deps.MetaRepo.GetKnowledgeFile(ctx, chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, metadata.ErrKnowledgeFileNotFound) {
			writeEntityNotFound(w, "knowledge file")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load knowledge file", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"file": knowledgeResponseFromRow(row)})
}

type knowledgeFilePayload struct {
	DatasourceID string `json:"datasource_id,omitempty"`
	Path         string `json:"path"`
	ContentMD    string `json:"content_md"`
}

func (p *knowledgeFilePayload) validate() string {
	if !validKnowledgePath(strings.TrimSpace(p.Path)) {
		return "path must be a relative markdown file like instructions/my-rule.md"
	}
	if strings.TrimSpace(p.ContentMD) == "" {
		return "content_md is required"
	}
	return ""
}

// Create stores a new draft file.
func (h *AIKnowledgeHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	payload, ok := decodeJSON[knowledgeFilePayload](w, r)
	if !ok {
		return
	}
	if payload.DatasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return
	}
	if msg := payload.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	path := strings.TrimSpace(payload.Path)
	fm, body := parseKnowledgeMarkdown(payload.ContentMD)
	id, err := h.deps.MetaRepo.InsertKnowledgeFile(ctx, metadata.KnowledgeFileInsert{
		DatasourceID: payload.DatasourceID,
		Path:         path,
		Title:        knowledgeTitle(fm, body, path),
		ContentMD:    payload.ContentMD,
		Frontmatter:  marshalKnowledgeFrontmatter(fm),
		Status:       "draft",
		CreatedBy:    bimw.UserID(ctx),
	})
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create knowledge file", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// Update replaces a file's path/content. An edited published file drops back
// to draft until re-published so the extracted records never silently drift
// from the visible markdown.
func (h *AIKnowledgeHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	payload, ok := decodeJSON[knowledgeFilePayload](w, r)
	if !ok {
		return
	}
	if msg := payload.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	path := strings.TrimSpace(payload.Path)
	fm, body := parseKnowledgeMarkdown(payload.ContentMD)
	err := h.deps.MetaRepo.UpdateKnowledgeFile(ctx, chi.URLParam(r, "id"), metadata.KnowledgeFileUpdate{
		Path:        path,
		Title:       knowledgeTitle(fm, body, path),
		ContentMD:   payload.ContentMD,
		Frontmatter: marshalKnowledgeFrontmatter(fm),
		Status:      "draft",
	})
	if err != nil {
		if errors.Is(err, metadata.ErrKnowledgeFileNotFound) {
			writeEntityNotFound(w, "knowledge file")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update knowledge file", err)
		return
	}
	writeOK(w)
}

// Delete removes a file and deactivates whatever it extracted.
func (h *AIKnowledgeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if err := h.deps.MetaRepo.DeactivateExtractionsForKnowledgeFile(ctx, id); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to detach knowledge file", err)
		return
	}
	if err := h.deps.MetaRepo.DeleteKnowledgeFile(ctx, id); err != nil {
		if errors.Is(err, metadata.ErrKnowledgeFileNotFound) {
			writeEntityNotFound(w, "knowledge file")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to delete knowledge file", err)
		return
	}
	writeOK(w)
}

// Publish marks a file published and extracts its structured record
// (instructions/glossary/sql-pairs folders; other folders publish as-is).
func (h *AIKnowledgeHandler) Publish(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	row, err := h.deps.MetaRepo.GetKnowledgeFile(ctx, id)
	if err != nil {
		if errors.Is(err, metadata.ErrKnowledgeFileNotFound) {
			writeEntityNotFound(w, "knowledge file")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load knowledge file", err)
		return
	}
	if err := h.extractKnowledgeFile(ctx, row); err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := h.deps.MetaRepo.UpdateKnowledgeFile(ctx, id, metadata.KnowledgeFileUpdate{
		Path:        row.Path,
		Title:       row.Title,
		ContentMD:   row.ContentMD,
		Frontmatter: row.Frontmatter,
		Status:      "published",
	}); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to publish knowledge file", err)
		return
	}
	writeOK(w)
}

// extractKnowledgeFile syncs a file's structured record by folder. Returns a
// user-facing error when required frontmatter is missing.
func (h *AIKnowledgeHandler) extractKnowledgeFile(ctx context.Context, row metadata.KnowledgeFileRow) error {
	fm, body := parseKnowledgeMarkdown(row.ContentMD)
	switch row.Folder {
	case knowledgeFolderInstructions:
		content := strings.TrimSpace(body)
		if content == "" {
			content = strings.TrimSpace(row.ContentMD)
		}
		return h.deps.MetaRepo.UpsertInstructionFromKnowledge(ctx, row.ID, row.DatasourceID, row.Title, content)
	case knowledgeFolderGlossary:
		term := fmString(fm, "term")
		if term == "" {
			term = row.Title
		}
		if term == "" {
			return errors.New("glossary file needs a term (frontmatter `term:` or a title)")
		}
		mapsToType := fmString(fm, "maps_to_type")
		if mapsToType == "" {
			mapsToType = "model"
		}
		definition := fmString(fm, "definition")
		if definition == "" {
			definition = strings.TrimSpace(body)
		}
		return h.deps.MetaRepo.UpsertGlossaryFromKnowledge(ctx, row.ID, row.DatasourceID, metadata.KnowledgeGlossaryUpsert{
			Term:       term,
			Definition: definition,
			MapsToType: mapsToType,
			MapsToName: fmString(fm, "maps_to_name"),
			Aliases:    fmStrings(fm, "aliases"),
		})
	case knowledgeFolderSQLPairs:
		question := fmString(fm, "question")
		if question == "" {
			return errors.New("sql-pairs file needs a `question:` frontmatter field")
		}
		sqlQuery := fmString(fm, "sql")
		if sqlQuery == "" {
			sqlQuery = knowledgeSQLFromBody(body)
		}
		if sqlQuery == "" {
			return errors.New("sql-pairs file needs a `sql:` frontmatter field or a ```sql code block")
		}
		return h.deps.MetaRepo.UpsertSavedQueryExampleFromKnowledge(ctx, row.ID, row.DatasourceID, metadata.KnowledgeExampleUpsert{
			Name:         row.Title,
			Description:  fmString(fm, "description"),
			Question:     question,
			QuestionHash: metadata.QuestionHash(question),
			SQLQuery:     sqlQuery,
			CreatedBy:    row.CreatedBy,
		})
	default:
		// metrics/ and free-form folders publish as plain knowledge; agents
		// read them via list/read_knowledge_file.
		return nil
	}
}

func marshalKnowledgeFrontmatter(fm map[string]any) []byte {
	if len(fm) == 0 {
		return nil
	}
	raw, err := sonic.Marshal(fm)
	if err != nil {
		return nil
	}
	return raw
}
