package security

import (
	"strings"
	"testing"
)

func TestReadOnlyCheckerAllowsIdentifiersContainingDangerousWords(t *testing.T) {
	checker := NewReadOnlyChecker()
	query := `SELECT MIN("orders"."created_at") AS "first_order_created_at" FROM "public"."orders"`

	if err := checker.Check(query); err != nil {
		t.Fatalf("expected query to pass readonly check, got %v", err)
	}
}

func TestReadOnlyCheckerRejectsDangerousKeyword(t *testing.T) {
	checker := NewReadOnlyChecker()
	query := `SELECT * FROM "orders" DROP TABLE "orders"`

	if err := checker.Check(query); err == nil {
		t.Fatal("expected query with DROP keyword to fail readonly check")
	}
}

func TestReadOnlyCheckerRejectsTrailingMultiStatement(t *testing.T) {
	checker := NewReadOnlyChecker()
	query := `SELECT 1; DROP TABLE x;`

	err := checker.Check(query)
	if err == nil {
		t.Fatal("expected multi-statement query to be rejected")
	}
}

func TestReadOnlyCheckerRejectsHiddenMultiStatement(t *testing.T) {
	checker := NewReadOnlyChecker()
	query := `SELECT 1 /* hi */; SELECT 2;`

	if err := checker.Check(query); err == nil {
		t.Fatal("expected multi-statement query (with comment) to be rejected")
	}
}

func TestReadOnlyCheckerAllowsTrailingSemicolon(t *testing.T) {
	checker := NewReadOnlyChecker()
	query := `SELECT 1;`

	if err := checker.Check(query); err != nil {
		t.Fatalf("expected single trailing-semicolon query to pass, got %v", err)
	}
}

func TestReadOnlyCheckerAllowsCTEPrefix(t *testing.T) {
	checker := NewReadOnlyChecker()
	query := `WITH x AS (SELECT 1) SELECT * FROM x`

	if err := checker.Check(query); err != nil {
		t.Fatalf("expected CTE query to pass, got %v", err)
	}
}

func TestReadOnlyCheckerAllowsExplain(t *testing.T) {
	checker := NewReadOnlyChecker()
	query := `EXPLAIN SELECT * FROM orders`

	if err := checker.Check(query); err != nil {
		t.Fatalf("expected EXPLAIN query to pass, got %v", err)
	}
}

func TestReadOnlyCheckerStripsStringLiterals(t *testing.T) {
	checker := NewReadOnlyChecker()
	// DROP / DELETE keywords inside a string literal must not trigger a reject
	query := `SELECT * FROM logs WHERE message = 'DROP TABLE x; DELETE FROM y'`

	if err := checker.Check(query); err != nil {
		t.Fatalf("expected query with dangerous words inside string to pass, got %v", err)
	}
}

func TestReadOnlyCheckerRejectsAdditionalDangerousFunctions(t *testing.T) {
	checker := NewReadOnlyChecker()
	cases := []string{
		`SELECT xp_cmdshell('whoami')`,
		`SELECT * FROM OPENROWSET('Microsoft.Jet.OLEDB.4.0', '...')`,
		`SELECT pg_read_file('/etc/passwd')`,
		`SELECT LOAD_FILE('/etc/passwd')`,
		`SELECT * FROM dblink('host=evil dbname=postgres', 'SELECT 1') AS t(c int)`,
		`SELECT lo_import('/etc/passwd')`,
	}
	for _, q := range cases {
		if err := checker.Check(q); err == nil {
			t.Errorf("expected query to be rejected: %s", q)
		}
	}
}

func TestReadOnlyCheckerRejectsBulkInsert(t *testing.T) {
	checker := NewReadOnlyChecker()
	query := `SELECT 1 /* */ BULK   INSERT mytable FROM 'c:\\data.csv'`

	err := checker.Check(query)
	if err == nil || !strings.Contains(err.Error(), "BULK") && !strings.Contains(err.Error(), "INSERT") {
		t.Fatalf("expected BULK INSERT to be rejected, got %v", err)
	}
}

func TestReadOnlyCheckerRejectsNonSelectPrefix(t *testing.T) {
	checker := NewReadOnlyChecker()
	query := `UPDATE orders SET total = 0`

	if err := checker.Check(query); err == nil {
		t.Fatal("expected UPDATE to be rejected")
	}
}

func TestReadOnlyCheckerRejectsNewDangerousKeywords(t *testing.T) {
	checker := NewReadOnlyChecker()
	cases := []string{
		`SELECT 1; SET role='admin'`,
		`SELECT 1; RESET ALL`,
		`SELECT 1; COPY users TO '/tmp/users.txt'`,
		`SELECT 1; DO $$ BEGIN END $$`,
		`SELECT 1; LOCK TABLE users`,
		`SELECT 1; VACUUM`,
		`SELECT 1; REINDEX DATABASE postgres`,
	}
	for _, q := range cases {
		if err := checker.Check(q); err == nil {
			t.Errorf("expected query to be rejected: %s", q)
		}
	}
}
