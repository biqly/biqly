package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bytedance/sonic"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/internal/semantic/drift"
	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
	"github.com/google/uuid"
)

const driftNotifyMaxConcurrent = 8

var driftNotifySem = make(chan struct{}, driftNotifyMaxConcurrent)

// DatasourceHandler handles datasource CRUD operations.
type DatasourceHandler struct {
	deps *app.CatalogDeps
}

// NewDatasourceHandler creates a new datasource handler.
func NewDatasourceHandler(deps *app.CatalogDeps) *DatasourceHandler {
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
	ID         string             `json:"id,omitempty"`
	Name       string             `json:"name"`
	Type       string             `json:"type"`
	Mode       string             `json:"mode,omitempty"` // raw | structured
	DSN        string             `json:"dsn,omitempty"`
	Connection *connectionRequest `json:"connection,omitempty"`
	Config     string             `json:"config,omitempty"`
}

type functionBlocklistRequest struct {
	Custom []string `json:"custom"`
}

type functionBlocklistResponse struct {
	Defaults []string `json:"defaults"`
	Custom   []string `json:"custom"`
}

func resolveCreateDatasourceMode(req *createDatasourceRequest) string {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode != "" {
		return mode
	}
	hasConn := req.Connection != nil && connectionHasPayload(req.Connection)
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
	return new(v)
}

// Create handles datasource creation.
func (h *DatasourceHandler) Create(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createDatasourceRequest](w, r)
	if !ok {
		return
	}
	if !rejectFunctionBlocklistConfig(w, req.Config) {
		return
	}
	ctx := r.Context()

	ds, _, status, message, err := h.datasourceDraft(ctx, *req, "", nil)
	if err != nil || status >= http.StatusBadRequest {
		writeDatasourcePayloadError(ctx, w, status, message, err)
		return
	}

	if err := h.deps.MetaRepo.CreateDatasource(ctx, ds); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create datasource", err)
		return
	}

	maskDatasourceSecrets(ds)
	writeJSON(w, http.StatusCreated, ds)
}

func (h *DatasourceHandler) datasourceDraft(_ context.Context, req createDatasourceRequest, id string, existing *metadata.Datasource) (*metadata.Datasource, string, int, string, error) {
	driverType, mode, status, msg, err := h.validateDatasourceDraftRequest(&req, existing)
	if status != 0 {
		return nil, "", status, msg, err
	}

	ds := newDatasourceDraftBase(req, id, driverType, existing)

	switch mode {
	case metadata.DSNModeRaw:
		return h.buildRawModeDraft(req, ds, existing)
	case metadata.DSNModeStructured:
		return h.buildStructuredModeDraft(req, ds, existing, driverType)
	default:
		return nil, "", http.StatusBadRequest, "unsupported datasource mode", nil
	}
}

// validateDatasourceDraftRequest validates the request and resolves the driver
// type and DSN mode. It returns status==0 on success; a non-zero status carries
// the HTTP status, client message, and optional error for the failure.
func (h *DatasourceHandler) validateDatasourceDraftRequest(req *createDatasourceRequest, existing *metadata.Datasource) (driverType, mode string, status int, msg string, err error) {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Type) == "" {
		return "", "", http.StatusBadRequest, "name and type are required", nil
	}
	if req.Config == "" {
		req.Config = "{}"
	}

	driverType = datasource.NormalizeDriverType(req.Type)
	if _, gerr := h.deps.DriverReg.Get(driverType); gerr != nil {
		return "", "", http.StatusBadRequest, "unsupported datasource type", gerr
	}

	mode = resolveCreateDatasourceMode(req)
	if mode == "" && existing != nil {
		mode = strings.TrimSpace(existing.DSNMode)
	}
	if mode != metadata.DSNModeRaw && mode != metadata.DSNModeStructured {
		return "", "", http.StatusBadRequest, "set mode to raw or structured, or supply only dsn or only structured connection fields", nil
	}
	if mode == metadata.DSNModeStructured && strings.TrimSpace(req.DSN) != "" {
		return "", "", http.StatusBadRequest, "omit dsn when mode is structured", nil
	}
	if mode == metadata.DSNModeRaw && req.Connection != nil && connectionHasPayload(req.Connection) {
		return "", "", http.StatusBadRequest, "omit connection when mode is raw", nil
	}
	return driverType, mode, 0, "", nil
}

