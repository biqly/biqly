package metadata

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/biqly/biqly/pkg/logicalquery"
	pkgquery "github.com/biqly/biqly/pkg/query"
	"github.com/stretchr/testify/assert"
)

// --- Custom SQL Mock Driver for Repository Tests ---

type mockCall struct {
	Op   string
	Args []driver.Value
}

type queryMock struct {
	Pattern string // substring match on query
	Cols    []string
	Rows    [][]driver.Value
	Err     error
}

type execMock struct {
	Pattern      string
	LastInsertID int64
	RowsAffected int64
	Err          error
}

type mockDBState struct {
	mu          sync.Mutex
	calls       []mockCall
	queries     []queryMock
	execs       []execMock
	defaultRows *mockRows
}

func (s *mockDBState) logCall(op string, args []driver.Value) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, mockCall{Op: s.simplifyQuery(op), Args: args})
}

func (*mockDBState) simplifyQuery(q string) string {
	return strings.Join(strings.Fields(q), " ")
}

func (s *mockDBState) nextQueryRows(query string, _ []driver.Value) (driver.Rows, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	qNormalized := s.simplifyQuery(query)
	for _, qm := range s.queries {
		if strings.Contains(strings.ToLower(qNormalized), strings.ToLower(qm.Pattern)) {
			if qm.Err != nil {
				return nil, qm.Err
			}
			return &mockRows{cols: qm.Cols, rows: qm.Rows, pos: 0}, nil
		}
	}
	if s.defaultRows != nil {
		return s.defaultRows, nil
	}
	return nil, fmt.Errorf("no mock query matched: %s", query)
}

func (s *mockDBState) nextExecResult(query string, _ []driver.Value) (driver.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	qNormalized := s.simplifyQuery(query)
	for _, em := range s.execs {
		if strings.Contains(strings.ToLower(qNormalized), strings.ToLower(em.Pattern)) {
			if em.Err != nil {
				return nil, em.Err
			}
			return driver.RowsAffected(em.RowsAffected), nil
		}
	}
	return driver.RowsAffected(1), nil
}

type mockRepositoryDriver struct {
	mu     sync.Mutex
	states map[string]*mockDBState
}

func (d *mockRepositoryDriver) Open(name string) (driver.Conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	state, ok := d.states[name]
	if !ok {
		return nil, fmt.Errorf("no mock db state registered for %s", name)
	}
	return &mockConn{db: state}, nil
}

var mDriver = &mockRepositoryDriver{states: make(map[string]*mockDBState)}

func init() {
	sql.Register("metadata_mock", mDriver)
}

func setupMockDB(t *testing.T) (*sql.DB, *mockDBState) {
	name := fmt.Sprintf("db-%d-%s", time.Now().UnixNano(), t.Name())
	state := &mockDBState{}
	mDriver.mu.Lock()
	mDriver.states[name] = state
	mDriver.mu.Unlock()

	db, err := sql.Open("metadata_mock", name)
	if err != nil {
		t.Fatalf("failed to open mock db: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close mock db: %v", err)
		}
		mDriver.mu.Lock()
		delete(mDriver.states, name)
		mDriver.mu.Unlock()
	})
	return db, state
}

type mockConn struct {
	db *mockDBState
}

func (c *mockConn) Prepare(query string) (driver.Stmt, error) {
	return &mockStmt{conn: c, query: query}, nil
}

func (*mockConn) Close() error {
	return nil
}

func (c *mockConn) Begin() (driver.Tx, error) {
	c.db.logCall("BEGIN", nil)
	return &mockTx{conn: c}, nil
}

func (c *mockConn) BeginTx(_ context.Context, _ driver.TxOptions) (driver.Tx, error) {
	c.db.logCall("BEGIN TX", nil)
	return &mockTx{conn: c}, nil
}

func (c *mockConn) QueryContext(_ context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	vals := make([]driver.Value, len(args))
	for i, arg := range args {
		vals[i] = arg.Value
	}
	c.db.logCall("QUERYContext: "+query, vals)
	return c.db.nextQueryRows(query, vals)
}

func (c *mockConn) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	vals := make([]driver.Value, len(args))
	for i, arg := range args {
		vals[i] = arg.Value
	}
	c.db.logCall("EXECContext: "+query, vals)
	return c.db.nextExecResult(query, vals)
}

type mockStmt struct {
	conn  *mockConn
	query string
}

func (*mockStmt) Close() error {
	return nil
}

func (*mockStmt) NumInput() int {
	return -1
}

func (s *mockStmt) Exec(args []driver.Value) (driver.Result, error) {
	s.conn.db.logCall("EXEC: "+s.query, args)
	return s.conn.db.nextExecResult(s.query, args)
}

func (s *mockStmt) Query(args []driver.Value) (driver.Rows, error) {
	s.conn.db.logCall("QUERY: "+s.query, args)
	return s.conn.db.nextQueryRows(s.query, args)
}

type mockTx struct {
	conn *mockConn
}

func (tx *mockTx) Commit() error {
	tx.conn.db.logCall("COMMIT", nil)
	return nil
}

func (tx *mockTx) Rollback() error {
	tx.conn.db.logCall("ROLLBACK", nil)
	return nil
}

type mockRows struct {
	cols []string
	rows [][]driver.Value
	pos  int
}

func (r *mockRows) Columns() []string {
	return r.cols
}

func (*mockRows) Close() error {
	return nil
}

func (r *mockRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.rows) {
		return io.EOF
	}
	row := r.rows[r.pos]
	for i := range dest {
		if i < len(row) {
			dest[i] = row[i]
		} else {
			dest[i] = nil
		}
	}
	r.pos++
	return nil
}

// --- Datasources Operations Tests ---

func TestCreateDatasource(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	ds := &Datasource{
		ID:                "ds-1",
		Name:              "My Postgres",
		Type:              "postgres",
		DSNEncrypted:      "enc-dsn",
		Config:            `{"ssl": true}`,
		IsActive:          true,
		Host:              func() *string { s := "localhost"; return &s }(),
		Port:              func() *int { p := 5432; return &p }(),
		Username:          func() *string { s := "postgres"; return &s }(),
		PasswordEncrypted: "enc-pass",
		DatabaseName:      func() *string { s := "mydb"; return &s }(),
		SSLMode:           func() *string { s := "require"; return &s }(),
		ConnectionParams:  []byte(`{"timeout":10}`),
		DSNMode:           DSNModeRaw,
	}

	state.execs = []execMock{
		{Pattern: "INSERT INTO datasources", RowsAffected: 1},
	}

	err := repo.CreateDatasource(ctx, ds)
	assert.NoError(t, err)

	state.mu.Lock()
	assert.Len(t, state.calls, 1)
	assert.Contains(t, state.calls[0].Op, "INSERT INTO datasources")
	state.mu.Unlock()
}

