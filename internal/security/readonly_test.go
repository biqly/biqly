package security

import "testing"

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
