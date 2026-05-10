package ai

import (
	"strings"
	"testing"

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

func TestBuildDescribePromptPrefersTurkishDescriptions(t *testing.T) {
	prompt := buildDescribePrompt("sales", "salesorderheader", []metadata.Column{
		{ColumnName: "totaldue", DataType: "numeric"},
	}, nil)

	if !strings.Contains(prompt, "Write descriptions in Turkish by default") {
		t.Fatalf("describe prompt should instruct Turkish descriptions, got:\n%s", prompt)
	}
	if !strings.Contains(prompt, "Keep original table/column names") {
		t.Fatalf("describe prompt should preserve technical schema names, got:\n%s", prompt)
	}
}