func TestUpdateDatasource(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	//nolint:gosec // fixture values for repository update test
	ds := &Datasource{
		ID:                "ds-1",
		Name:              "Updated MySQL",
		Type:              "mysql",
		DSNEncrypted:      "new-enc-dsn",
		Config:            `{"param": 42}`,
		IsActive:          false,
		Host:              func() *string { s := "127.0.0.1"; return &s }(),
		Port:              func() *int { p := 3306; return &p }(),
		Username:          func() *string { s := "root"; return &s }(),
		PasswordEncrypted: "new-enc-pass",
		DatabaseName:      func() *string { s := "inventory"; return &s }(),
		SSLMode:           nil,
		ConnectionParams:  nil,
		DSNMode:           DSNModeStructured,
	}

	state.execs = []execMock{
		{Pattern: "UPDATE datasources SET", RowsAffected: 1},
	}

	err := repo.UpdateDatasource(ctx, ds)
	assert.NoError(t, err)

	state.mu.Lock()
	assert.Len(t, state.calls, 1)
	assert.Contains(t, state.calls[0].Op, "UPDATE datasources SET")
	state.mu.Unlock()
}

func TestGetDatasource(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	now := time.Now()
	state.queries = []queryMock{
		{
			Pattern: "FROM datasources WHERE id = $1",
			Cols:    []string{"id", "name", "type", "dsn_encrypted", "config", "is_active", "last_sync_at", "created_at", "updated_at", "host", "port", "username", "password_encrypted", "database_name", "ssl_mode", "connection_params", "dsn_mode"},
			Rows: [][]driver.Value{
				{"ds-1", "My SQL Server", "sqlserver", "enc", "{}", true, now, now, now, "sql.host", int64(1433), "sa", "pass", "master", "disable", []byte(`{}`), "raw"},
			},
		},
	}

	ds, err := repo.GetDatasource(ctx, "ds-1")
	assert.NoError(t, err)
	assert.NotNil(t, ds)
	assert.Equal(t, "ds-1", ds.ID)
	assert.Equal(t, "My SQL Server", ds.Name)
	assert.Equal(t, "sqlserver", ds.Type)
	assert.Equal(t, "enc", ds.DSNEncrypted)
}

func TestGetDatasourceByName(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	now := time.Now()
	state.queries = []queryMock{
		{
			Pattern: "FROM datasources WHERE name = $1",
			Cols:    []string{"id", "name", "type", "dsn_encrypted", "config", "is_active", "last_sync_at", "created_at", "updated_at", "host", "port", "username", "password_encrypted", "database_name", "ssl_mode", "connection_params", "dsn_mode"},
			Rows: [][]driver.Value{
				{"ds-2", "Clickhouse DS", "clickhouse", "encch", "{}", true, now, now, now, "ch.host", int64(8123), "default", "", "default", "", []byte(`{}`), "raw"},
			},
		},
	}

	ds, err := repo.GetDatasourceByName(ctx, "Clickhouse DS")
	assert.NoError(t, err)
	assert.NotNil(t, ds)
	assert.Equal(t, "ds-2", ds.ID)
	assert.Equal(t, "Clickhouse DS", ds.Name)
}

func TestListDatasources(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	now := time.Now()
	state.queries = []queryMock{
		{
			Pattern: "FROM datasources",
			Cols:    []string{"id", "name", "type", "dsn_encrypted", "config", "is_active", "last_sync_at", "created_at", "updated_at", "host", "port", "username", "password_encrypted", "database_name", "ssl_mode", "connection_params", "dsn_mode"},
			Rows: [][]driver.Value{
				{"id-1", "DS 1", "postgres", "enc1", "{}", true, now, now, now, "host1", int64(5432), "user1", "p1", "db1", "req1", []byte(`{}`), "raw"},
				{"id-2", "DS 2", "mysql", "enc2", "{}", false, nil, now, now, "host2", int64(3306), "user2", "p2", "db2", "req2", nil, "structured"},
			},
		},
	}

	list, err := repo.ListDatasources(ctx)
	assert.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Equal(t, "id-1", list[0].ID)
	assert.Equal(t, "id-2", list[1].ID)
}

func TestDeleteDatasource(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "DELETE FROM datasources WHERE id", RowsAffected: 1},
	}

	err := repo.DeleteDatasource(ctx, "ds-1")
	assert.NoError(t, err)
}

func TestUpdateDatasourceSync(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "UPDATE datasources SET last_sync_at", RowsAffected: 1},
	}

	err := repo.UpdateDatasourceSync(ctx, "ds-1")
	assert.NoError(t, err)
}

// --- Schemas, Tables and Columns operations Tests ---

func TestUpsertSchemas_And_Schema(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{Pattern: "INSERT INTO schemas", Cols: []string{"id"}, Rows: [][]driver.Value{{"sch-123"}}},
	}

	// 1. Single UpsertSchema
	s := Schema{
		ID:           "s-1",
		DatasourceID: "ds-1",
		SchemaName:   "public",
	}
	id, err := repo.UpsertSchema(ctx, "ds-1", s)
	assert.NoError(t, err)
	assert.Equal(t, "sch-123", id)

	// 2. Bulk UpsertSchemas
	err = repo.UpsertSchemas(ctx, "ds-1", []Schema{s})
	assert.NoError(t, err)
}

func TestUpsertTables_And_Table(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.queries = []queryMock{
		{Pattern: "INSERT INTO tables", Cols: []string{"id"}, Rows: [][]driver.Value{{"tbl-123"}}},
	}

	rowEst := int64(1000)
	desc := "My description"
	t1 := Table{
		ID:           "t-1",
		DatasourceID: "ds-1",
		SchemaID:     "sch-123",
		SchemaName:   "public",
		TableName:    "users",
		TableType:    "BASE TABLE",
		RowEstimate:  &rowEst,
		Description:  &desc,
	}

	// 1. Single UpsertTable
	id, err := repo.UpsertTable(ctx, "ds-1", t1)
	assert.NoError(t, err)
	assert.Equal(t, "tbl-123", id)

	// 2. Bulk UpsertTables
	err = repo.UpsertTables(ctx, "ds-1", []Table{t1})
	assert.NoError(t, err)
}