// connectionHasPayload reports whether any structured connection field is set.
func connectionHasPayload(c *connectionRequest) bool {
	return strings.TrimSpace(c.Host) != "" ||
		c.Port != nil ||
		strings.TrimSpace(c.Username) != "" ||
		c.Password != "" ||
		strings.TrimSpace(c.DatabaseName) != "" ||
		len(c.ConnectionParams) > 0
}

// newDatasourceDraftBase builds the common Datasource fields shared by both DSN modes.
func newDatasourceDraftBase(req createDatasourceRequest, id, driverType string, existing *metadata.Datasource) *metadata.Datasource {
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
	return ds
}

// buildRawModeDraft completes a raw-DSN datasource draft, reusing the existing
// encrypted DSN when the request omits a new one.
func (h *DatasourceHandler) buildRawModeDraft(req createDatasourceRequest, ds, existing *metadata.Datasource) (*metadata.Datasource, string, int, string, error) {
	dsnPlain := strings.TrimSpace(req.DSN)
	if dsnPlain == "" {
		if existing == nil || strings.TrimSpace(existing.DSNEncrypted) == "" {
			return nil, "", http.StatusBadRequest, "dsn is required when mode is raw", nil
		}
		runtimeDSN, err := metadata.RuntimeDSN(existing, h.deps.Encryptor)
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
}

// buildStructuredModeDraft completes a structured-connection datasource draft,
// composing the runtime DSN and encrypting the password.
func (h *DatasourceHandler) buildStructuredModeDraft(req createDatasourceRequest, ds, existing *metadata.Datasource, driverType string) (*metadata.Datasource, string, int, string, error) {
	if req.Connection == nil {
		return nil, "", http.StatusBadRequest, "connection is required when mode is structured", nil
	}
	c := req.Connection
	host := strings.TrimSpace(c.Host)
	if host != "" {
		ds.Host = new(host)
	}

	port := datasource.DefaultPort(driverType)
	if c.Port != nil && *c.Port > 0 {
		port = *c.Port
	}
	if port > 0 {
		ds.Port = new(port)
	}

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
		ds.SSLMode = new(ssl)
	}

	ext := connectionParamsMap(c.ConnectionParams)
	cpRaw, err := sonic.ConfigStd.Marshal(ext)
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

	runtimeDSN, err := datasource.ComposeDSN(driverType, datasource.ConnectionFields{
		Host:         host,
		Port:         port,
		Username:     strings.TrimSpace(c.Username),
		Password:     password,
		DatabaseName: strings.TrimSpace(c.DatabaseName),
		SSLMode:      ssl,
		Extra:        ext,
	})
	if err != nil {
		return nil, "", http.StatusBadRequest, err.Error(), err
	}

	if c.Password != "" || ds.PasswordEncrypted == "" {
		passEnc, encErr := encryptSecret(h.deps.Encryptor, c.Password)
		if encErr != nil {
			return nil, "", http.StatusInternalServerError, "failed to encrypt password", encErr
		}
		ds.PasswordEncrypted = passEnc
	}
	ds.DSNMode = metadata.DSNModeStructured
	ds.DSNEncrypted = ""
	return ds, runtimeDSN, http.StatusOK, "", nil
}

// connectionParamsMap returns a trimmed-key copy of the connection params, dropping empty keys.
func connectionParamsMap(params map[string]string) map[string]string {
	ext := make(map[string]string, len(params))
	for k, v := range params {
		if k = strings.TrimSpace(k); k != "" {
			ext[k] = v
		}
	}
	return ext
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
	ds.ConnectionParams = nil
}

