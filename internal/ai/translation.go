package ai

import (
	"context"
	"fmt"
	"github.com/bytedance/sonic"
	"strings"

	"github.com/biqly/biqly/internal/ai/jsonextract"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/config"
)

const (
	defaultTranslationTargetLanguage = "Turkish"
	defaultTranslationTargetCode     = "tr"
	// OpenAI-compatible backends reject max_tokens=0 ("Param Incorrect"); metadata
	// translation payloads can be large, so keep a generous completion budget.
	defaultTranslationMaxTokens = 4096
)

// TranslationService normalizes AI-generated metadata descriptions into a target language.
type TranslationService struct {
	provider       providerpkg.Provider
	model          string
	targetLanguage string
	targetCode     string
}

// NewTranslationServiceFromConfig returns nil when the optional translation layer is not configured.
func NewTranslationServiceFromConfig(cfg config.AIConfig) *TranslationService {
	return newTranslationServiceFromConfig(cfg, config.AIConfig{})
}

// NewTranslationServiceFromProviderStore layers the translation-purpose model's
// generation knobs from the provider store over env config.
func NewTranslationServiceFromProviderStore(store *ProviderStore, cfg config.AIConfig) *TranslationService {
	purposeCfg := config.AIConfig{}
	if store != nil {
		if pcfg, ok := store.ChatConfigForPurpose(PurposeTranslation); ok {
			purposeCfg = pcfg
		}
	}
	return newTranslationServiceFromConfig(cfg, purposeCfg)
}

func newTranslationServiceFromConfig(cfg, purposeCfg config.AIConfig) *TranslationService {
	tr := cfg.ResolvedTranslation()
	if !tr.Configured() {
		return nil
	}

	gen := purposeCfg.Generation
	if gen.MaxTokens <= 0 {
		gen.MaxTokens = cfg.Generation.MaxTokens
	}
	gen.MaxTokens = effectiveTranslationMaxTokens(gen.MaxTokens)
	gen.Temperature = 0
	if gen.TopP <= 0 {
		gen.TopP = cfg.Generation.TopP
	}
	if gen.NumCtx <= 0 {
		gen.NumCtx = cfg.Generation.NumCtx
	}

	translationCfg := config.AIConfig{
		Connection: config.AIConnectionConfig{
			Provider:           "openai-compatible",
			BaseURL:            tr.BaseURL,
			APIKey:             tr.APIKey,
			Model:              tr.Model,
			HTTPTimeoutSeconds: int(tr.HTTPTimeout.Seconds()),
		},
		Generation: gen,
	}
	return NewTranslationService(
		providerpkg.NewClient(translationCfg),
		translationCfg.Connection.Model,
		cfg.Translation.TargetLanguage,
		cfg.Translation.TargetCode,
	)
}

func effectiveTranslationMaxTokens(maxTokens int) int {
	if maxTokens > 0 {
		return maxTokens
	}
	return defaultTranslationMaxTokens
}

// NewTranslationService wires a translation provider. Tests pass a fake provider here.
func NewTranslationService(provider providerpkg.Provider, model, targetLanguage, targetCode string) *TranslationService {
	targetLanguage = strings.TrimSpace(targetLanguage)
	if targetLanguage == "" {
		targetLanguage = defaultTranslationTargetLanguage
	}
	targetCode = strings.TrimSpace(targetCode)
	if targetCode == "" {
		targetCode = defaultTranslationTargetCode
	}
	return &TranslationService{
		provider:       provider,
		model:          strings.TrimSpace(model),
		targetLanguage: targetLanguage,
		targetCode:     targetCode,
	}
}

// Model returns the configured translation model label.
func (s *TranslationService) Model() string {
	if s == nil {
		return ""
	}
	return s.model
}

func (s *TranslationService) TargetCode() string {
	if s == nil {
		return ""
	}
	return s.targetCode
}

// TranslateDescribeResult translates only free-text descriptions. Table/column identifiers are preserved.
func (s *TranslationService) TranslateDescribeResult(ctx context.Context, result *DescribeResult) error {
	if s == nil || result == nil {
		return nil
	}

	payload := describeTranslationPayload{
		TableDescription: result.Description,
		Columns:          append([]ColumnDescription(nil), result.Columns...),
	}
	rawPayload, err := sonic.ConfigStd.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal describe translation payload: %w", err)
	}

	gen, err := s.provider.GenerateAt(ctx, buildDescribeTranslationPrompt(string(rawPayload), s.targetLanguage, s.targetCode), 0)
	if err != nil {
		return fmt.Errorf("translate metadata descriptions: %w", err)
	}

	var translated describeTranslationPayload
	if err := sonic.ConfigStd.Unmarshal([]byte(jsonextract.TrimToJSONObject(gen.Content)), &translated); err != nil {
		return fmt.Errorf("parse translated metadata descriptions: %w", err)
	}
	if err := validateDescribeTranslation(payload, translated); err != nil {
		return err
	}

	result.Description = translated.TableDescription
	result.Columns = translated.Columns
	result.TranslationApplied = true
	result.TranslationModel = s.model
	return nil
}

type describeTranslationPayload struct {
	TableDescription string              `json:"table_description"`
	Columns          []ColumnDescription `json:"columns"`
}

func buildDescribeTranslationPrompt(payload, targetLanguage, targetCode string) string {
	return fmt.Sprintf(`You are a professional metadata translation layer for a Turkish-first BI application.

Task:
- Return only valid JSON with exactly the same shape as the input.
- Translate non-%[1]s description text to natural %[1]s.
- If description text is already %[1]s, keep the meaning and lightly normalize wording.
- Translate only "table_description" and each column "description" value.
- Do not change JSON keys.
- Do not change column "name" values.
- Preserve table names, column names, SQL identifiers, codes, IDs, enums, abbreviations, and common technical words such as ID, email, URL, SKU, status, key, timestamp.
- Empty descriptions must stay empty.
- Target language code: %[2]s.

Input JSON:

%[3]s`, targetLanguage, targetCode, payload)
}

func validateDescribeTranslation(original, translated describeTranslationPayload) error {
	if len(translated.Columns) != len(original.Columns) {
		return fmt.Errorf("translated metadata descriptions changed column count: want %d, got %d", len(original.Columns), len(translated.Columns))
	}
	for i := range original.Columns {
		if translated.Columns[i].Name != original.Columns[i].Name {
			return fmt.Errorf("translated metadata descriptions changed column name at index %d: want %q, got %q", i, original.Columns[i].Name, translated.Columns[i].Name)
		}
	}
	return nil
}