func TestListTables_And_GetTable(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	now := time.Now()
	rowEst := int64(500)
	desc := "test desk"
	lbl := "Users Label"
	state.queries = []queryMock{
		{
			Pattern: "SELECT id, datasource_id, schema_id, schema_name, table_name, table_type, row_estimate, description, label",
			Cols:    []string{"id", "datasource_id", "schema_id", "schema_name", "table_name", "table_type", "row_estimate", "description", "label", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"t-1", "ds-1", "sch-1", "public", "users", "BASE TABLE", rowEst, desc, lbl, now, now},
			},
		},
	}

	// Test ListTables with schema filter
	tables, err := repo.ListTables(ctx, "ds-1", "public")
	assert.NoError(t, err)
	assert.Len(t, tables, 1)
	assert.Equal(t, "users", tables[0].TableName)

	// Test GetTable
	tbl, err := repo.GetTable(ctx, "t-1")
	assert.NoError(t, err)
	assert.NotNil(t, tbl)
	assert.Equal(t, "users", tbl.TableName)
}

func TestUpdateTableDescriptionAndLabel(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "UPDATE tables SET description =", RowsAffected: 1},
		{Pattern: "UPDATE tables SET label =", RowsAffected: 1},
		{Pattern: "UPDATE tables SET description = $2, label = $3", RowsAffected: 1},
	}

	newDesc := "New description"
	newLabel := "New Label"

	err := repo.UpdateTableDescription(ctx, "t-1", &newDesc)
	assert.NoError(t, err)

	err = repo.UpdateTableLabel(ctx, "t-1", &newLabel)
	assert.NoError(t, err)

	err = repo.UpdateTableDescriptionAndLabel(ctx, "t-1", &newDesc, &newLabel)
	assert.NoError(t, err)
}

func TestTableEmbeddings(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "UPDATE tables SET embedding =", RowsAffected: 1},
	}

	state.queries = []queryMock{
		{
			Pattern: "SELECT embedding, embedding_model FROM tables WHERE id",
			Cols:    []string{"embedding", "embedding_model"},
			Rows: [][]driver.Value{
				{[]byte(`[0.1, 0.2, 0.3]`), "text-embedding-3-small"},
			},
		},
		{
			Pattern: "SELECT schema_name, table_name, embedding_model, embedding FROM tables",
			Cols:    []string{"schema_name", "table_name", "embedding_model", "embedding"},
			Rows: [][]driver.Value{
				{"public", "users", "text-embedding-3-small", []byte(`[0.1, 0.2, 0.3]`)},
			},
		},
	}

	err := repo.UpsertTableEmbedding(ctx, "t-1", "text-embedding-3-small", []float32{0.1, 0.2, 0.3})
	assert.NoError(t, err)

	embs, err := repo.ListTableEmbeddings(ctx, "ds-1")
	assert.NoError(t, err)
	assert.Len(t, embs, 1)
	assert.Equal(t, "users", embs[0].TableName)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, embs[0].Embedding)
}

func TestUpsertColumns_And_GetColumn(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "INSERT INTO columns", RowsAffected: 5},
	}

	colCount := 1
	charMax := 100
	prec := 10
	scale := 2
	defVal := "default_val"
	desc := "column desc"

	c := Column{
		ID:               "col-1",
		DatasourceID:     "ds-1",
		TableID:          "t-1",
		SchemaName:       "public",
		TableName:        "users",
		ColumnName:       "username",
		DataType:         "varchar",
		Nullable:         true,
		OrdinalPosition:  &colCount,
		CharMaxLength:    &charMax,
		NumericPrecision: &prec,
		NumericScale:     &scale,
		ColumnDefault:    &defVal,
		Description:      &desc,
		IsPrimaryKey:     true,
		IsForeignKey:     false,
	}

	err := repo.UpsertColumns(ctx, "ds-1", []Column{c})
	assert.NoError(t, err)

	now := time.Now()
	state.queries = []queryMock{
		{
			Pattern: "SELECT id, datasource_id, table_id, schema_name, table_name, column_name, data_type, nullable, ordinal_position, character_maximum_length, numeric_precision, numeric_scale, column_default, description, is_primary_key, is_foreign_key, referenced_schema, referenced_table, referenced_column, created_at, pii_type, pii_confidence, pii_detected_at, pii_reviewed_by, pii_masking_strategy FROM columns",
			Cols:    []string{"id", "datasource_id", "table_id", "schema_name", "table_name", "column_name", "data_type", "nullable", "ordinal_position", "character_maximum_length", "numeric_precision", "numeric_scale", "column_default", "description", "is_primary_key", "is_foreign_key", "referenced_schema", "referenced_table", "referenced_column", "created_at", "pii_type", "pii_confidence", "pii_detected_at", "pii_reviewed_by", "pii_masking_strategy"},
			Rows: [][]driver.Value{
				{"col-1", "ds-1", "t-1", "public", "users", "username", "varchar", true, colCount, charMax, prec, scale, defVal, desc, true, false, nil, nil, nil, now, nil, nil, nil, nil, nil},
			},
		},
	}

	col, err := repo.GetColumn(ctx, "col-1")
	assert.NoError(t, err)
	assert.NotNil(t, col)
	assert.Equal(t, "username", col.ColumnName)

	columns, err := repo.ListColumns(ctx, "ds-1", "public", "users")
	assert.NoError(t, err)
	assert.Len(t, columns, 1)
	assert.Equal(t, "username", columns[0].ColumnName)
}

func TestUpdateColumnDescription_And_Embeddings(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	state.execs = []execMock{
		{Pattern: "UPDATE columns SET description =", RowsAffected: 1},
		{Pattern: "UPDATE columns SET embedding =", RowsAffected: 1},
	}

	state.queries = []queryMock{
		{
			Pattern: "SELECT embedding, embedding_model FROM columns WHERE id",
			Cols:    []string{"embedding", "embedding_model"},
			Rows: [][]driver.Value{
				{[]byte(`[0.1, 0.2, 0.3]`), "text-embedding-3-small"},
			},
		},
		{
			Pattern: "SELECT schema_name, table_name, column_name, embedding_model, embedding FROM columns",
			Cols:    []string{"schema_name", "table_name", "column_name", "embedding_model", "embedding"},
			Rows: [][]driver.Value{
				{"public", "users", "username", "text-embedding-3-small", []byte(`[0.1, 0.2, 0.3]`)},
			},
		},
	}

	newDesc := "New description"
	err := repo.UpdateColumnDescription(ctx, "col-1", &newDesc)
	assert.NoError(t, err)

	err = repo.UpsertColumnEmbedding(ctx, "col-1", "text-embedding-3-small", []float32{0.1, 0.2, 0.3})
	assert.NoError(t, err)

	embs, err := repo.ListColumnEmbeddings(ctx, "ds-1")
	assert.NoError(t, err)
	assert.Len(t, embs, 1)
	assert.Equal(t, "username", embs[0].ColumnName)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, embs[0].Embedding)
}

