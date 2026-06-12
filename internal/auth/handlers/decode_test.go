package handlers

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestAuthHandlersUseSharedJSONDecoder(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	handlersDir := filepath.Dir(testFile)
	entries, err := os.ReadDir(handlersDir)
	if err != nil {
		t.Fatalf("os.ReadDir(%q) error = %v", handlersDir, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(handlersDir, name)
		// #nosec G304 -- path is selected from the fixed auth handlers package directory.
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", name, err)
		}
		if strings.Contains(string(content), "sonic.ConfigStd.NewDecoder(r.Body)") {
			t.Errorf("%s uses a raw JSON body decoder; use decodeJSON/decodeJSONAllowEmpty", name)
		}
	}
}
