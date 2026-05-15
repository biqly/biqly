package metadata

import (
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/security"
)

// RuntimeDSN returns a driver-ready connection string. Raw mode decrypts
// dsn_encrypted; structured mode composes from columns and decrypts the
// password field when applicable.
func (ds *Datasource) RuntimeDSN(enc *security.Encryption) (string, error) {
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