func TestSearchColumns_And_Tables(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	now := time.Now()
	rowEst := int64(100)
	desc := "my matches users"

	state.queries = []queryMock{
		{
			Pattern: "FROM columns",
			Cols:    []string{"id", "datasource_id", "table_id", "schema_name", "table_name", "column_name", "data_type", "nullable", "ordinal_position", "character_maximum_length", "numeric_precision", "numeric_scale", "column_default", "description", "is_primary_key", "is_foreign_key", "referenced_schema", "referenced_table", "referenced_column", "created_at", "pii_type", "pii_confidence", "pii_detected_at", "pii_reviewed_by", "pii_masking_strategy"},
			Rows: [][]driver.Value{
				{"col-1", "ds-1", "t-1", "public", "users", "username", "varchar", true, 1, 100, 10, 2, "df", desc, true, false, nil, nil, nil, now, nil, nil, nil, nil, nil},
			},
		},
		{
			Pattern: "FROM tables",
			Cols:    []string{"id", "datasource_id", "schema_id", "schema_name", "table_name", "table_type", "row_estimate", "description", "label", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"t-1", "ds-1", "sch-1", "public", "users", "BASE TABLE", rowEst, desc, "lbl", now, now},
			},
		},
	}

	cols, err := repo.SearchColumns(ctx, "ds-1", "users")
	assert.NoError(t, err)
	assert.Len(t, cols, 1)
	assert.Equal(t, "username", cols[0].ColumnName)

	tbls, err := repo.SearchTables(ctx, "ds-1", "users")
	assert.NoError(t, err)
	assert.Len(t, tbls, 1)
	assert.Equal(t, "users", tbls[0].TableName)
}

func TestRelations_And_Permissions(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	now := time.Now()

	state.execs = []execMock{
		{Pattern: "INSERT INTO relations", RowsAffected: 1},
	}

	state.queries = []queryMock{
		{
			Pattern: "SELECT id, datasource_id, constraint_name, from_schema, from_table, from_column",
			Cols:    []string{"id", "datasource_id", "constraint_name", "from_schema", "from_table", "from_column", "to_schema", "to_table", "to_column", "relationship_type", "created_at"},
			Rows: [][]driver.Value{
				{"rel-1", "ds-1", "fk_users_orgs", "public", "users", "org_id", "public", "orgs", "id", "many_to_one", now},
			},
		},
		{
			Pattern: "FROM permissions",
			Cols:    []string{"denied_fields", "row_filters"},
			Rows: [][]driver.Value{
				{`{salary,ssn}`, []byte(`[{"field":"country = 'US'"}]`)},
			},
		},
	}

	relations := []Relation{
		{
			ID:               "rel-1",
			ConstraintName:   "fk_users_orgs",
			FromSchema:       "public",
			FromTable:        "users",
			FromColumn:       "org_id",
			ToSchema:         "public",
			ToTable:          "orgs",
			ToColumn:         "id",
			RelationshipType: "many_to_one",
		},
	}

	err := repo.UpsertRelations(ctx, "ds-1", relations)
	assert.NoError(t, err)

	list, err := repo.ListRelations(ctx, "ds-1")
	assert.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "fk_users_orgs", list[0].ConstraintName)

	policies, err := repo.ListPermissionPolicies(ctx, "ds-1")
	assert.NoError(t, err)
	assert.Len(t, policies, 1)
	assert.Equal(t, []string{"salary", "ssn"}, policies[0].DeniedFields)
	assert.NotNil(t, policies[0].RowFilters)
}

func TestQueryAndAIHistory(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	now := time.Now()

	state.queries = []queryMock{
		{
			Pattern: "INSERT INTO query_history",
			Cols:    []string{"id", "created_at"},
			Rows: [][]driver.Value{
				{"qh-123", now},
			},
		},
		{
			Pattern: "SELECT id, datasource_id, model_id, user_id, logical_query, compiled_sql, sql_args, status, row_count, duration_ms, error_message, query_fingerprint, created_at FROM query_history WHERE id =",
			Cols:    []string{"id", "datasource_id", "model_id", "user_id", "logical_query", "compiled_sql", "sql_args", "status", "row_count", "duration_ms", "error_message", "query_fingerprint", "created_at"},
			Rows: [][]driver.Value{
				{"qh-123", "ds-1", "m-1", "u-1", []byte(`{"version":"v1"}`), "SELECT 1", "[]", "success", int64(10), int64(250), nil, "fp", now},
			},
		},
		{
			Pattern: "SELECT id, datasource_id, model_id, user_id, logical_query, compiled_sql, sql_args, status, row_count, duration_ms, error_message, query_fingerprint, created_at FROM query_history",
			Cols:    []string{"id", "datasource_id", "model_id", "user_id", "logical_query", "compiled_sql", "sql_args", "status", "row_count", "duration_ms", "error_message", "query_fingerprint", "created_at"},
			Rows: [][]driver.Value{
				{"qh-123", "ds-1", "m-1", "u-1", []byte(`{"version":"v1"}`), "SELECT 1", "[]", "success", int64(10), int64(250), nil, "fp", now},
			},
		},
		{
			Pattern: "SELECT question, logical_query FROM ai_query_history",
			Cols:    []string{"question", "logical_query"},
			Rows: [][]driver.Value{
				{"how many users?", []byte(`{"version":"v1"}`)},
			},
		},
		{
			Pattern: "INSERT INTO ai_query_history",
			Cols:    []string{"id", "created_at"},
			Rows: [][]driver.Value{
				{"aqh-123", now},
			},
		},
		{
			Pattern: "SELECT id, datasource_id, model_id, user_id, question, prompt_context",
			Cols:    []string{"id", "datasource_id", "model_id", "user_id", "question", "prompt_context", "ai_response", "logical_query", "confidence_score", "warnings", "outcome_status", "retry_count", "needs_clarification", "model_used", "prompt_tokens", "completion_tokens", "token_count", "cost_usd", "latency_ms", "created_at", "ab_experiment_id", "ab_variant_id"},
			Rows: [][]driver.Value{
				{"aqh-123", "ds-1", "m-1", "u-1", "how many users?", []byte(`{}`), []byte(`{}`), []byte(`{"version":"v1"}`), 0.95, `{warn1}`, "success", int64(0), false, "gpt-4", int64(6), int64(4), int64(10), 0.05, int64(120), now, nil, nil},
			},
		},
	}

	// 1. CreateQueryHistory
	entry := &pkgquery.HistoryEntry{
		DatasourceID: "ds-1",
		ModelID:      new("m-1"),
		UserID:       new("u-1"),
		LogicalQuery: logicalquery.LogicalQuery{Version: "v1"},
		CompiledSQL:  new("SELECT 1"),
		SQLArgs:      new("[]"),
		Status:       "success",
		RowCount:     new(10),
		DurationMs:   new(250),
		Fingerprint:  "fp",
	}
	err := repo.CreateQueryHistory(ctx, entry)
	assert.NoError(t, err)
	assert.Equal(t, "qh-123", entry.ID)

	// 2. ListQueryHistory
	historyList, err := repo.ListQueryHistory(ctx, 10)
	assert.NoError(t, err)
	assert.Len(t, historyList, 1)
	assert.Equal(t, "qh-123", historyList[0].ID)

	// 3. GetQueryHistory
	historyEntry, err := repo.GetQueryHistory(ctx, "qh-123")
	assert.NoError(t, err)
	assert.Equal(t, "qh-123", historyEntry.ID)

	// 4. ListSuccessfulAIQueries
	successfulQueries, err := repo.ListSuccessfulAIQueries(ctx, "ds-1", new("m-1"), 5)
	assert.NoError(t, err)
	assert.Len(t, successfulQueries, 1)
	assert.Equal(t, "how many users?", successfulQueries[0].Question)

	// 5. CreateAIQueryHistory
	aiEntry := &AIQueryHistoryEntry{
		DatasourceID:       "ds-1",
		ModelID:            new("m-1"),
		UserID:             new("u-1"),
		Question:           "how many users?",
		PromptContext:      map[string]any{},
		AIResponse:         map[string]any{},
		LogicalQuery:       map[string]any{"version": "v1"},
		ConfidenceScore:    new(0.95),
		Warnings:           []string{"warn1"},
		OutcomeStatus:      "success",
		RetryCount:         0,
		NeedsClarification: false,
		ModelUsed:          new("gpt-4"),
		TokenCount:         new(10),
		CostUSD:            new(0.05),
		LatencyMs:          new(120),
	}
	err = repo.CreateAIQueryHistory(ctx, aiEntry)
	assert.NoError(t, err)
	assert.Equal(t, "aqh-123", aiEntry.ID)

	// 6. ListAIQueryHistory
	aiHistoryList, err := repo.ListAIQueryHistory(ctx, "u-1", 10)
	assert.NoError(t, err)
	assert.Len(t, aiHistoryList, 1)
	assert.Equal(t, "aqh-123", aiHistoryList[0].ID)
}