// List returns all configured datasources.
func (h *DatasourceHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	datasources, err := h.deps.MetaRepo.ListDatasources(ctx)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list datasources", err)
		return
	}

	allowedSet, scoped, err := resolveDatasourceScope(ctx, h.deps.Config, false)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to verify datasource access", err)
		return
	}
	if scoped {
		filtered := make([]metadata.Datasource, 0, len(datasources))
		for _, ds := range datasources {
			if _, ok := allowedSet[ds.ID]; ok {
				filtered = append(filtered, ds)
			}
		}
		datasources = filtered
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
		writeEntityNotFound(w, "datasource")
		return
	}

	maskDatasourceSecrets(ds)
	writeJSON(w, http.StatusOK, ds)
}

// GetFunctionBlocklist returns immutable defaults and datasource-specific
// function denials. It deliberately never serializes the datasource itself.
func (h *DatasourceHandler) GetFunctionBlocklist(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ds, err := h.deps.MetaRepo.GetDatasource(r.Context(), id)
	if err != nil {
		writeEntityNotFound(w, "datasource")
		return
	}
	custom, err := pkgmetadata.ParseDatasourceFunctionBlocklist(ds.Config)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "invalid datasource function blocklist configuration", err)
		return
	}
	custom, err = security.NormalizeCustomFunctionBlocklist(custom)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "invalid datasource function blocklist configuration", err)
		return
	}
	writeJSON(w, http.StatusOK, functionBlocklistResponse{
		Defaults: security.DefaultDeniedFunctions(),
		Custom:   custom,
	})
}

// ReplaceFunctionBlocklist replaces datasource-specific function denials while
// retaining unrelated datasource configuration and immutable defaults.
func (h *DatasourceHandler) ReplaceFunctionBlocklist(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeJSON[functionBlocklistRequest](w, r)
	if !ok {
		return
	}
	custom, err := security.NormalizeCustomFunctionBlocklist(req.Custom)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	ds, err := h.deps.MetaRepo.GetDatasource(r.Context(), id)
	if err != nil {
		writeEntityNotFound(w, "datasource")
		return
	}
	config, err := pkgmetadata.WithDatasourceFunctionBlocklist(ds.Config, custom)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "invalid datasource configuration", err)
		return
	}
	ds.Config = config
	if err := h.deps.MetaRepo.UpdateDatasource(r.Context(), ds); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to update datasource function blocklist", err)
		return
	}
	writeJSON(w, http.StatusOK, functionBlocklistResponse{
		Defaults: security.DefaultDeniedFunctions(),
		Custom:   custom,
	})
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
	if !rejectFunctionBlocklistConfig(w, req.Config) {
		return
	}
	ctx := r.Context()

	existing, err := h.deps.MetaRepo.GetDatasource(ctx, id)
	if err != nil {
		writeEntityNotFound(w, "datasource")
		return
	}

	ds, _, status, message, err := h.datasourceDraft(ctx, *req, id, existing)
	if err != nil || status >= http.StatusBadRequest {
		writeDatasourcePayloadError(ctx, w, status, message, err)
		return
	}
	if !preserveFunctionBlocklist(ctx, existing, ds, w) {
		return
	}

	if err := h.deps.MetaRepo.UpdateDatasource(ctx, ds); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update datasource", err)
		return
	}

	maskDatasourceSecrets(ds)
	writeJSON(w, http.StatusOK, ds)
}

func rejectFunctionBlocklistConfig(w http.ResponseWriter, config string) bool {
	hasBlocklist, err := pkgmetadata.DatasourceConfigHasFunctionBlocklist(config)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid datasource configuration")
		return false
	}
	if hasBlocklist {
		writeError(w, http.StatusBadRequest, "function_blocklist can only be changed through the datasource governance endpoint")
		return false
	}
	return true
}

