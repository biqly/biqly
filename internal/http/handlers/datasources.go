package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/security"
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

// Create handles datasource creation.
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
	if req.Config == "" {
		req.Config = "{}"
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
		DSNEncrypted: req.DSN,
		Config:       req.Config,
		IsActive:     true,
	}

	// Encrypt DSN before storing if encryptor is available
	ctx := r.Context()
	if h.deps.Encryptor != nil {
		encrypted, err := h.deps.Encryptor.Encrypt(req.DSN)
		if err != nil {
			slog.ErrorContext(ctx, "encrypt DSN failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to encrypt DSN")
			return
		}
		ds.DSNEncrypted = encrypted
	}

	if err := h.deps.MetaRepo.CreateDatasource(ctx, ds); err != nil {
		slog.ErrorContext(ctx, "create datasource failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create datasource")
		return
	}

	writeJSON(w, http.StatusCreated, ds)
}

// List returns all configured datasources.
func (h *DatasourceHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	datasources, err := h.deps.MetaRepo.ListDatasources(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "list datasources failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to list datasources")
		return
	}

	// Mask DSN in response
	for i := range datasources {
		datasources[i].DSNEncrypted = "***REDACTED***"
	}

	writeJSON(w, http.StatusOK, datasources)
}

// Get returns a single datasource by ID.
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

// Delete removes a datasource by ID.
func (h *DatasourceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	if err := h.deps.MetaRepo.DeleteDatasource(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete datasource")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Test verifies connectivity to a datasource.
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

	dsn, err := security.ConnectionDSN(h.deps.Encryptor, ds.DSNEncrypted)
	if err != nil {
		slog.ErrorContext(ctx, "decrypt DSN failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to decrypt DSN")
		return
	}

	start := time.Now()
	if err := driver.Ping(ctx, dsn); err != nil {
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

// SyncMetadata introspects and persists the schema of a datasource.
//
//nolint:gocyclo // linear step-by-step sync process, each step is independent
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

	dsn, err := security.ConnectionDSN(h.deps.Encryptor, ds.DSNEncrypted)
	if err != nil {
		slog.ErrorContext(ctx, "decrypt DSN failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to decrypt DSN")
		return
	}

	db, err := driver.Open(ctx, dsn)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to open connection: %s", err.Error()))
		return
	}
	defer func() { _ = db.Close() }()

	result, err := driver.Introspect(ctx, db)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("introspection failed: %s", err.Error()))
		return
	}

	schemaIDs := make(map[string]string, len(result.Schemas))
	tableIDs := make(map[[2]string]string, len(result.Tables))

	// Store schemas
	for _, s := range result.Schemas {
		schema := metadata.Schema{
			ID:           uuid.New().String(),
			DatasourceID: ds.ID,
			SchemaName:   s.Name,
		}
		schemaID, err := h.deps.MetaRepo.UpsertSchema(ctx, ds.ID, schema)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save schemas: %s", err.Error()))
			return
		}
		schemaIDs[s.Name] = schemaID
	}

	// Store tables (description from native DB comment when present)
	for _, t := range result.Tables {
		schemaID, ok := schemaIDs[t.SchemaName]
		if !ok {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("missing schema for table: %s.%s", t.SchemaName, t.TableName))
			return
		}

		table := metadata.Table{
			ID:           uuid.New().String(),
			DatasourceID: ds.ID,
			SchemaID:     schemaID,
			SchemaName:   t.SchemaName,
			TableName:    t.TableName,
			TableType:    t.TableType,
			RowEstimate:  t.RowEstimate,
		}
		if t.Comment != "" {
			c := t.Comment
			table.Description = &c
		}
		tableID, err := h.deps.MetaRepo.UpsertTable(ctx, ds.ID, table)
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save tables: %s", err.Error()))
			return
		}
		tableIDs[[2]string{t.SchemaName, t.TableName}] = tableID
	}

	// Build FK lookup so column rows carry referenced_table/_column even when the
	// driver-level introspection only reports relations separately.
	type fkTarget struct {
		schema string
		table  string
		column string
	}
	fkBySource := make(map[[3]string]fkTarget, len(result.Relations))
	for _, rel := range result.Relations {
		fkBySource[[3]string{rel.FromSchema, rel.FromTable, rel.FromColumn}] = fkTarget{
			schema: rel.ToSchema,
			table:  rel.ToTable,
			column: rel.ToColumn,
		}
	}

	// Store columns (description from native DB comment when present)
	for _, c := range result.Columns {
		tableID, ok := tableIDs[[2]string{c.SchemaName, c.TableName}]
		if !ok {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("missing table for column: %s.%s.%s", c.SchemaName, c.TableName, c.ColumnName))
			return
		}

		colDefault := c.ColumnDefault
		col := metadata.Column{
			ID:               uuid.New().String(),
			DatasourceID:     ds.ID,
			TableID:          tableID,
			SchemaName:       c.SchemaName,
			TableName:        c.TableName,
			ColumnName:       c.ColumnName,
			DataType:         c.DataType,
			Nullable:         c.Nullable,
			OrdinalPosition:  &c.OrdinalPosition,
			CharMaxLength:    c.CharMaxLength,
			NumericPrecision: c.NumericPrecision,
			NumericScale:     c.NumericScale,
			ColumnDefault:    &colDefault,
			IsPrimaryKey:     c.IsPrimaryKey,
			IsForeignKey:     c.IsForeignKey,
		}
		if target, isFK := fkBySource[[3]string{c.SchemaName, c.TableName, c.ColumnName}]; isFK {
			col.IsForeignKey = true
			schema, table, column := target.schema, target.table, target.column
			col.ReferencedSchema = &schema
			col.ReferencedTable = &table
			col.ReferencedColumn = &column
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
