package sqlite

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDriver_IntrospectRealDB(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	d := NewDriver()
	ctx := context.Background()

	rw, err := d.Open(ctx, "file:"+path)
	require.NoError(t, err)
	_, err = rw.ExecContext(ctx, `CREATE TABLE customers (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`)
	require.NoError(t, err)
	_, err = rw.ExecContext(ctx, `CREATE TABLE orders (
		id INTEGER PRIMARY KEY,
		customer_id INTEGER NOT NULL REFERENCES customers(id),
		total REAL DEFAULT 0
	)`)
	require.NoError(t, err)
	require.NoError(t, rw.Close())

	db, err := d.Open(ctx, "file:"+path+"?mode=ro")
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	require.NoError(t, d.Ping(ctx, "file:"+path+"?mode=ro"))

	res, err := d.Introspect(ctx, db)
	require.NoError(t, err)
	require.Len(t, res.Schemas, 1)
	require.Equal(t, "main", res.Schemas[0].Name)
	require.Len(t, res.Tables, 2)
	require.Len(t, res.Relations, 1)
	require.Equal(t, "orders", res.Relations[0].FromTable)
	require.Equal(t, "customers", res.Relations[0].ToTable)

	var cols int
	for _, c := range res.Columns {
		if c.TableName == "orders" {
			cols++
		}
	}
	require.Equal(t, 3, cols)
}
