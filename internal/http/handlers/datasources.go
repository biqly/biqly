package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/security"
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

type connectionRequest struct {
	Host             string            `json:"host"`
	Port             *int              `json:"port,omitempty"`
	Username         string            `json:"username,omitempty"`
	Password         string            `json:"password,omitempty"`
	DatabaseName     string            `json:"database_name,omitempty"`
	SSLMode          string            `json:"ssl_mode,omitempty"`
	ConnectionParams map[string]string `json:"connection_params,omitempty"`
}

type createDatasourceRequest struct {
	ID         string              `json:"id,omitempty"`
	Name       string              `json:"name"`
	Type       string              `json:"type"`
	Mode       string              `json:"mode,omitempty"` // raw | structured
	DSN        string              `json:"dsn,omitempty"`
	Connection *connectionRequest  `json:"connection,omitempty"`
	Config     string              `json:"config,omitempty"`
}

func resolveCreateDatasourceMode(req *createDatasourceRequest) string {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode != "" {
		return mode
	}
	hasConn := req.Connection != nil && strings.TrimSpace(req.Connection.Host) != ""
	hasDSN := strings.TrimSpace(req.DSN) != ""
	if hasConn && hasDSN {
		return ""
	}
	if hasConn {
		return metadata.DSNModeStructured
	}
	if hasDSN {
		return metadata.DSNModeRaw
	}
	return ""
}

func encryptSecret(enc *security.Encryption, plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	if enc == nil {
		return plaintext, nil
	}
	return enc.Encrypt(plaintext)
}

func optionalStringPtr(s string) *string {
	v := strings.TrimSpace(s)
	if v == "" {
		return nil
	}
	return &v
}

// Create handles datasource creation.
//
//nolint:gocyclo // raw vs structured branching and validation are explicit UX surface
func (h *DatasourceHandler) Create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createDatasourceRequest](w, r)
	if !ok {
		return
	}

	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Type) == "" {
		writeError(w, http.StatusBadRequest, "name and type are required")
		return
	}
	if req.Config == "" {
		req.Config = "{}"
	}

	driverType := datasource.NormalizeDriverType(req.Type)
	if _, err := h.deps.DriverReg.Get(driverType); err != nil {
		writeError(w, http.StatusBadRequest, "unsupported datasource type")
		return
	}

	mode := resolveCreateDatasourceMode(req)
	if mode != metadata.DSNModeRaw && mode != metadata.DSNModeStructured {
		writeError(w, http.StatusBadRequest, "set mode to raw or structured, or supply only dsn or only structured connection fields")
		return
	}

	if mode == metadata.DSNModeStructured && strings.TrimSpace(req.DSN) != "" {
		writeError(w, http.StatusBadRequest, "omit dsn when mode is structured")
		return
	}
	if mode == metadata.DSNModeRaw && req.Connection != nil {
		hasConnPayload := strings.TrimSpace(req.Connection.Host) != "" ||
			(req.Connection.Port != nil && *req.Connection.Port > 0) ||
			strings.TrimSpace(req.Connection.Username) != "" ||
			strings.TrimSpace(req.Connection.Password) != "" ||
			strings.TrimSpace(req.Connection.DatabaseName) != "" ||
			len(req.Connection.ConnectionParams) > 0
		if hasConnPayload {
			writeError(w, http.StatusBadRequest, "omit connection when mode is raw")
			return
		}
	}

	ctx := r.Context()
	ds := &metadata.Datasource{
		ID:       uuid.New().String(),
		Name:     strings.TrimSpace(req.Name),
		Type:     driverType,
		Config:   req.Config,
		IsActive: true,
	}

	switch mode {
	case metadata.DSNModeRaw:
		dsnPlain := strings.TrimSpace(req.DSN)
		if dsnPlain == "" {
			writeError(w, http.StatusBadRequest, "dsn is required when mode is raw")
			return
		}
		ds.DSNMode = metadata.DSNModeRaw
		encStr, err := encryptSecret(h.deps.Encryptor, dsnPlain)
		if err != nil {
			slog.ErrorContext(ctx, "encrypt DSN failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to encrypt DSN")
			return
		}
		ds.DSNEncrypted = encStr
	case metadata.DSNModeStructured:
		if req.Connection == nil {
			writeError(w, http.StatusBadRequest, "connection is required when mode is structured")
			return
		}
		c := req.Connection
		host := strings.TrimSpace(c.Host)
		if host == "" {
			writeError(w, http.StatusBadRequest, "connection.host is required")
			return
		}
		ds.Host = &host

		port := datasource.DefaultPort(driverType)
		if c.Port != nil && *c.Port > 0 {
			port = *c.Port
		}
		ds.Port = &port

		if u := optionalStringPtr(c.Username); u != nil {
			ds.Username = u
		}
		if db := optionalStringPtr(c.DatabaseName); db != nil {
			ds.DatabaseName = db
		}
		defaults := datasource.DriverConnectionDefaults(driverType)
		ssl := strings.TrimSpace(c.SSLMode)
		if ssl == "" {
			ssl = defaults.SSLMode
		}
		if ssl != "" {
			ds.SSLMode = &ssl
		}

		ext := map[string]string{}
		for k, v := range c.ConnectionParams {
			k = strings.TrimSpace(k)
			if k != "" {
				ext[k] = v
			}
		}
		cpRaw, err := json.Marshal(ext)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid connection_params")
			return
		}
		ds.ConnectionParams = append(json.RawMessage(nil), cpRaw...)

		fields := datasource.ConnectionFields{
			Host:         host,
			Port:         port,
			Username:     strings.TrimSpace(c.Username),
			Password:     c.Password,
			DatabaseName: strings.TrimSpace(c.DatabaseName),
			SSLMode:      ssl,
			Extra:        ext,
		}
		if _, err := datasource.ComposeDSN(driverType, fields); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		passEnc, err := encryptSecret(h.deps.Encryptor, c.Password)
		if err != nil {
			slog.ErrorContext(ctx, "encrypt datasource password failed", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to encrypt password")
			return
		}
		ds.PasswordEncrypted = passEnc

		ds.DSNMode = metadata.DSNModeStructured
		ds.DSNEncrypted = ""
	}

	if err := h.deps.MetaRepo.CreateDatasource(ctx, ds); err != nil {
		slog.ErrorContext(ctx, "create datasource failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to create datasource")
		return
	}

	ds.DSNEncrypted = ""
	ds.PasswordEncrypted = ""
	writeJSON(w, http.StatusCreated, ds)
}

