package eval

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bytedance/sonic"

	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

type rawGoldenCase struct {
	ID       string             `json:"id"`
	Question string             `json:"question"`
	ModelID  string             `json:"model_id"`
	Expected query.LogicalQuery `json:"expected"`
}

// sanitizeID validates that id is a safe filename component with no path
// traversal. It returns the base name or an error if the id is empty, contains
// path separators, or resolves to a directory traversal ("..").
func sanitizeID(id string) (string, error) {
	if id == "" {
		return "", errors.New("empty id")
	}
	if strings.ContainsAny(id, `/\`) {
		return "", fmt.Errorf("id contains path separator: %q", id)
	}
	clean := filepath.Base(id)
	if clean == "." || clean == ".." {
		return "", fmt.Errorf("invalid id: %q", id)
	}
	return clean, nil
}

func goldenCaseFileName(safeID string) string {
	return safeID + ".json"
}

// LoadGoldenCasesFromDir scans and loads all *.json files in the given directory.
func LoadGoldenCasesFromDir(dir string) ([]GoldenCase, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to glob golden cases files: %w", err)
	}

	var cases []GoldenCase
	for _, file := range files {
		//nolint:gosec // path is globbed and verified in secure directory
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read golden case file %s: %w", file, err)
		}

		var raw rawGoldenCase
		if err := sonic.ConfigStd.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("failed to parse golden case JSON in file %s: %w", file, err)
		}

		cases = append(cases, GoldenCase{
			ID:       raw.ID,
			Question: raw.Question,
			Model:    &semantic.SemanticModel{ID: raw.ModelID},
			Expected: raw.Expected,
		})
	}
	return cases, nil
}

// SaveGoldenCaseToDir saves a golden case to a JSON file in the directory.
func SaveGoldenCaseToDir(dir string, id string, question string, modelID string, expected query.LogicalQuery) error {
	safeID, err := sanitizeID(id)
	if err != nil {
		return fmt.Errorf("invalid golden case id: %w", err)
	}

	raw := rawGoldenCase{
		ID:       id,
		Question: question,
		ModelID:  modelID,
		Expected: expected,
	}

	data, err := sonic.ConfigStd.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal golden case: %w", err)
	}

	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create golden cases directory: %w", err)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("failed to open golden cases directory: %w", err)
	}
	defer func() { _ = root.Close() }()

	file, err := root.OpenFile(goldenCaseFileName(safeID), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to write golden case file: %w", err)
	}
	defer func() { _ = file.Close() }()

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("failed to write golden case file: %w", err)
	}
	return nil
}

// DeleteGoldenCaseFromDir removes a golden case JSON file from the directory.
func DeleteGoldenCaseFromDir(dir string, id string) error {
	safeID, err := sanitizeID(id)
	if err != nil {
		return fmt.Errorf("invalid golden case id: %w", err)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("failed to open golden cases directory: %w", err)
	}
	defer func() { _ = root.Close() }()

	if err := root.Remove(goldenCaseFileName(safeID)); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("case %s not found", id)
		}
		return fmt.Errorf("failed to delete golden case file: %w", err)
	}
	return nil
}
