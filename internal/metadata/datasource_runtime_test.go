package metadata

import (
	"testing"

	"github.com/biqly/biqly/internal/security"
	"github.com/stretchr/testify/require"
)

func TestDatasource_RuntimeDSN_rawPlaintext(t *testing.T) {
	t.Parallel()
	ds := &Datasource{Type: "postgres", DSNMode: DSNModeRaw, DSNEncrypted: "postgres://u:x@example:5432/db"} //nolint:gosec // test fixture URL, not real credentials
	out, err := ds.RuntimeDSN(nil)
	require.NoError(t, err)
	require.Equal(t, ds.DSNEncrypted, out)
}

func TestDatasource_RuntimeDSN_structuredMinimalPostgres(t *testing.T) {
	t.Parallel()
	host := "127.0.0.1"
	port := 5432
	user := "u"
	db := "bi"
	ssl := "disable"
	ds := &Datasource{
		Type:             "postgres",
		DSNMode:          DSNModeStructured,
		Host:             &host,
		Port:             &port,
		Username:         &user,
		DatabaseName:     &db,
		SSLMode:          &ssl,
		ConnectionParams: []byte("{}"),
	}
	out, err := ds.RuntimeDSN(nil)
	require.NoError(t, err)
	require.Contains(t, out, "postgres://")
	require.Contains(t, out, "127.0.0.1:5432")
}

func TestDatasource_RuntimeDSN_decryptsPassword(t *testing.T) {
	t.Parallel()
	key := make([]byte, 32)
	copy(key, []byte("01234567890123456789012345678901"))
	enc, err := security.NewEncryptionWithKey(key)
	require.NoError(t, err)
	cipher, err := enc.Encrypt("secret")
	require.NoError(t, err)

	host := "h"
	port := 5433
	user := "root"
	db := "app"
	ssl := "disable"
	ds := &Datasource{
		Type:              "mysql",
		DSNMode:           DSNModeStructured,
		PasswordEncrypted: cipher,
		Host:              &host,
		Port:              &port,
		Username:          &user,
		DatabaseName:      &db,
		SSLMode:           &ssl,
		ConnectionParams:  []byte("{}"),
	}
	out, err := ds.RuntimeDSN(enc)
	require.NoError(t, err)
	require.Contains(t, out, "secret")
}
