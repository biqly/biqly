package datasource

import (
	"errors"
	"fmt"
	"github.com/bytedance/sonic"
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
	if err := sonic.ConfigStd.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("connection_params: %w", err)
	}
	if m == nil {
		return map[string]string{}, nil
	}
	return m, nil
}

type dsnParts struct {
	driver string
	host   string
	port   int
	db     string
	user   string
	pass   string
	ssl    string
	extra  map[string]string
}

func prepareDSNParts(driver string, f ConnectionFields) (dsnParts, error) {
	d := NormalizeDriverType(driver)
	host := strings.TrimSpace(f.Host)
	if host == "" {
		return dsnParts{}, errors.New("host is required")
	}
	port := f.Port
	if port <= 0 {
		port = DefaultPort(d)
	}
	if port <= 0 {
		return dsnParts{}, fmt.Errorf("unsupported datasource type: %s", driver)
	}
	return dsnParts{
		driver: d,
		host:   host,
		port:   port,
		db:     strings.TrimSpace(f.DatabaseName),
		user:   strings.TrimSpace(f.Username),
		pass:   f.Password,
		ssl:    strings.TrimSpace(f.SSLMode),
		extra:  f.Extra,
	}, nil
}

func mergeExtraQuery(q url.Values, extra map[string]string) {
	for k, v := range extra {
		k = strings.TrimSpace(k)
		if k != "" {
			q.Set(k, v)
		}
	}
}

func composePostgresDSN(p dsnParts) (string, error) {
	if p.db == "" {
		return "", errors.New("database name is required for postgres")
	}
	mode := p.ssl
	if mode == "" {
		mode = "disable"
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(p.user, p.pass),
		Host:   fmt.Sprintf("%s:%d", p.host, p.port),
		Path:   "/" + p.db,
	}
	q := url.Values{}
	q.Set("sslmode", mode)
	for k, v := range p.extra {
		k = strings.TrimSpace(k)
		if k != "" && strings.ToLower(k) != "sslmode" {
			q.Set(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func composeMySQLDSN(p dsnParts) (string, error) {
	if p.db == "" {
		return "", errors.New("database name is required for mysql")
	}
	cfg := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		url.QueryEscape(p.user), url.QueryEscape(p.pass), p.host, p.port, url.QueryEscape(p.db))
	params := url.Values{}
	params.Set("parseTime", "true")
	params.Set("loc", "UTC")
	switch strings.ToLower(p.ssl) {
	case "true", "require", "required", "skip-verify", "skip_verify":
		params.Set("tls", "true")
	case "false", "disable", "":
	default:
		params.Set("tls", p.ssl)
	}
	mergeExtraQuery(params, p.extra)
	if enc := params.Encode(); enc != "" {
		cfg += "?" + enc
	}
	return cfg, nil
}

func composeSQLServerDSN(p dsnParts) (string, error) {
	if p.db == "" {
		return "", errors.New("database name is required for sqlserver")
	}
	u := url.URL{
		Scheme: "sqlserver",
		User:   url.UserPassword(p.user, p.pass),
		Host:   fmt.Sprintf("%s:%d", p.host, p.port),
	}
	q := url.Values{}
	q.Set("database", p.db)
	switch strings.ToLower(p.ssl) {
	case "disable", "false", "":
		q.Set("encrypt", "disable")
	case "require", "true", "strict":
		q.Set("encrypt", "true")
	default:
		q.Set("encrypt", p.ssl)
	}
	mergeExtraQuery(q, p.extra)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func composeClickHouseDSN(p dsnParts) (string, error) {
	db := p.db
	if db == "" {
		db = "default"
	}
	u := url.URL{
		Scheme: "clickhouse",
		User:   url.UserPassword(p.user, p.pass),
		Host:   fmt.Sprintf("%s:%d", p.host, p.port),
		Path:   "/" + db,
	}
	q := url.Values{}
	l := strings.ToLower(p.ssl)
	if l == "true" || l == "require" {
		q.Set("secure", "true")
	}
	mergeExtraQuery(q, p.extra)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ComposeDSN builds a single connection string for Open/Ping.
func ComposeDSN(driver string, f ConnectionFields) (string, error) {
	p, err := prepareDSNParts(driver, f)
	if err != nil {
		return "", err
	}
	switch p.driver {
	case "postgres":
		return composePostgresDSN(p)
	case "mysql":
		return composeMySQLDSN(p)
	case "sqlserver":
		return composeSQLServerDSN(p)
	case "clickhouse":
		return composeClickHouseDSN(p)
	default:
		return "", fmt.Errorf("compose DSN: unsupported driver %q", driver)
	}
}