func (h *DatasourceHandler) datasourceDraft(ctx context.Context, req createDatasourceRequest, id string, existing *metadata.Datasource) (*metadata.Datasource, string, int, string, error) {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Type) == "" {
		return nil, "", http.StatusBadRequest, "name and type are required", nil
	}
	if req.Config == "" {
		req.Config = "{}"
	}

	driverType := datasource.NormalizeDriverType(req.Type)
	if _, err := h.deps.DriverReg.Get(driverType); err != nil {
		return nil, "", http.StatusBadRequest, "unsupported datasource type", err
	}

	mode := resolveCreateDatasourceMode(&req)
	if mode == "" && existing != nil {
		mode = strings.TrimSpace(existing.DSNMode)
	}
	if mode != metadata.DSNModeRaw && mode != metadata.DSNModeStructured {
		return nil, "", http.StatusBadRequest, "set mode to raw or structured, or supply only dsn or only structured connection fields", nil
	}
	if mode == metadata.DSNModeStructured && strings.TrimSpace(req.DSN) != "" {
		return nil, "", http.StatusBadRequest, "omit dsn when mode is structured", nil
	}
	if mode == metadata.DSNModeRaw && req.Connection != nil {
		hasConnPayload := strings.TrimSpace(req.Connection.Host) != "" ||
			req.Connection.Port != nil ||
			strings.TrimSpace(req.Connection.Username) != "" ||
			req.Connection.Password != "" ||
			strings.TrimSpace(req.Connection.DatabaseName) != "" ||
			len(req.Connection.ConnectionParams) > 0
		if hasConnPayload {
			return nil, "", http.StatusBadRequest, "omit connection when mode is raw", nil
		}
	}

	ds := &metadata.Datasource{
		ID:       id,
		Name:     strings.TrimSpace(req.Name),
		Type:     driverType,
		Config:   req.Config,
		IsActive: true,
	}
	if ds.ID == "" {
		ds.ID = uuid.New().String()
	}
	if existing != nil {
		ds.LastSyncAt = existing.LastSyncAt
		ds.CreatedAt = existing.CreatedAt
	}

	switch mode {
	case metadata.DSNModeRaw:
		dsnPlain := strings.TrimSpace(req.DSN)
		if dsnPlain == "" {
			if existing == nil || strings.TrimSpace(existing.DSNEncrypted) == "" {
				return nil, "", http.StatusBadRequest, "dsn is required when mode is raw", nil
			}
			runtimeDSN, err := existing.RuntimeDSN(h.deps.Encryptor)
			if err != nil {
				return nil, "", http.StatusInternalServerError, "failed to decrypt DSN", err
			}
			ds.DSNEncrypted = existing.DSNEncrypted
			ds.DSNMode = metadata.DSNModeRaw
			return ds, runtimeDSN, http.StatusOK, "", nil
		}
		encStr, err := encryptSecret(h.deps.Encryptor, dsnPlain)
		if err != nil {
			return nil, "", http.StatusInternalServerError, "failed to encrypt DSN", err
		}
		ds.DSNEncrypted = encStr
		ds.DSNMode = metadata.DSNModeRaw
		return ds, dsnPlain, http.StatusOK, "", nil
	case metadata.DSNModeStructured:
		if req.Connection == nil {
			return nil, "", http.StatusBadRequest, "connection is required when mode is structured", nil
		}
		c := req.Connection
		host := strings.TrimSpace(c.Host)
		if host == "" {
			return nil, "", http.StatusBadRequest, "connection.host is required", nil
		}
		ds.Host = &host

		port := datasource.DefaultPort(driverType)
		if c.Port != nil && *c.Port > 0 {
			port = *c.Port
		}
		ds.Port = &port

		if u := optionalStringPtr(c.Username); u != nil {
			ds.Username = u
		}
		if db := optionalStringPtr(c.DatabaseName); db != nil {
			ds.DatabaseName = db
		}
		defaults := datasource.DriverConnectionDefaults(driverType)
		ssl := strings.TrimSpace(c.SSLMode)
		if ssl == "" {
			ssl = defaults.SSLMode
		}
		if ssl != "" {
			ds.SSLMode = &ssl
		}

		ext := map[string]string{}
		for k, v := range c.ConnectionParams {
			k = strings.TrimSpace(k)
			if k != "" {
				ext[k] = v
			}
		}
		cpRaw, err := json.Marshal(ext)
		if err != nil {
			return nil, "", http.StatusBadRequest, "invalid connection_params", err
		}
		ds.ConnectionParams = append(json.RawMessage(nil), cpRaw...)

		password := c.Password
		if password == "" && existing != nil && existing.DSNMode == metadata.DSNModeStructured {
			password, err = security.ConnectionDSN(h.deps.Encryptor, existing.PasswordEncrypted)
			if err != nil {
				return nil, "", http.StatusInternalServerError, "failed to decrypt password", err
			}
			ds.PasswordEncrypted = existing.PasswordEncrypted
		}

		fields := datasource.ConnectionFields{
			Host:         host,
			Port:         port,
			Username:     strings.TrimSpace(c.Username),
			Password:     password,
			DatabaseName: strings.TrimSpace(c.DatabaseName),
			SSLMode:      ssl,
			Extra:        ext,
		}
		runtimeDSN, err := datasource.ComposeDSN(driverType, fields)
		if err != nil {
			return nil, "", http.StatusBadRequest, err.Error(), err
		}

		if c.Password != "" || ds.PasswordEncrypted == "" {
			passEnc, err := encryptSecret(h.deps.Encryptor, c.Password)
			if err != nil {
				return nil, "", http.StatusInternalServerError, "failed to encrypt password", err
			}
			ds.PasswordEncrypted = passEnc
		}
		ds.DSNMode = metadata.DSNModeStructured
		ds.DSNEncrypted = ""
		return ds, runtimeDSN, http.StatusOK, "", nil
	default:
		return nil, "", http.StatusBadRequest, "unsupported datasource mode", nil
	}
}

