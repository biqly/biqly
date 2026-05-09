package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

// Service orchestrates the AI text-to-query flow.
type Service struct {
	client        *Client
	promptBuilder *PromptBuilder
	validator     *query.Validator
}

// NewService creates a new AI service.
func NewService(cfg config.AIConfig, validator *query.Validator) *Service {
	return &Service{
		client:        NewClient(cfg),
		promptBuilder: &PromptBuilder{},
		validator:     validator,
	}
}

// ProcessQuestion handles a natural language question.
func (s *Service) ProcessQuestion(ctx context.Context, question string, model *semantic.SemanticModel) (*AIResponse, error) {
	// Build prompt
	prompt := s.promptBuilder.Build(question, model)

	// Call LLM
	rawResponse, err := s.client.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	// Parse and validate
	lq, warnings, err := s.parseAndValidate(rawResponse, model)
	if err != nil {
		return &AIResponse{
			Warnings:    append(warnings, err.Error()),
			Prompt:      prompt,
			RawResponse: rawResponse,
		}, nil
	}

	resp := &AIResponse{
		LogicalQuery: lq,
		Confidence:   0.8,
		Warnings:     warnings,
		Prompt:       prompt,
		RawResponse:  rawResponse,
	}

	return resp, nil
}

func (s *Service) parseAndValidate(raw string, model *semantic.SemanticModel) (*query.LogicalQuery, []string, error) {
	var warnings []string

	// Clean response - strip markdown code blocks if present
	cleaned := raw
	if idx := strings.Index(cleaned, "```json"); idx >= 0 {
		cleaned = cleaned[idx+7:]
		if end := strings.Index(cleaned, "```"); end >= 0 {
			cleaned = cleaned[:end]
		}
	}
	cleaned = strings.TrimSpace(cleaned)

	// Parse JSON
	var lq query.LogicalQuery
	if err := json.Unmarshal([]byte(cleaned), &lq); err != nil {
		return nil, warnings, fmt.Errorf("invalid JSON from AI: %w", err)
	}
	normalizeLogicalQueryContext(&lq, model)

	// Guardrails: reject empty selects
	if len(lq.Select) == 0 {
		warnings = append(warnings, "AI returned empty select - question may be ambiguous")
		return nil, warnings, fmt.Errorf("ambiguous question")
	}

	// Validate against semantic model
	if err := s.validator.Validate(lq, model); err != nil {
		warnings = append(warnings, "validation warnings: "+err.Error())
		// Still return the query but with warnings
	}

	return &lq, warnings, nil
}

func normalizeLogicalQueryContext(lq *query.LogicalQuery, model *semantic.SemanticModel) {
	lq.DatasourceID = model.DatasourceID
	lq.ModelID = model.Name
	if lq.ModelID == "" {
		lq.ModelID = model.ID
	}
}
