package ai

import (
	"encoding/json"
	"fmt"

	"github.com/biqly/biqly/internal/ai/jsonextract"
	promptpkg "github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

const defaultSchemaValidatorMaxRows = 10_000

// SchemaValidator parses AI JSON output and validates it against the semantic model.
type SchemaValidator struct {
	validator *query.Validator
}

// NewSchemaValidator creates a validator with the default row limit.
func NewSchemaValidator() *SchemaValidator {
	return NewSchemaValidatorWith(nil)
}

// NewSchemaValidatorWith uses the given query validator (or a default when nil).
func NewSchemaValidatorWith(validator *query.Validator) *SchemaValidator {
	if validator == nil {
		validator = query.NewValidator(defaultSchemaValidatorMaxRows)
	}
	return &SchemaValidator{validator: validator}
}

func parseLogicalQueryFromRaw(raw string) (query.LogicalQuery, error) {
	cleaned := jsonextract.TrimToJSONObject(raw)
	if cleaned == "" {
		return query.LogicalQuery{}, fmt.Errorf("empty AI response")
	}
	var lq query.LogicalQuery
	if err := json.Unmarshal([]byte(cleaned), &lq); err != nil {
		return query.LogicalQuery{}, fmt.Errorf("invalid JSON: %w (raw: %s)", err, promptpkg.TruncateRunes(cleaned, 200))
	}
	return lq, nil
}

// Validate parses raw AI output and runs full semantic validation.
func (sv *SchemaValidator) Validate(rawJSON string, model *semantic.SemanticModel) (*query.LogicalQuery, error) {
	lq, err := parseLogicalQueryFromRaw(rawJSON)
	if err != nil {
		return nil, err
	}
	if len(lq.Select) == 0 {
		return nil, fmt.Errorf("missing required field: select")
	}
	if err := sv.validator.Validate(&lq, model); err != nil {
		return nil, err
	}
	return &lq, nil
}