func writeDatasourcePayloadError(ctx context.Context, w http.ResponseWriter, status int, message string, err error) {
	if status >= http.StatusInternalServerError {
		slog.ErrorContext(ctx, "datasource payload failed", "error", err)
	}
	writeError(w, status, message)
}

func maskDatasourceSecrets(ds *metadata.Datasource) {
	ds.DSNEncrypted = ""
	ds.PasswordEncrypted = ""
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
		maskDatasourceSecrets(&datasources[i])
	}

	writeJSON(w, http.StatusOK, datasources)
}

// Get returns a single datasource by ID.
func (h *DatasourceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()

	ds, err := h.deps.MetaRepo.GetDatasource(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "datasource not found")
		return
	}

	maskDatasourceSecrets(ds)
	writeJSON(w, http.StatusOK, ds)
}

// Update changes connection settings for an existing datasource.
func (h *DatasourceHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeJSON[createDatasourceRequest](w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	existing, err := h.deps.MetaRepo.GetDatasource(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "datasource not found")
		return
	}

	ds, _, status, message, err := h.datasourceDraft(ctx, *req, id, existing)
	if err != nil || status >= http.StatusBadRequest {
		writeDatasourcePayloadError(ctx, w, status, message, err)
		return
	}

	if err := h.deps.MetaRepo.UpdateDatasource(ctx, ds); err != nil {
		slog.ErrorContext(ctx, "update datasource failed", "datasource_id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to update datasource")
		return
	}

	maskDatasourceSecrets(ds)
	writeJSON(w, http.StatusOK, ds)
}