//nolint:funlen
func TestTimeGrains_PromptTemplates_And_BusinessGlossary(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	now := time.Now()

	state.execs = []execMock{
		{Pattern: "UPDATE ai_time_grains", RowsAffected: 1},
		{Pattern: "INSERT INTO ai_time_grains", RowsAffected: 1},
		{Pattern: "UPDATE ai_prompt_templates", RowsAffected: 1},
		{Pattern: "INSERT INTO ai_prompt_templates", RowsAffected: 1},
		{Pattern: "DELETE FROM ai_prompt_templates", RowsAffected: 1},
		{Pattern: "UPDATE business_glossary_terms", RowsAffected: 1},
		{Pattern: "DELETE FROM business_glossary_terms", RowsAffected: 1},
	}

	state.queries = []queryMock{
		{
			Pattern: "SELECT COUNT(*) FROM ai_time_grains",
			Cols:    []string{"count"},
			Rows: [][]driver.Value{
				{int64(4)},
			},
		},
		{
			Pattern: "SELECT grain, suffix, requires_time, synonyms, created_at, updated_at FROM ai_time_grains",
			Cols:    []string{"grain", "suffix", "requires_time", "synonyms", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"day", "_day", false, `{gün,gül}`, now, now},
			},
		},
		{
			Pattern: "SELECT COUNT(*) FROM ai_prompt_templates",
			Cols:    []string{"count"},
			Rows: [][]driver.Value{
				{int64(2)},
			},
		},
		{
			Pattern: "SELECT content, version FROM ai_prompt_templates",
			Cols:    []string{"content", "version"},
			Rows: [][]driver.Value{
				{"prompt context value", int64(1)},
			},
		},
		{
			Pattern: "SELECT COALESCE(MAX(version), 0) + 1 FROM ai_prompt_templates",
			Cols:    []string{"next_version"},
			Rows: [][]driver.Value{
				{int64(2)},
			},
		},
		{
			Pattern: "SELECT name, locale, version, content, is_active, created_at, updated_at FROM ai_prompt_templates",
			Cols:    []string{"name", "locale", "version", "content", "is_active", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"system_prompt", "en", int64(1), "prompt content context", true, now, now},
			},
		},
		{
			Pattern: "SELECT id::text, datasource_id::text, COALESCE(model_id::text, ''), term, COALESCE(definition, ''), maps_to_type, maps_to_name, COALESCE(aliases, '{}'), is_active, created_at, updated_at FROM business_glossary_terms",
			Cols:    []string{"id", "datasource_id", "model_id", "term", "definition", "maps_to_type", "maps_to_name", "aliases", "is_active", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"bg-1", "ds-1", "m-1", "musteri", "user accounts table", "table", "users", `{customer,client}`, true, now, now},
			},
		},
		{
			Pattern: "INSERT INTO business_glossary_terms",
			Cols:    []string{"id"},
			Rows: [][]driver.Value{
				{"bg-1"},
			},
		},
	}

	// --- 1. Time Grains ---
	tgCount, err := repo.CountTimeGrains(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 4, tgCount)

	tgList, err := repo.ListTimeGrains(ctx)
	assert.NoError(t, err)
	assert.Len(t, tgList, 1)
	assert.Equal(t, "day", tgList[0].Grain)
	assert.Equal(t, []string{"gün", "gül"}, tgList[0].Synonyms)

	err = repo.UpdateTimeGrain(ctx, TimeGrain{Grain: "day", Suffix: "_day", RequiresTime: false, Synonyms: []string{"gün", "gül"}})
	assert.NoError(t, err)

	err = repo.UpsertTimeGrain(ctx, TimeGrain{Grain: "day", Suffix: "_day", RequiresTime: false, Synonyms: []string{"gün", "gül"}})
	assert.NoError(t, err)

	// --- 2. Prompt Templates ---
	ptCount, err := repo.CountPromptTemplates(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 2, ptCount)

	ptContent, err := repo.GetPromptTemplate(ctx, "system_prompt", "en")
	assert.NoError(t, err)
	assert.Equal(t, "prompt context value", ptContent)

	err = repo.UpsertPromptTemplate(ctx, "system_prompt", "en", "new prompt instruction")
	assert.NoError(t, err)

	ptList, err := repo.ListPromptTemplates(ctx)
	assert.NoError(t, err)
	assert.Len(t, ptList, 1)
	assert.Equal(t, "system_prompt", ptList[0].Name)

	err = repo.DeleteAllPromptTemplates(ctx)
	assert.NoError(t, err)

	// --- 3. Business Glossary ---
	bgList, err := repo.ListBusinessGlossary(ctx, "ds-1", "m-1")
	assert.NoError(t, err)
	assert.Len(t, bgList, 1)
	assert.Equal(t, "musteri", bgList[0].Term)
	assert.Equal(t, []string{"customer", "client"}, bgList[0].Aliases)

	bgID, err := repo.InsertBusinessGlossary(ctx, BusinessGlossaryInsert{
		DatasourceID: "ds-1",
		ModelID:      "m-1",
		Term:         "musteri",
		Definition:   "user accounts table",
		MapsToType:   "table",
		MapsToName:   "users",
		Aliases:      []string{"customer", "client"},
	})
	assert.NoError(t, err)
	assert.Equal(t, "bg-1", bgID)

	err = repo.UpdateBusinessGlossary(ctx, "bg-1", BusinessGlossaryUpdate{
		Term:       "musteri_updated",
		Definition: "updated_definition",
		MapsToType: "table",
		MapsToName: "users",
		Aliases:    []string{"customer"},
		IsActive:   new(true),
	})
	assert.NoError(t, err)

	ok, err := repo.DeleteBusinessGlossary(ctx, "bg-1")
	assert.NoError(t, err)
	assert.True(t, ok)
}

