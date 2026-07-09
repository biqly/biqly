package security

import (
	"slices"
	"testing"
)

func TestNormalizeFunctionBlocklist(t *testing.T) {
	got, err := NormalizeFunctionBlocklist([]string{" Custom_Fn ", "custom_fn", "Other_Fn"})
	if err != nil {
		t.Fatalf("NormalizeFunctionBlocklist() error = %v", err)
	}
	want := []string{"custom_fn", "other_fn"}
	if !slices.Equal(got, want) {
		t.Fatalf("NormalizeFunctionBlocklist() = %v, want %v", got, want)
	}
}

func TestNormalizeFunctionBlocklistRejectsInvalidName(t *testing.T) {
	if _, err := NormalizeFunctionBlocklist([]string{"dblink; DROP TABLE users"}); err == nil {
		t.Fatal("NormalizeFunctionBlocklist() error = nil, want invalid name error")
	}
}

func TestNormalizeCustomFunctionBlocklistOmitsDefaults(t *testing.T) {
	got, err := NormalizeCustomFunctionBlocklist([]string{"PG_READ_FILE", "custom_reader"})
	if err != nil {
		t.Fatalf("NormalizeCustomFunctionBlocklist() error = %v", err)
	}
	if !slices.Equal(got, []string{"custom_reader"}) {
		t.Fatalf("NormalizeCustomFunctionBlocklist() = %v, want [custom_reader]", got)
	}
}

func TestReadOnlyCheckerBlocksCustomFunctionsOnlyAsCalls(t *testing.T) {
	checker, err := NewReadOnlyCheckerWithAdditionalDeniedFunctions([]string{"custom_reader"})
	if err != nil {
		t.Fatalf("NewReadOnlyCheckerWithAdditionalDeniedFunctions() error = %v", err)
	}

	if err := checker.Check(`SELECT custom_reader('secret')`); err == nil {
		t.Fatal("Check() error = nil, want custom function rejection")
	}
	if err := checker.Check(`SELECT "custom_reader"('secret')`); err == nil {
		t.Fatal("Check() error = nil, want quoted custom function rejection")
	}
	if err := checker.Check(`SELECT custom_reader FROM reports`); err != nil {
		t.Fatalf("Check() error = %v, want identifier to pass", err)
	}
	if err := checker.Check(`SELECT 'custom_reader()' AS note`); err != nil {
		t.Fatalf("Check() error = %v, want literal to pass", err)
	}
	if err := checker.Check(`SELECT 1 /* custom_reader() */`); err != nil {
		t.Fatalf("Check() error = %v, want comment to pass", err)
	}
}

func TestReadOnlyCheckerBlocksQuotedDefaultFunction(t *testing.T) {
	if err := NewReadOnlyChecker().Check(`SELECT "pg_read_file"('/etc/passwd')`); err == nil {
		t.Fatal("Check() error = nil, want quoted default function rejection")
	}
}

func TestEffectiveDeniedFunctionsKeepsDefaults(t *testing.T) {
	got, err := EffectiveDeniedFunctions([]string{"pg_read_file", "custom_reader"})
	if err != nil {
		t.Fatalf("EffectiveDeniedFunctions() error = %v", err)
	}
	if !slices.Contains(got, "pg_read_file") || !slices.Contains(got, "custom_reader") {
		t.Fatalf("EffectiveDeniedFunctions() = %v, want defaults and custom entry", got)
	}
	if countFunction(got, "pg_read_file") != 1 {
		t.Fatalf("pg_read_file count = %d, want 1", countFunction(got, "pg_read_file"))
	}
}

func countFunction(functions []string, want string) int {
	count := 0
	for _, function := range functions {
		if function == want {
			count++
		}
	}
	return count
}