func preserveFunctionBlocklist(ctx context.Context, existing, updated *metadata.Datasource, w http.ResponseWriter) bool {
	custom, err := pkgmetadata.ParseDatasourceFunctionBlocklist(existing.Config)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "invalid datasource function blocklist configuration", err)
		return false
	}
	config, err := pkgmetadata.WithDatasourceFunctionBlocklist(updated.Config, custom)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "invalid datasource configuration", err)
		return false
	}
	updated.Config = config
	return true
}

// Delete removes a datasource by ID. Cascade FKs (migration 027a) make the
// DELETE clean up dependent rows; we additionally drop any cached *sql.DB
// pool so the next operation against a recreated datasource (same ID) does
// not serve a stale connection.
func (h *DatasourceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()

	if err := h.deps.MetaRepo.DeleteDatasource(ctx, id); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to delete datasource", err)
		return
	}

	if h.deps.PoolCache != nil {
		h.deps.PoolCache.Invalidate(id)
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
		writeEntityNotFound(w, "datasource")
		return
	}

	driver, err := h.deps.DriverReg.Get(ds.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, core.MsgUnsupportedDriver)
		return
	}

	dsn, err := metadata.RuntimeDSN(ds, h.deps.Encryptor)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to decrypt DSN", err)
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
			writeEntityNotFound(w, "datasource")
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
		writeError(w, http.StatusBadRequest, core.MsgUnsupportedDriver)
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

	result, err := resolved.Driver.Introspect(ctx, resolved.DB)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "introspection failed", err)
		return
	}

	schemaIDs, ok := h.storeSchemas(ctx, w, ds.ID, result)
	if !ok {
		return
	}
	tableIDs, ok := h.storeTables(ctx, w, ds.ID, result, schemaIDs)
	if !ok {
		return
	}
	if !h.storeColumns(ctx, w, ds.ID, result, tableIDs) {
		return
	}
	if !h.storeRelations(ctx, w, ds.ID, result) {
		return
	}

	// Objects dropped at the source must leave the catalog too, or stale
	// tables keep surfacing in the UI and AI describe 42P01s sampling them.
	// Skipped when introspection saw no tables at all — that smells like a
	// permission problem, and wiping the whole catalog over it would be worse.
	pruned := 0
	if len(result.Tables) > 0 {
		pruneResult, err := h.deps.MetaRepo.PruneStaleMetadata(ctx, ds.ID, syncSnapshotKeys(result))
		if err != nil {
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to prune stale metadata", err)
			return
		}
		pruned = pruneResult.Total()
	}

	if err := h.deps.MetaRepo.UpdateDatasourceSync(ctx, ds.ID); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update sync timestamp", err)
		return
	}

	h.detectSchemaDrift(ctx, ds.ID)

	response := map[string]any{
		"success":   true,
		"schemas":   len(result.Schemas),
		"tables":    len(result.Tables),
		"columns":   len(result.Columns),
		"relations": len(result.Relations),
		"pruned":    pruned,
	}
	h.appendPostSyncPIIScan(ctx, r, resolved, response)

	writeJSON(w, http.StatusOK, response)
}

// syncSnapshotKeys flattens the introspection result into the natural keys
// PruneStaleMetadata matches catalog rows against.
func syncSnapshotKeys(result *datasource.IntrospectionResult) metadata.SyncSnapshotKeys {
	keys := metadata.SyncSnapshotKeys{
		SchemaNames:         make([]string, 0, len(result.Schemas)),
		TableKeys:           make([]string, 0, len(result.Tables)),
		ColumnKeys:          make([]string, 0, len(result.Columns)),
		RelationConstraints: make([]string, 0, len(result.Relations)),
	}
	for _, s := range result.Schemas {
		keys.SchemaNames = append(keys.SchemaNames, s.Name)
	}
	for _, t := range result.Tables {
		keys.TableKeys = append(keys.TableKeys, t.SchemaName+"."+t.TableName)
	}
	for _, c := range result.Columns {
		keys.ColumnKeys = append(keys.ColumnKeys, c.SchemaName+"."+c.TableName+"."+c.ColumnName)
	}
	for _, rel := range result.Relations {
		keys.RelationConstraints = append(keys.RelationConstraints, rel.ConstraintName)
	}
	return keys
}