//nolint:funlen
func TestCuratedAI(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	now := time.Now()

	state.execs = []execMock{
		{Pattern: "DELETE FROM few_shot_examples WHERE id", RowsAffected: 1},
		{Pattern: "UPDATE few_shot_examples SET", RowsAffected: 1},
		{Pattern: "INSERT INTO ai_feedback", RowsAffected: 1},
		{Pattern: "UPDATE ai_query_history SET user_rating =", RowsAffected: 1},
	}

	state.queries = []queryMock{
		{
			Pattern: "WHERE is_favorite",
			Cols:    []string{"id", "datasource_id", "model_id", "question", "logical_query", "tags", "dialect", "locale", "created_by", "created_at", "updated_at", "name", "description", "is_few_shot", "is_favorite"},
			Rows: [][]driver.Value{
				{"fe-1", "ds-1", "m-1", "how long?", []byte(`{}`), `{tag1}`, "postgres", "en", "admin", now, now, "How Long Example", "desc", true, true},
			},
		},
		{
			Pattern: "SELECT id::text FROM few_shot_examples WHERE",
			Cols:    []string{"id"},
			Rows: [][]driver.Value{
				{"fe-1"},
			},
		},
		{
			Pattern: "SELECT id::text, datasource_id::text",
			Cols:    []string{"id", "datasource_id", "model_id", "question", "logical_query", "tags", "dialect", "locale", "created_by", "created_at", "updated_at", "name", "description", "is_few_shot", "is_favorite"},
			Rows: [][]driver.Value{
				{"fe-1", "ds-1", "m-1", "how long?", []byte(`{}`), `{tag1}`, "postgres", "en", "admin", now, now, "How Long Example", "desc", true, true},
			},
		},
		{
			Pattern: "INSERT INTO few_shot_examples",
			Cols:    []string{"id"},
			Rows: [][]driver.Value{
				{"fe-1"},
			},
		},
		{
			Pattern: "SELECT COALESCE(h.model_id::text, 'unknown')",
			Cols:    []string{"model_id", "total_queries", "success_count", "failure_count", "avg_confidence", "avg_latency_ms", "positive_count", "negative_count"},
			Rows: [][]driver.Value{
				{"gpt-4", int64(10), int64(8), int64(2), 0.9, 150.0, int64(5), int64(1)},
			},
		},
		{
			Pattern: "SELECT DATE(created_at) AS usage_date, COUNT(*)",
			Cols:    []string{"usage_date", "count", "positive", "negative", "avg_latency", "total_cost", "total_tokens"},
			Rows: [][]driver.Value{
				{now, int64(10), int64(5), int64(1), 120.0, 0.05, int64(1500)},
			},
		},
		{
			Pattern: "SELECT COUNT(*), COALESCE(AVG(CASE WHEN user_rating IS NULL THEN 0.5",
			Cols:    []string{"count", "success_rate", "avg_latency_ms", "total_cost"},
			Rows: [][]driver.Value{
				{int64(10), 0.8, 120.0, 0.05},
			},
		},
	}

	// 1. ListFewShotCurated
	curatedList, err := repo.ListFewShotCurated(ctx, "ds-1", "m-1")
	assert.NoError(t, err)
	assert.Len(t, curatedList, 1)
	assert.Equal(t, "fe-1", curatedList[0].ID)

	// 2. ListFavoriteExamples
	favList, err := repo.ListFavoriteExamples(ctx, 10)
	assert.NoError(t, err)
	assert.Len(t, favList, 1)
	assert.Equal(t, "fe-1", favList[0].ID)

	// 3. InsertFewShotCurated
	feID, err := repo.InsertFewShotCurated(ctx, FewShotCuratedInsert{
		DatasourceID: "ds-1",
		ModelID:      "m-1",
		Question:     "how long?",
		LogicalQuery: json.RawMessage(`{}`),
		Tags:         []string{"tag1"},
		Dialect:      "postgres",
		Locale:       "en",
		Name:         "How Long Example",
		Description:  "desc",
		IsFewShot:    map[bool]bool{true: true}[true],
		IsFavorite:   true,
	})
	assert.NoError(t, err)
	assert.Equal(t, "fe-1", feID)

	// 4. UpdateFewShotCurated
	err = repo.UpdateFewShotCurated(ctx, "fe-1", FewShotCuratedUpdate{
		Question:     "how long active?",
		LogicalQuery: json.RawMessage(`{}`),
		Tags:         []string{"tag1", "tag2"},
		Dialect:      "postgres",
		Locale:       "en",
		Name:         "How Long Example Updated",
		Description:  "desc-updated",
		IsFewShot:    true,
		IsFavorite:   true,
	})
	assert.NoError(t, err)

	// 5. DeleteFewShotCurated
	deleted, err := repo.DeleteFewShotCurated(ctx, "fe-1")
	assert.NoError(t, err)
	assert.True(t, deleted)

	// 6. InsertAIFeedback
	err = repo.InsertAIFeedback(ctx, "how long?", "ds-1", "positive", []string{"speed"}, "very fast")
	assert.NoError(t, err)

	// 7. UpdateLatestAIQueryHistoryRating
	err = repo.UpdateLatestAIQueryHistoryRating(ctx, "ds-1", "positive", "user-1", "how long?")
	assert.NoError(t, err)

	// 8. ListModelSuccessRates
	rates, err := repo.ListModelSuccessRates(ctx, "30")
	assert.NoError(t, err)
	assert.Len(t, rates, 1)
	assert.Equal(t, "gpt-4", rates[0].ModelID)

	// 9. GetAIUsageLast30Days
	daily, summary, err := repo.GetAIUsageLast30Days(ctx)
	assert.NoError(t, err)
	assert.Len(t, daily, 1)
	assert.Equal(t, 10, summary.TotalQueries)

	// 10. ListFewShotExampleIDs
	ids, err := repo.ListFewShotExampleIDs(ctx, "ds-1", "m-1", 5)
	assert.NoError(t, err)
	assert.Len(t, ids, 1)
	assert.Equal(t, "fe-1", ids[0])
}

