package dialect

import "testing"

func TestQuoteIdentSegment_doesNotSplitOnDot(t *testing.T) {
	d := SQLServer
	got := d.QuoteIdentSegment("Emp.StartDate")
	want := "[Emp.StartDate]"
	if got != want {
		t.Fatalf("QuoteIdentSegment = %q, want %q", got, want)
	}
}

func TestQuoteIdent_stillSplitsQualified(t *testing.T) {
	d := SQLServer
	got := d.QuoteIdent("dbo.Orders")
	want := "[dbo].[Orders]"
	if got != want {
		t.Fatalf("QuoteIdent = %q, want %q", got, want)
	}
}

func TestPostgresQuoteIdentSegment(t *testing.T) {
	d := Postgres
	if got, want := d.QuoteIdentSegment(`Emp.StartDate`), `"Emp.StartDate"`; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if got, want := d.QuoteIdentSegment(`weird"name`), `"weird""name"`; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
