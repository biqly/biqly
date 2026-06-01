package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

type rawGoldenCase struct {
	ID       string              `json:"id"`
	Question string              `json:"question"`
	ModelID  string              `json:"model_id"`
	Expected query.LogicalQuery `json:"expected"`
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
		if err := json.Unmarshal(data, &raw); err != nil {
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
	raw := rawGoldenCase{
		ID:       id,
		Question: question,
		ModelID:  modelID,
		Expected: expected,
	}

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal golden case: %w", err)
	}

	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create golden cases directory: %w", err)
	}

	filename := filepath.Join(dir, id+".json")
	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write golden case file: %w", err)
	}
	return nil
}

// DeleteGoldenCaseFromDir removes a golden case JSON file from the directory.
func DeleteGoldenCaseFromDir(dir string, id string) error {
	filename := filepath.Join(dir, id+".json")
	if err := os.Remove(filename); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("case %s not found", id)
		}
		return fmt.Errorf("failed to delete golden case file: %w", err)
	}
	return nil
}