//nolint:funlen
func TestAIJobs_And_AIMetrics(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	now := time.Now()
	dsID := "ds-1"

	state.execs = []execMock{
		{Pattern: "INSERT INTO ai_jobs", RowsAffected: 1},
		{Pattern: "UPDATE ai_jobs SET progress_pct =", RowsAffected: 1},
		{Pattern: "UPDATE ai_jobs SET phase =", RowsAffected: 1},
		{Pattern: "UPDATE ai_jobs SET status = 'running'", RowsAffected: 1},
		{Pattern: "UPDATE ai_jobs SET status = 'failed'", RowsAffected: 1},
		{Pattern: "UPDATE ai_jobs SET status = 'cancelled'", RowsAffected: 1},
		{Pattern: "UPDATE ai_jobs SET status = 'succeeded'", RowsAffected: 1},
		{Pattern: "UPDATE ai_jobs SET status = $2", RowsAffected: 1}, // TryMarkAIJobRunning
	}

	state.queries = []queryMock{
		{
			Pattern: "SELECT id, client_session_id, kind, status, phase, phase_message, progress_pct",
			Cols:    []string{"id", "client_session_id", "kind", "status", "phase", "phase_message", "progress_pct", "datasource_id", "scope_schemas", "progress_json", "request_json", "result_json", "error_message", "created_at", "updated_at", "started_at", "finished_at", "user_id"},
			Rows: [][]driver.Value{
				{"job-1", "sess-1", "describe", "pending", "routing", "", 5, "ds-1", "{schema1}", []byte(`{}`), []byte(`{}`), []byte(`{}`), "", now, now, now, now, nil},
			},
		},
		{
			Pattern: "SELECT id FROM ai_jobs WHERE datasource_id = $1::uuid AND status = 'running'",
			Cols:    []string{"id"},
			Rows: [][]driver.Value{
				{"job-conflict"},
			},
		},
		{
			Pattern: "SELECT id, client_session_id, kind, status, phase, phase_message, progress_pct", // ListStaleAIJobs
			Cols:    []string{"id", "client_session_id", "kind", "status", "phase", "phase_message", "progress_pct", "datasource_id", "scope_schemas", "progress_json", "request_json", "result_json", "error_message", "created_at", "updated_at", "started_at", "finished_at", "user_id"},
			Rows: [][]driver.Value{
				{"job-stale", "sess-1", "describe", "pending", "routing", "", 5, "ds-1", "{schema1}", []byte(`{}`), []byte(`{}`), []byte(`{}`), "", now, now, now, now, nil},
			},
		},
		{
			Pattern: "SELECT COUNT(*) FROM ai_jobs WHERE status IN", // GetAIQueueStatus count pending
			Cols:    []string{"count"},
			Rows: [][]driver.Value{
				{int64(2)},
			},
		},
		{
			Pattern: "SELECT id, status, created_at FROM ai_jobs WHERE client_session_id =", // GetAIQueueStatus my job
			Cols:    []string{"id", "status", "created_at"},
			Rows: [][]driver.Value{
				{"job-1", "pending", now},
			},
		},
		{
			Pattern: "SELECT COUNT(*) + 1 FROM ai_jobs WHERE status IN", // GetAIQueueStatus position
			Cols:    []string{"count"},
			Rows: [][]driver.Value{
				{int64(1)},
			},
		},
		{
			Pattern: "SELECT DATE(created_at)", // Dashboard daily rows
			Cols:    []string{"date", "total", "success", "failed", "partial", "clarification", "avg_retry", "avg_latency", "sum_cost", "sum_tokens"},
			Rows: [][]driver.Value{
				{now, 10, 8, 1, 1, 0, 1.2, 180.0, 0.05, 1200},
			},
		},
		{
			Pattern: "SELECT COUNT(*), COUNT(*) FILTER (WHERE outcome_status = 'success')", // Dashboard summary row
			Cols:    []string{"total", "success", "failed", "partial", "clarification", "avg_retry", "avg_latency", "sum_cost", "sum_tokens", "pos_feed", "neg_feed"},
			Rows: [][]driver.Value{
				{10, 8, 1, 1, 0, 1.2, 180.0, 0.05, 1200, 4, 1},
			},
		},
	}

	// 1. CreateAIJob
	job := &AIJob{
		ID:              "job-1",
		ClientSessionID: "sess-1",
		Kind:            "describe",
		Status:          "pending",
		Phase:           "routing",
		PhaseMessage:    "",
		ProgressPct:     5,
		DatasourceID:    &dsID,
		ScopeSchemas:    []string{"schema1"},
		RequestJSON:     json.RawMessage(`{}`),
	}
	err := repo.CreateAIJob(ctx, job)
	assert.NoError(t, err)

	// 2. GetAIJob
	gotJob, err := repo.GetAIJob(ctx, "job-1")
	assert.NoError(t, err)
	assert.Equal(t, "job-1", gotJob.ID)

	// 3. ListAIJobsBySession
	jobs, err := repo.ListAIJobsBySession(ctx, "sess-1", true, 10)
	assert.NoError(t, err)
	assert.Len(t, jobs, 1)

	// 4. UpdateAIJobProgress
	err = repo.UpdateAIJobProgress(ctx, "job-1", "running", "routing", "routing update", 50)
	assert.NoError(t, err)

	// 5. UpdateAIJobProgressDetail
	err = repo.UpdateAIJobProgressDetail(ctx, "job-1", "running", "indexing", "indexing table products", 60, json.RawMessage(`{}`))
	assert.NoError(t, err)

	// 6. FindConflictingDescribeBatch
	conf, err := repo.FindConflictingDescribeBatch(ctx, "ds-1", []string{"schema1"})
	assert.NoError(t, err)
	assert.NotNil(t, conf)
	assert.Equal(t, "job-1", conf.ID)

	// 7. MarkAIJobRunning
	err = repo.MarkAIJobRunning(ctx, "job-1")
	assert.NoError(t, err)

	// 8. CompleteAIJob
	err = repo.CompleteAIJob(ctx, "job-1", json.RawMessage(`{}`))
	assert.NoError(t, err)

	// 9. FailAIJob
	err = repo.FailAIJob(ctx, "job-1", "something went wrong")
	assert.NoError(t, err)

	// 10. CancelAIJob
	cancelled, err := repo.CancelAIJob(ctx, "job-1")
	assert.NoError(t, err)
	assert.True(t, cancelled)

	// 11. ListStaleAIJobs
	stale, err := repo.ListStaleAIJobs(ctx, "sess-1", time.Minute, 10)
	assert.NoError(t, err)
	assert.Len(t, stale, 1)

	// 12. CancelAIJobs
	numCancelled, err := repo.CancelAIJobs(ctx, []string{"job-stale"})
	assert.NoError(t, err)
	assert.Equal(t, 1, numCancelled)

	// 13. CancelActiveAIJobsBySession
	numCancelledSess, err := repo.CancelActiveAIJobsBySession(ctx, "sess-1")
	assert.NoError(t, err)
	assert.Equal(t, 1, numCancelledSess)

	// 14. GetAIQueueStatus
	qStatus, err := repo.GetAIQueueStatus(ctx, "sess-1")
	assert.NoError(t, err)
	assert.Equal(t, 2, qStatus.TotalPending)
	assert.Equal(t, 1, *qStatus.MyPosition)

	// 15. TryMarkAIJobRunning
	ok, err := repo.TryMarkAIJobRunning(ctx, "job-1")
	assert.NoError(t, err)
	assert.True(t, ok)

	// 16. GetAIMetricsDashboard
	summary, daily, err := repo.GetAIMetricsDashboard(ctx, 30)
	assert.NoError(t, err)
	assert.Len(t, daily, 1)
	assert.Equal(t, 10, summary.TotalQueries)
}

