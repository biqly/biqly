package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestConfigDocSync(t *testing.T) {
	// 1. Read config.go
	configPath := "config.go"
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config.go: %v", err)
	}
	configContent := string(configBytes)

	// 2. Extract BI_ keys in quotes using regexp
	re := regexp.MustCompile(`"BI_[A-Z0-9_]+"`)
	matches := re.FindAllString(configContent, -1)
	if len(matches) == 0 {
		t.Fatal("no environment variable keys found in config.go")
	}

	// Unique keys
	keys := make(map[string]bool)
	for _, match := range matches {
		key := strings.Trim(match, `"`)
		keys[key] = true
	}

	// 3. Read docs/configuration.md
	docBytes, err := os.ReadFile(filepath.Join("..", "..", "docs", "configuration.md"))
	if err != nil {
		t.Fatalf("failed to read docs/configuration.md: %v", err)
	}
	docContent := string(docBytes)

	// 4. Assert that every key exists in docs/configuration.md
	var missing []string
	for key := range keys {
		if !strings.Contains(docContent, "`"+key+"`") {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		t.Errorf("The following environment variables are defined in config.go but not documented in docs/configuration.md: %v", missing)
	}
}
