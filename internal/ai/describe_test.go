package ai

import (
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/metadata"
)

func TestValidIdent_columnNames(t *testing.T) {
	tests := []struct {
		name string
		id   string
		ok   bool
	}{
		{"letter", "Year", true},
		{"leading digit", "2012", true},
		{"underscore", "_x", true},
		{"dollar suffix", "col$", true},
		{"dot inside name", "a.b", true},
		{"Emp.StartDate", "Emp.StartDate", true},
		{"semicolon", "x;drop", false},
		{"space", "x y", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validIdent(tt.id); got != tt.ok {
				t.Fatalf("validIdent(%q) = %v, want %v", tt.id, got, tt.ok)
			}
		})
	}
}

func TestBuildDescribePromptRequestsEnglishDescriptions(t *testing.T) {
	prompt := buildDescribePrompt("sales", "salesorderheader", []metadata.Column{
		{ColumnName: "totaldue", DataType: "numeric"},
	}, nil)

	if !strings.Contains(prompt, "Write descriptions in English") {
		t.Fatalf("describe prompt should instruct English descriptions, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Keep original table/column names") {
		t.Fatalf("describe prompt should preserve technical schema names, got:\n%s", prompt)
	}
}

func TestBuildTableSampleSQLUsesTopForSQLServer(t *testing.T) {
	got, err := buildTableSampleSQL(dialect.SQLServer, []metadata.Column{
		{ColumnName: "brand_id"},
		{ColumnName: "brand_name"},
	}, "production", "brands", 10)
	if err != nil {
		t.Fatalf("buildTableSampleSQL() error = %v", err)
	}

	want := "SELECT TOP (10) [brand_id], [brand_name] FROM [production].[brands]"
	if got != want {
		t.Fatalf("sample SQL mismatch.\nGot:  %s\nWant: %s", got, want)
	}
}
