package datasource

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ConnectionFields holds plain connection values used to build a driver DSN.
// Password is plaintext and must only exist transiently at request/compose time.
type ConnectionFields struct {
	Host         string
	Port         int
	Username     string
	Password     string
	DatabaseName string
	SSLMode      string
	Extra        map[string]string
}

// NormalizeDriverType maps UI or alias names to registry driver keys.
func NormalizeDriverType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "postgresql", "postgres", "pg":
		return "postgres"
	case "mysql", "mariadb":
		return "mysql"
	case "sqlserver", "mssql":
		return "sqlserver"
	case "clickhouse", "ch":
		return "clickhouse"
	default:
		return strings.ToLower(strings.TrimSpace(t))
	}
}

// DefaultPort returns the conventional TCP port when none is set.
func DefaultPort(driver string) int {
	switch NormalizeDriverType(driver) {
	case "postgres":
		return 5432
	case "mysql":
		return 3306
	case "sqlserver":
		return 1433
	case "clickhouse":
		return 9000
	default:
		return 0
	}
}

// DriverConnectionDefaults returns UI-friendly defaults per driver.
func DriverConnectionDefaults(driver string) ConnectionFields {
	d := NormalizeDriverType(driver)
	port := DefaultPort(d)
	ssl := ""
	switch d {
	case "postgres":
		ssl = "disable"
	case "mysql":
		ssl = "false"
	case "sqlserver":
		ssl = "disable"
	case "clickhouse":
		ssl = "false"
	}
	return ConnectionFields{Port: port, SSLMode: ssl}
}

// ParseConnectionParams unmarshals JSON metadata connection_params into a string map.
func ParseConnectionParams(raw []byte) (map[string]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]string{}, nil
	}
	var m map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("connection_params: %w", err)
	}
	if m == nil {
		return map[string]string{}, nil
	}
	return m, nil
}

// ComposeDSN builds a single connection string for Open/Ping.
func ComposeDSN(driver string, f ConnectionFields) (string, error) {
	d := NormalizeDriverType(driver)
	host := strings.TrimSpace(f.Host)
	if host == "" {
		return "", errors.New("host is required")
	}
	port := f.Port
	if port <= 0 {
		port = DefaultPort(d)
	}
	if port <= 0 {
		return "", fmt.Errorf("unsupported datasource type: %s", driver)
	}
	db := strings.TrimSpace(f.DatabaseName)
	user := strings.TrimSpace(f.Username)
	pass := f.Password
	ssl := strings.TrimSpace(f.SSLMode)

	switch d {
	case "postgres":
		if db == "" {
			return "", errors.New("database name is required for postgres")
		}
		mode := ssl
		if mode == "" {
			mode = "disable"
		}
		u := url.URL{
			Scheme: "postgres",
			User:   url.UserPassword(user, pass),
			Host:   fmt.Sprintf("%s:%d", host, port),
			Path:   "/" + db,
		}
		q := url.Values{}
		q.Set("sslmode", mode)
		for k, v := range f.Extra {
			k = strings.TrimSpace(k)
			if k != "" && strings.ToLower(k) != "sslmode" {
				q.Set(k, v)
			}
		}
		u.RawQuery = q.Encode()
		return u.String(), nil

	case "mysql":
		if db == "" {
			return "", errors.New("database name is required for mysql")
		}
		cfg := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
			url.QueryEscape(user), url.QueryEscape(pass), host, port, url.QueryEscape(db))
		params := url.Values{}
		params.Set("parseTime", "true")
		params.Set("loc", "UTC")
		l := strings.ToLower(ssl)
		switch l {
		case "true", "require", "required", "skip-verify", "skip_verify":
			params.Set("tls", "true")
		case "false", "disable", "":
			break
		default:
			params.Set("tls", ssl)
		}
		for k, v := range f.Extra {
			k = strings.TrimSpace(k)
			if k != "" {
				params.Set(k, v)
			}
		}
		if enc := params.Encode(); enc != "" {
			cfg += "?" + enc
		}
		return cfg, nil

	case "sqlserver":
		if db == "" {
			return "", errors.New("database name is required for sqlserver")
		}
		u := url.URL{
			Scheme: "sqlserver",
			User:   url.UserPassword(user, pass),
			Host:   fmt.Sprintf("%s:%d", host, port),
		}
		q := url.Values{}
		q.Set("database", db)
		l := strings.ToLower(ssl)
		switch l {
		case "disable", "false", "":
			q.Set("encrypt", "disable")
		case "require", "true", "strict":
			q.Set("encrypt", "true")
		default:
			q.Set("encrypt", ssl)
		}
		for k, v := range f.Extra {
			k = strings.TrimSpace(k)
			if k != "" {
				q.Set(k, v)
			}
		}
		u.RawQuery = q.Encode()
		return u.String(), nil

	case "clickhouse":
		if db == "" {
			db = "default"
		}
		u := url.URL{
			Scheme: "clickhouse",
			User:   url.UserPassword(user, pass),
			Host:   fmt.Sprintf("%s:%d", host, port),
			Path:   "/" + db,
		}
		q := url.Values{}
		l := strings.ToLower(ssl)
		if l == "true" || l == "require" {
			q.Set("secure", "true")
		}
		for k, v := range f.Extra {
			k = strings.TrimSpace(k)
			if k != "" {
				q.Set(k, v)
			}
		}
		u.RawQuery = q.Encode()
		return u.String(), nil

	default:
		return "", fmt.Errorf("compose DSN: unsupported driver %q", driver)
	}
}