// Delete removes a datasource by ID.
func (h *DatasourceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()

	if err := h.deps.MetaRepo.DeleteDatasource(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete datasource")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Test verifies connectivity to a datasource.
func (h *DatasourceHandler) Test(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()

	ds, err := h.deps.MetaRepo.GetDatasource(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "datasource not found")
		return
	}

	driver, err := h.deps.DriverReg.Get(ds.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, app.MsgUnsupportedDriver)
		return
	}

	dsn, err := ds.RuntimeDSN(h.deps.Encryptor)
	if err != nil {
		slog.ErrorContext(ctx, "decrypt DSN failed", "error", err)
		writeError(w, http.StatusInternalServerError, "failed to decrypt DSN")
		return
	}

	start := time.Now()
	if err := driver.Ping(ctx, dsn); err != nil {
		slog.ErrorContext(ctx, "datasource ping failed", "datasource_id", id, "error", err)
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   "connection failed",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"latency_ms": time.Since(start).Milliseconds(),
	})
}

// TestDraft verifies connectivity for unsaved datasource settings.
func (h *DatasourceHandler) TestDraft(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createDatasourceRequest](w, r)
	if !ok {
		return
	}
	ctx := r.Context()

	var existing *metadata.Datasource
	if strings.TrimSpace(req.ID) != "" {
		ds, err := h.deps.MetaRepo.GetDatasource(ctx, strings.TrimSpace(req.ID))
		if err != nil {
			writeError(w, http.StatusNotFound, "datasource not found")
			return
		}
		existing = ds
	}

	ds, runtimeDSN, status, message, err := h.datasourceDraft(ctx, *req, strings.TrimSpace(req.ID), existing)
	if err != nil || status >= http.StatusBadRequest {
		writeDatasourcePayloadError(ctx, w, status, message, err)
		return
	}

	driver, err := h.deps.DriverReg.Get(ds.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, app.MsgUnsupportedDriver)
		return
	}

	start := time.Now()
	if err := driver.Ping(ctx, runtimeDSN); err != nil {
		slog.ErrorContext(ctx, "draft datasource ping failed", "datasource_id", req.ID, "error", err)
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"error":   "connection failed",
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
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()

	resolved, err := h.deps.ResolveDatasourceDB(ctx, id)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to open connection", err)
		return
	}
	defer func() { _ = resolved.DB.Close() }()
	ds := resolved.Record
	driver := resolved.Driver
	db := resolved.DB

	result, err := driver.Introspect(ctx, db)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "introspection failed", err)
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
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to save schemas", err)
			return
		}
		schemaIDs[s.Name] = schemaID
	}

	// Store tables (description from native DB comment when present)
	for _, t := range result.Tables {
		schemaID, ok := schemaIDs[t.SchemaName]
		if !ok {
			writeInternalError(ctx, w, http.StatusInternalServerError, "metadata sync failed",
				fmt.Errorf("missing schema for table %s.%s", t.SchemaName, t.TableName))
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
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to save tables", err)
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
			writeInternalError(ctx, w, http.StatusInternalServerError, "metadata sync failed",
				fmt.Errorf("missing table for column %s.%s.%s", c.SchemaName, c.TableName, c.ColumnName))
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
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to save columns", err)
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
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to save relations", err)
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
