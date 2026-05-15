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