// storeSchemas upserts introspected schemas and returns a name→id map. The bool
// is false (and the response already written, except on context cancellation)
// when the caller should stop.
func (h *DatasourceHandler) storeSchemas(ctx context.Context, w http.ResponseWriter, dsID string, result *datasource.IntrospectionResult) (map[string]string, bool) {
	schemaIDs := make(map[string]string, len(result.Schemas))
	for _, s := range result.Schemas {
		if ctx.Err() != nil {
			return nil, false
		}
		schemaID, err := h.deps.MetaRepo.UpsertSchema(ctx, dsID, metadata.Schema{
			ID:           uuid.New().String(),
			DatasourceID: dsID,
			SchemaName:   s.Name,
		})
		if err != nil {
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to save schemas", err)
			return nil, false
		}
		schemaIDs[s.Name] = schemaID
	}
	return schemaIDs, true
}

// storeTables upserts introspected tables and returns a (schema,table)→id map.
func (h *DatasourceHandler) storeTables(ctx context.Context, w http.ResponseWriter, dsID string, result *datasource.IntrospectionResult, schemaIDs map[string]string) (map[[2]string]string, bool) {
	tableIDs := make(map[[2]string]string, len(result.Tables))
	for _, t := range result.Tables {
		if ctx.Err() != nil {
			return nil, false
		}
		schemaID, ok := schemaIDs[t.SchemaName]
		if !ok {
			writeInternalError(ctx, w, http.StatusInternalServerError, "metadata sync failed",
				fmt.Errorf("missing schema for table %s.%s", t.SchemaName, t.TableName))
			return nil, false
		}

		table := metadata.Table{
			ID:           uuid.New().String(),
			DatasourceID: dsID,
			SchemaID:     schemaID,
			SchemaName:   t.SchemaName,
			TableName:    t.TableName,
			TableType:    t.TableType,
			RowEstimate:  t.RowEstimate,
		}
		if t.Comment != "" {
			table.Description = new(t.Comment)
		}
		tableID, err := h.deps.MetaRepo.UpsertTable(ctx, dsID, table)
		if err != nil {
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to save tables", err)
			return nil, false
		}
		tableIDs[[2]string{t.SchemaName, t.TableName}] = tableID
	}
	return tableIDs, true
}

// fkTarget is the referenced side of a foreign key, keyed by source column.
type fkTarget struct {
	schema string
	table  string
	column string
}

// storeColumns upserts introspected columns (in batches), enriching each with FK
// target info derived from the introspected relations.
func (h *DatasourceHandler) storeColumns(ctx context.Context, w http.ResponseWriter, dsID string, result *datasource.IntrospectionResult, tableIDs map[[2]string]string) bool {
	fkBySource := make(map[[3]string]fkTarget, len(result.Relations))
	for _, rel := range result.Relations {
		fkBySource[[3]string{rel.FromSchema, rel.FromTable, rel.FromColumn}] = fkTarget{
			schema: rel.ToSchema,
			table:  rel.ToTable,
			column: rel.ToColumn,
		}
	}

	colBatch := make([]metadata.Column, 0, 100)
	flush := func() bool {
		if err := h.deps.MetaRepo.UpsertColumns(ctx, dsID, colBatch); err != nil {
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to save columns", err)
			return false
		}
		colBatch = colBatch[:0]
		return true
	}

	for _, c := range result.Columns {
		if ctx.Err() != nil {
			return false
		}
		tableID, ok := tableIDs[[2]string{c.SchemaName, c.TableName}]
		if !ok {
			writeInternalError(ctx, w, http.StatusInternalServerError, "metadata sync failed",
				fmt.Errorf("missing table for column %s.%s.%s", c.SchemaName, c.TableName, c.ColumnName))
			return false
		}
		colBatch = append(colBatch, buildSyncColumn(dsID, tableID, c, fkBySource))
		if len(colBatch) >= 100 {
			if !flush() {
				return false
			}
		}
	}
	if len(colBatch) > 0 {
		return flush()
	}
	return true
}

