package metadata

import (
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/security"
)

// RuntimeDSN returns a driver-ready connection string for ds. Raw mode
// decrypts dsn_encrypted; structured mode composes the DSN from individual
// columns and decrypts the password field when applicable.
//
// This is a free function rather than a method so the pkg/metadata.Datasource
// data type can stay free of encryption/dialect dependencies — the metadata
// package only needs to know "what" a datasource is, while resolution of
// "how" to connect lives here in internal/metadata next to the security and
// driver layers it talks to.
func RuntimeDSN(ds *Datasource, enc *security.Encryption) (string, error) {
	if ds == nil {
		return "", fmt.Errorf("metadata: nil datasource")
	}
	mode := strings.TrimSpace(ds.DSNMode)
	if mode == "" {
		mode = DSNModeRaw
	}
	if mode != DSNModeStructured {
		return security.ConnectionDSN(enc, ds.DSNEncrypted)
	}
	pass, err := security.ConnectionDSN(enc, ds.PasswordEncrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt datasource password: %w", err)
	}
	extra, err := datasource.ParseConnectionParams(ds.ConnectionParams)
	if err != nil {
		return "", err
	}
	host := ""
	if ds.Host != nil {
		host = strings.TrimSpace(*ds.Host)
	}
	port := 0
	if ds.Port != nil {
		port = *ds.Port
	}
	user := ""
	if ds.Username != nil {
		user = strings.TrimSpace(*ds.Username)
	}
	dbn := ""
	if ds.DatabaseName != nil {
		dbn = strings.TrimSpace(*ds.DatabaseName)
	}
	ssl := ""
	if ds.SSLMode != nil {
		ssl = strings.TrimSpace(*ds.SSLMode)
	}
	fields := datasource.ConnectionFields{
		Host:         host,
		Port:         port,
		Username:     user,
		Password:     pass,
		DatabaseName: dbn,
		SSLMode:      ssl,
		Extra:        extra,
	}
	return datasource.ComposeDSN(ds.Type, fields)
}
