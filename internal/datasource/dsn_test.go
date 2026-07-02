package datasource_test

import (
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/datasource"
)

func TestComposeDSN_postgres(t *testing.T) {
	f := datasource.ConnectionFields{
		Host: "db.example", Port: 5432, Username: "u", Password: "p",
		DatabaseName: "app", SSLMode: "require",
	}
	got, err := datasource.ComposeDSN("postgres", f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "postgres://") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "sslmode=require") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "db.example:5432") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "/app") {
		t.Fatalf("got %q", got)
	}
}

func TestComposeDSN_mysql(t *testing.T) {
	f := datasource.ConnectionFields{
		Host: "h", Port: 3306, Username: "root", Password: "s:ec@ret",
		DatabaseName: "db1", SSLMode: "true",
	}
	got, err := datasource.ComposeDSN("mysql", f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "root:s%3Aec%40ret@tcp(h:3306)/db1") && !strings.Contains(got, "@tcp(h:3306)/") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "tls=true") {
		t.Fatalf("got %q", got)
	}
}

func TestComposeDSN_sqlserver(t *testing.T) {
	f := datasource.ConnectionFields{
		Host: "srv", Port: 1433, Username: "sa", Password: "x",
		DatabaseName: "bi", SSLMode: "disable",
	}
	got, err := datasource.ComposeDSN("sqlserver", f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "sqlserver://") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "database=bi") {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeDriverType(t *testing.T) {
	if datasource.NormalizeDriverType("PostgreSQL") != "postgres" {
		t.Fail()
	}
	if datasource.DefaultPort("mysql") != 3306 {
		t.Fail()
	}
}

func TestNormalizeDriverType_newDrivers(t *testing.T) {
	cases := map[string]string{
		"sqlite3": "sqlite", "SQLite": "sqlite",
		"Snowflake": "snowflake",
		"spark":     "databricks", "dbx": "databricks", "Databricks": "databricks",
		"ora": "oracle", "Oracle": "oracle",
	}
	for in, want := range cases {
		if got := datasource.NormalizeDriverType(in); got != want {
			t.Errorf("NormalizeDriverType(%q) = %q, want %q", in, got, want)
		}
	}
	if datasource.DefaultPort("oracle") != 1521 {
		t.Error("oracle default port")
	}
	if datasource.DefaultPort("databricks") != 443 {
		t.Error("databricks default port")
	}
	if datasource.DefaultPort("sqlite") != 0 || datasource.DefaultPort("snowflake") != 0 {
		t.Error("sqlite/snowflake must have no default port")
	}
}

func TestComposeDSN_sqlite(t *testing.T) {
	got, err := datasource.ComposeDSN("sqlite", datasource.ConnectionFields{DatabaseName: "/data/app.db"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "file:/data/app.db?mode=ro" {
		t.Fatalf("got %q", got)
	}
	if _, err := datasource.ComposeDSN("sqlite", datasource.ConnectionFields{}); err == nil {
		t.Fatal("want error for missing file path")
	}
}

func TestComposeDSN_snowflake(t *testing.T) {
	f := datasource.ConnectionFields{
		Host: "myorg-acct1", Username: "u", Password: "p@ss",
		DatabaseName: "ANALYTICS",
		Extra:        map[string]string{"warehouse": "WH1", "role": "READER", "schema": "PUBLIC"},
	}
	got, err := datasource.ComposeDSN("snowflake", f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "u:p%40ss@myorg-acct1/ANALYTICS/PUBLIC?") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "warehouse=WH1") || !strings.Contains(got, "role=READER") {
		t.Fatalf("got %q", got)
	}
	if _, err := datasource.ComposeDSN("snowflake", datasource.ConnectionFields{Username: "u"}); err == nil {
		t.Fatal("want error for missing account")
	}
	if _, err := datasource.ComposeDSN("snowflake", datasource.ConnectionFields{Host: "a", Username: "u"}); err == nil {
		t.Fatal("want error for missing database")
	}
}

func TestComposeDSN_databricks(t *testing.T) {
	f := datasource.ConnectionFields{
		Host: "adb-123.4.azuredatabricks.net", Password: "dapiTOKEN",
		DatabaseName: "main",
		Extra:        map[string]string{"http_path": "/sql/1.0/warehouses/abc", "schema": "default"},
	}
	got, err := datasource.ComposeDSN("databricks", f)
	if err != nil {
		t.Fatal(err)
	}
	want := "token:dapiTOKEN@adb-123.4.azuredatabricks.net:443/sql/1.0/warehouses/abc?catalog=main&schema=default"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if _, err := datasource.ComposeDSN("databricks", datasource.ConnectionFields{Host: "h", Password: "t"}); err == nil {
		t.Fatal("want error for missing http_path")
	}
	if _, err := datasource.ComposeDSN("databricks", datasource.ConnectionFields{Host: "h", Extra: map[string]string{"http_path": "/p"}}); err == nil {
		t.Fatal("want error for missing token")
	}
}

func TestComposeDSN_oracle(t *testing.T) {
	f := datasource.ConnectionFields{
		Host: "dbhost", Port: 1521, Username: "scott", Password: "tiger",
		DatabaseName: "ORCLPDB1", SSLMode: "require",
	}
	got, err := datasource.ComposeDSN("oracle", f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "dbhost:1521/ORCLPDB1") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "ssl=true") {
		t.Fatalf("got %q", got)
	}
	if _, err := datasource.ComposeDSN("oracle", datasource.ConnectionFields{Host: "h", Port: 1521}); err == nil {
		t.Fatal("want error for missing service name")
	}
}