// buildSyncColumn maps an introspected column to a metadata.Column, applying FK
// target info and the native DB comment when present.
func buildSyncColumn(dsID, tableID string, c datasource.ColumnInfo, fkBySource map[[3]string]fkTarget) metadata.Column {
	col := metadata.Column{
		ID:               uuid.New().String(),
		DatasourceID:     dsID,
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
		ColumnDefault:    new(c.ColumnDefault),
		IsPrimaryKey:     c.IsPrimaryKey,
		IsForeignKey:     c.IsForeignKey,
	}
	if target, isFK := fkBySource[[3]string{c.SchemaName, c.TableName, c.ColumnName}]; isFK {
		col.IsForeignKey = true
		col.ReferencedSchema = new(target.schema)
		col.ReferencedTable = new(target.table)
		col.ReferencedColumn = new(target.column)
	}
	if c.Comment != "" {
		col.Description = new(c.Comment)
	}
	return col
}

// storeRelations upserts introspected foreign-key relations in batches.
func (h *DatasourceHandler) storeRelations(ctx context.Context, w http.ResponseWriter, dsID string, result *datasource.IntrospectionResult) bool {
	relBatch := make([]metadata.Relation, 0, 100)
	flush := func() bool {
		if err := h.deps.MetaRepo.UpsertRelations(ctx, dsID, relBatch); err != nil {
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to save relations", err)
			return false
		}
		relBatch = relBatch[:0]
		return true
	}

	for _, rel := range result.Relations {
		if ctx.Err() != nil {
			return false
		}
		relBatch = append(relBatch, metadata.Relation{
			ID:               uuid.New().String(),
			DatasourceID:     dsID,
			ConstraintName:   rel.ConstraintName,
			FromSchema:       rel.FromSchema,
			FromTable:        rel.FromTable,
			FromColumn:       rel.FromColumn,
			ToSchema:         rel.ToSchema,
			ToTable:          rel.ToTable,
			ToColumn:         rel.ToColumn,
			RelationshipType: rel.RelationshipType,
		})
		if len(relBatch) >= 100 {
			if !flush() {
				return false
			}
		}
	}
	if len(relBatch) > 0 {
		return flush()
	}
	return true
}

// detectSchemaDrift compares every active semantic model against the freshly
// synced schema, persisting and notifying on drift. Failures here are logged but
// never fail the sync (metadata is already persisted).
func (h *DatasourceHandler) detectSchemaDrift(ctx context.Context, dsID string) {
	freshColumns, err := h.deps.MetaRepo.ListColumns(ctx, dsID, "", "")
	if err != nil {
		slog.ErrorContext(ctx, "failed to list columns for drift check", "datasource_id", dsID, "error", err)
	}
	freshTables, err := h.deps.MetaRepo.ListTables(ctx, dsID, "")
	if err != nil {
		slog.ErrorContext(ctx, "failed to list tables for drift check", "datasource_id", dsID, "error", err)
	}
	models, err := h.deps.SemanticRepo.ListModels(ctx, dsID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list models for drift check", "datasource_id", dsID, "error", err)
	}

	for _, model := range models {
		if !model.IsActive {
			continue
		}
		h.checkModelDrift(ctx, dsID, model.ID, freshColumns, freshTables)
	}
}