func TestSecurityPolicies(t *testing.T) {
	db, state := setupMockDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	now := time.Now()

	state.execs = []execMock{
		{Pattern: "INSERT INTO permissions", RowsAffected: 1},
		{Pattern: "DELETE FROM permissions WHERE id", RowsAffected: 1},
		{Pattern: "DELETE FROM permissions WHERE user_id", RowsAffected: 1},
	}

	state.queries = []queryMock{
		{
			Pattern: "SELECT id, user_id, datasource_id, allowed_models, denied_fields, row_filters, pii_policy, created_at, updated_at FROM permissions ORDER BY",
			Cols:    []string{"id", "user_id", "datasource_id", "allowed_models", "denied_fields", "row_filters", "pii_policy", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"p-1", "role:viewer", "ds-1", `{model1}`, `{field1}`, []byte(`[{"field":"country", "operator":"eq", "value":"US"}]`), []byte(`{"public.customers.email":{"access":"masked"}}`), now, now},
			},
		},
		{
			Pattern: "SELECT id, user_id, datasource_id, allowed_models, denied_fields, row_filters, pii_policy, created_at, updated_at FROM permissions WHERE id =",
			Cols:    []string{"id", "user_id", "datasource_id", "allowed_models", "denied_fields", "row_filters", "pii_policy", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"p-1", "role:viewer", "ds-1", `{model1}`, `{field1}`, []byte(`[{"field":"country", "operator":"eq", "value":"US"}]`), []byte(`{"public.customers.email":{"access":"masked"}}`), now, now},
			},
		},
		{
			Pattern: "SELECT id, user_id, datasource_id, allowed_models, denied_fields, row_filters, pii_policy, created_at, updated_at FROM permissions WHERE user_id = $1 AND datasource_id =",
			Cols:    []string{"id", "user_id", "datasource_id", "allowed_models", "denied_fields", "row_filters", "pii_policy", "created_at", "updated_at"},
			Rows: [][]driver.Value{
				{"p-1", "role:viewer", "ds-1", `{model1}`, `{field1}`, []byte(`[{"field":"country", "operator":"eq", "value":"US"}]`), []byte(`{"public.customers.email":{"access":"masked"}}`), now, now},
			},
		},
	}

	// 1. List
	policies, err := repo.ListSecurityPolicies(ctx)
	assert.NoError(t, err)
	assert.Len(t, policies, 1)
	assert.Equal(t, "role:viewer", policies[0].UserID)
	assert.Equal(t, []string{"field1"}, policies[0].DeniedFields)
	assert.Len(t, policies[0].RowFilters, 1)
	assert.Equal(t, "country", policies[0].RowFilters[0].Field)
	assert.Equal(t, "eq", policies[0].RowFilters[0].Operator)
	assert.Equal(t, "US", policies[0].RowFilters[0].Value)

	// 2. Get by ID
	p, err := repo.GetSecurityPolicy(ctx, "p-1")
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "p-1", p.ID)

	// 3. Get by Keys
	pk, err := repo.GetSecurityPolicyByKeys(ctx, "role:viewer", "ds-1")
	assert.NoError(t, err)
	assert.NotNil(t, pk)
	assert.Equal(t, "p-1", pk.ID)

	// 4. Upsert
	newPolicy := &SecurityPolicy{
		ID:           "p-1",
		UserID:       "role:viewer",
		DatasourceID: "ds-1",
		RowFilters: []PermissionRowFilter{
			{Field: "country", Operator: "eq", Value: "US"},
		},
	}
	err = repo.UpsertSecurityPolicy(ctx, newPolicy)
	assert.NoError(t, err)

	// 5. Delete by ID
	err = repo.DeleteSecurityPolicy(ctx, "p-1")
	assert.NoError(t, err)

	// 6. Delete by Keys
	err = repo.DeleteSecurityPolicyByKeys(ctx, "role:viewer", "ds-1")
	assert.NoError(t, err)
}
