package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// DatasourceHandler handles datasource CRUD operations.
type DatasourceHandler struct {
	deps *app.Dependencies
}

// NewDatasourceHandler creates a new datasource handler.
func NewDatasourceHandler(deps *app.Dependencies) *DatasourceHandler {
	return &DatasourceHandler{deps: deps}
}

type createDatasourceRequest struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	DSN    string `json:"dsn"`
	Config string `json:"config,omitempty"`
}

func (h *DatasourceHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createDatasourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Type == "" || req.DSN == "" {
		writeError(w, http.StatusBadRequest, "name, type, and dsn are required")
		return
	}

	// Get driver to validate type
	_, err := h.deps.DriverReg.Get(req.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ds := &metadata.Datasource{
		ID:           uuid.New().String(),
		Name:         req.Name,
		Type:         req.Type,
		DSNEncrypted: req.DSN, // TODO: encrypt this
		Config:       req.Config,
		IsActive:     true,
	}

	ctx := r.Context()
	if err := h.deps.MetaRepo.CreateDatasource(ctx, ds); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create datasource")
		return
	}

	writeJSON(w, http.StatusCreated, ds)
}

func (h *DatasourceHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	datasources, err := h.deps.MetaRepo.ListDatasources(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list datasources")
		return
	}

	// Mask DSN in response
	for i := range datasources {
		datasources[i].DSNEncrypted = "***REDACTED***"
	}

	writeJSON(w, http.StatusOK, datasources)
}

func (h *DatasourceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	ds, err := h.deps.MetaRepo.GetDatasource(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "datasource not found")
		return
	}

	ds.DSNEncrypted = "***REDACTED***"
	writeJSON(w, http.StatusOK, ds)
}

func (h *DatasourceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	if err := h.deps.MetaRepo.DeleteDatasource(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete datasource")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *DatasourceHandler) Test(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	ds, err := h.deps.MetaRepo.GetDatasource(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "datasource not found")
		return
	}

	driver, err := h.deps.DriverReg.Get(ds.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported driver: %s", ds.Type))
		return
	}

	start := time.Now()
	if err := driver.Ping(ctx, ds.DSNEncrypted); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"latency_ms": time.Since(start).Milliseconds(),
	})
}

func (h *DatasourceHandler) SyncMetadata(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	ds, err := h.deps.MetaRepo.GetDatasource(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "datasource not found")
		return
	}

	driver, err := h.deps.DriverReg.Get(ds.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported driver: %s", ds.Type))
		return
	}

	db, err := driver.Open(ctx, ds.DSNEncrypted)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to open connection: %s", err.Error()))
		return
	}
	defer db.Close()

	result, err := driver.Introspect(ctx, db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("introspection failed: %s", err.Error()))
		return
	}

	// Store schemas
	for _, s := range result.Schemas {
		schema := metadata.Schema{
			ID:           uuid.New().String(),
			DatasourceID: ds.ID,
			SchemaName:   s.Name,
		}
		if err := h.deps.MetaRepo.UpsertSchemas(ctx, ds.ID, []metadata.Schema{schema}); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save schemas: %s", err.Error()))
			return
		}
	}

	// Store tables (description from native DB comment when present)
	for _, t := range result.Tables {
		table := metadata.Table{
			ID:           uuid.New().String(),
			DatasourceID: ds.ID,
			SchemaName:   t.SchemaName,
			TableName:    t.TableName,
			TableType:    t.TableType,
			RowEstimate:  t.RowEstimate,
		}
		if t.Comment != "" {
			c := t.Comment
			table.Description = &c
		}
		if err := h.deps.MetaRepo.UpsertTables(ctx, ds.ID, []metadata.Table{table}); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save tables: %s", err.Error()))
			return
		}
	}

	// Store columns (description from native DB comment when present)
	for _, c := range result.Columns {
		colDefault := c.ColumnDefault
		col := metadata.Column{
			ID:             uuid.New().String(),
			DatasourceID:   ds.ID,
			SchemaName:     c.SchemaName,
			TableName:      c.TableName,
			ColumnName:     c.ColumnName,
			DataType:       c.DataType,
			Nullable:       c.Nullable,
			OrdinalPosition: &c.OrdinalPosition,
			CharMaxLength:  c.CharMaxLength,
			NumericPrecision: c.NumericPrecision,
			NumericScale:   c.NumericScale,
			ColumnDefault:  &colDefault,
			IsPrimaryKey:   c.IsPrimaryKey,
			IsForeignKey:   c.IsForeignKey,
		}
		if c.Comment != "" {
			cm := c.Comment
			col.Description = &cm
		}
		if err := h.deps.MetaRepo.UpsertColumns(ctx, ds.ID, []metadata.Column{col}); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save columns: %s", err.Error()))
			return
		}
	}

	// Store relations
	for _, rel := range result.Relations {
		relation := metadata.Relation{
			ID:               uuid.New().String(),
			DatasourceID:     ds.ID,
			ConstraintName:   rel.ConstraintName,
			FromSchema:       rel.FromSchema,
			FromTable:        rel.FromTable,
			FromColumn:       rel.FromColumn,
			ToSchema:         rel.ToSchema,
			ToTable:          rel.ToTable,
			ToColumn:         rel.ToColumn,
			RelationshipType: rel.RelationshipType,
		}
		if err := h.deps.MetaRepo.UpsertRelations(ctx, ds.ID, []metadata.Relation{relation}); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save relations: %s", err.Error()))
			return
		}
	}

	// Update sync timestamp
	if err := h.deps.MetaRepo.UpdateDatasourceSync(ctx, ds.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update sync timestamp")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"schemas":   len(result.Schemas),
		"tables":    len(result.Tables),
		"columns":   len(result.Columns),
		"relations": len(result.Relations),
	})
}