// checkModelDrift runs drift detection for a single model and persists/notifies
// when drift is found.
func (h *DatasourceHandler) checkModelDrift(ctx context.Context, dsID, modelID string, freshColumns []metadata.Column, freshTables []metadata.Table) {
	fullModel, err := h.deps.SemanticRepo.GetFullModel(ctx, modelID)
	if err != nil {
		slog.ErrorContext(ctx, "failed to fetch full model for drift check", "model_id", modelID, "error", err)
		return
	}

	report, err := h.deps.DriftDetector.Compare(ctx, *fullModel, freshColumns, freshTables)
	if err != nil {
		slog.ErrorContext(ctx, "drift comparison failed", "model_id", modelID, "error", err)
		return
	}
	if report == nil || len(report.Drifts) == 0 {
		return
	}

	// A report identical to the latest one — resolved or not — is not news:
	// re-inserting it would resurrect drifts the user already acknowledged.
	// Fail open on lookup errors so a broken lookup never silences alerts.
	latest, err := h.deps.DriftRepo.GetLatestByModel(ctx, modelID)
	if err != nil && !errors.Is(err, drift.ErrNoDriftReport) {
		slog.ErrorContext(ctx, "failed to fetch latest drift report", "model_id", modelID, "error", err)
	}
	if latest != nil && drift.DriftsMatch(latest.Drifts, report.Drifts) {
		return
	}

	if err := h.deps.DriftRepo.InsertReport(ctx, report); err != nil {
		slog.ErrorContext(ctx, "failed to insert drift report", "model_id", modelID, "error", err)
		return
	}

	if report.Severity == drift.SeverityCritical || report.Severity == drift.SeverityWarning {
		h.emitDriftNotification(ctx, dsID, modelID, report, fullModel)
	}
}

// emitDriftNotification records a drift audit event and dispatches an owner
// notification (best-effort, bounded by driftNotifySem backpressure).
func (h *DatasourceHandler) emitDriftNotification(ctx context.Context, dsID, modelID string, report *drift.DriftReport, fullModel *semantic.SemanticModel) {
	driftSummaries := make([]string, 0, len(report.Drifts))
	for _, item := range report.Drifts {
		driftSummaries = append(driftSummaries, fmt.Sprintf("%s: %s", item.Type, item.Description))
	}

	h.deps.AuditLogger.Log(ctx, audit.Event{
		EventType:    audit.EventDriftDetected,
		DatasourceID: dsID,
		ModelID:      modelID,
		Details: map[string]any{
			"drift_count": len(report.Drifts),
			"severity":    report.Severity,
			"summary":     strings.Join(driftSummaries, "; "),
		},
	})

	select {
	case driftNotifySem <- struct{}{}:
		go func(rep *drift.DriftReport, mName string, creator *string, parent context.Context) {
			defer func() { <-driftNotifySem }()
			notifyCtx, cancel := context.WithTimeout(context.WithoutCancel(parent), 15*time.Second)
			defer cancel()

			var frontendURL string
			if h.deps.Config != nil {
				frontendURL = h.deps.Config.Mail.FrontendURL
			}

			if err := h.deps.DriftNotifier.NotifyOwner(notifyCtx, rep, mName, creator, frontendURL); err != nil {
				slog.Error("failed to notify owner about schema drift", "model_id", rep.ModelID, "error", err)
			}
		}(report, fullModel.Name, fullModel.CreatedBy, ctx)
	default:
		slog.Warn("drift notification skipped due to backpressure", "model_id", report.ModelID)
	}
}

// appendPostSyncPIIScan runs the post-sync PII scan (opt-out via
// ?scan_pii=false or config) and attaches the summary to the sync response.
// Scan failures don't fail the sync: metadata is already persisted here.
func (h *DatasourceHandler) appendPostSyncPIIScan(ctx context.Context, r *http.Request, resolved *app.ResolvedDatasource, response map[string]any) {
	autoScan := piiEnabled(h.deps) && (h.deps.Config == nil || h.deps.Config.PII.AutoScanOnSync)
	if !autoScan || r.URL.Query().Get("scan_pii") == "false" {
		return
	}
	summary, err := runPIIScan(ctx, h.deps, resolved)
	if err != nil {
		slog.ErrorContext(ctx, "post-sync pii scan failed", "datasource_id", resolved.Record.ID, "error", err)
		return
	}
	response["pii_scan"] = summary
}
