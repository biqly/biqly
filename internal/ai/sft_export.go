package ai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	evalpkg "github.com/biqly/biqly/internal/ai/eval"
	promptpkg "github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

const (
	defaultSFTLimit       = 100
	defaultMaxPromptRunes = 80000
)

// SFTRecord is one supervised fine-tuning example for Gemma/Unsloth training.
type SFTRecord struct {
	Messages []SFTMessage `json:"messages"`
	Text     string       `json:"text"`
}

// SFTMessage is one chat turn in an SFT record.
type SFTMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// SFTExportOptions configures dataset export.
type SFTExportOptions struct {
	OutDir          string
	TrainRatio      float64
	ValidationRatio float64
	MaxPromptRunes  int
	MinHistoryConf  float64
	IncludeGolden   bool
}

// SFTExportResult summarizes written files.
type SFTExportResult struct {
	TrainCount      int
	ValidationCount int
	HardEvalCount   int
	Skipped         int
	Errors          []string
}

// SFTExporter builds train/validation/hard_eval JSONL from metadata using PromptBuilder.
type SFTExporter struct {
	meta      *metadata.Repository
	semantic  *semantic.Repository
	builder   *promptpkg.PromptBuilder
	validator *query.Validator
}

// NewSFTExporter creates an exporter wired to metadata and semantic repositories.
func NewSFTExporter(meta *metadata.Repository, semantic *semantic.Repository, validator *query.Validator) *SFTExporter {
	return &SFTExporter{
		meta:      meta,
		semantic:  semantic,
		builder:   &promptpkg.PromptBuilder{},
		validator: validator,
	}
}

type sftWorkItem struct {
	source   string
	question string
	lqRaw    []byte
	splitKey string
	build    func() (userPrompt string, assistant string, err error)
}

// Export writes train.jsonl, validation.jsonl, and hard_eval.jsonl under OutDir.
func (e *SFTExporter) Export(ctx context.Context, opts SFTExportOptions) (*SFTExportResult, error) {
	if opts.OutDir == "" {
		opts.OutDir = "data/biqly-gemma4"
	}
	if opts.TrainRatio <= 0 {
		opts.TrainRatio = 0.8
	}
	if opts.ValidationRatio <= 0 {
		opts.ValidationRatio = 0.1
	}
	if opts.MaxPromptRunes <= 0 {
		opts.MaxPromptRunes = defaultMaxPromptRunes
	}
	if err := os.MkdirAll(opts.OutDir, 0o750); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	items, skipped, errs := e.collectItems(ctx, opts)
	result := &SFTExportResult{Skipped: skipped, Errors: errs}

	trainPath := filepath.Join(opts.OutDir, "train.jsonl")
	valPath := filepath.Join(opts.OutDir, "validation.jsonl")
	hardPath := filepath.Join(opts.OutDir, "hard_eval.jsonl")

	trainW, err := os.Create(trainPath) //nolint:gosec // output path is under the caller-provided export directory
	if err != nil {
		return nil, err
	}
	defer func() { _ = trainW.Close() }()
	valW, err := os.Create(valPath) //nolint:gosec // output path is under the caller-provided export directory
	if err != nil {
		return nil, err
	}
	defer func() { _ = valW.Close() }()
	hardW, err := os.Create(hardPath) //nolint:gosec // output path is under the caller-provided export directory
	if err != nil {
		return nil, err
	}
	defer func() { _ = hardW.Close() }()

	for _, item := range items {
		user, assistant, err := item.build()
		if err != nil {
			result.Skipped++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", item.source, err))
			continue
		}
		rec := SFTRecord{
			Messages: []SFTMessage{
				{Role: "system", Content: promptpkg.SystemDirective},
				{Role: "user", Content: user},
				{Role: "assistant", Content: assistant},
			},
			Text: renderGemma4SFTText(promptpkg.SystemDirective, user, assistant),
		}
		line, err := json.Marshal(rec)
		if err != nil {
			result.Skipped++
			continue
		}
		split := splitForKey(item.splitKey, opts.TrainRatio, opts.ValidationRatio)
		var w io.Writer
		switch split {
		case "train":
			w = trainW
			result.TrainCount++
		case "validation":
			w = valW
			result.ValidationCount++
		default:
			w = hardW
			result.HardEvalCount++
		}
		if _, err := w.Write(append(line, '\n')); err != nil {
			return nil, fmt.Errorf("write %s: %w", split, err)
		}
	}
	return result, nil
}

func (e *SFTExporter) collectItems(ctx context.Context, opts SFTExportOptions) ([]sftWorkItem, int, []string) {
	seen := make(map[string]bool)
	var items []sftWorkItem
	var errs []string
	skipped := 0

	add := func(source, question string, lqRaw []byte, splitKey string, buildFn func() (string, string, error)) {
		key := splitKey
		if key == "" {
			key = normalizeSplitKey(question)
		}
		dedup := source + "|" + key
		if seen[dedup] {
			skipped++
			return
		}
		seen[dedup] = true
		items = append(items, sftWorkItem{
			source:   source,
			question: question,
			lqRaw:    lqRaw,
			splitKey: key,
			build:    buildFn,
		})
	}

	fewShot, err := e.meta.ListFewShotSFTCandidates(ctx)
	if err != nil {
		errs = append(errs, "few_shot: "+err.Error())
	} else {
		for _, row := range fewShot {
			add(row.Source, row.Question, row.LogicalQuery, "", func() (string, string, error) {
				return e.buildFromDB(ctx, row.Question, row.LogicalQuery, row.DatasourceID, row.SemanticModelID, row.Dialect, opts.MaxPromptRunes)
			})
		}
	}

	history, err := e.meta.ListPositiveAIHistorySFTCandidates(ctx, opts.MinHistoryConf)
	if err != nil {
		errs = append(errs, "history: "+err.Error())
	} else {
		for _, row := range history {
			add(row.Source, row.Question, row.LogicalQuery, "", func() (string, string, error) {
				return e.buildFromDB(ctx, row.Question, row.LogicalQuery, row.DatasourceID, row.SemanticModelID, "", opts.MaxPromptRunes)
			})
		}
	}

	if opts.IncludeGolden {
		for _, c := range evalpkg.DefaultGoldenCases() {
			lqBytes, err := json.Marshal(c.Expected)
			if err != nil {
				skipped++
				continue
			}
			if err := validateTrainingLogicalQuery(lqBytes, c.Model, e.validator); err != nil {
				skipped++
				continue
			}
			assistant, err := canonicalTrainingLogicalQuery(lqBytes)
			if err != nil {
				skipped++
				continue
			}
			add("golden", c.Question, lqBytes, "golden:"+c.ID, func() (string, string, error) {
				user := e.builder.Build(context.Background(), c.Question, c.Model, promptpkg.PromptConfig{
					MaxRunes: opts.MaxPromptRunes,
					Locale:   i18n.DefaultLocale,
					Dialect:  "postgres",
				})
				return user, assistant, nil
			})
		}
	}

	return items, skipped, errs
}

func (e *SFTExporter) buildFromDB(
	ctx context.Context,
	question string,
	lqRaw []byte,
	datasourceID, semanticModelID, dialectHint string,
	maxPromptRunes int,
) (string, string, error) {
	if semanticModelID == "" {
		return "", "", fmt.Errorf("missing semantic model id")
	}
	model, err := e.semantic.GetPublishedFullModel(ctx, semanticModelID)
	if err != nil {
		return "", "", fmt.Errorf("load model %s: %w", semanticModelID, err)
	}
	if err := validateTrainingLogicalQuery(lqRaw, model, e.validator); err != nil {
		return "", "", err
	}
	assistant, err := canonicalTrainingLogicalQuery(lqRaw)
	if err != nil {
		return "", "", err
	}
	dialect := dialectHint
	if dialect == "" && datasourceID != "" {
		ds, err := e.meta.GetDatasource(ctx, datasourceID)
		if err == nil {
			dialect = ds.Type
		}
	}
	user := e.builder.Build(context.Background(), question, model, promptpkg.PromptConfig{
		MaxRunes: maxPromptRunes,
		Locale:   i18n.DefaultLocale,
		Dialect:  dialect,
	})
	return user, assistant, nil
}

func validateTrainingLogicalQuery(raw []byte, model *semantic.SemanticModel, v *query.Validator) error {
	var lq query.LogicalQuery
	if err := json.Unmarshal(raw, &lq); err != nil {
		return fmt.Errorf("parse logical_query: %w", err)
	}
	if len(lq.Select) == 0 {
		return fmt.Errorf("empty select")
	}
	if v != nil && model != nil {
		if err := v.Validate(&lq, model); err != nil {
			return fmt.Errorf("validate: %w", err)
		}
	}
	return nil
}

func canonicalTrainingLogicalQuery(raw []byte) (string, error) {
	var lq query.LogicalQuery
	if err := json.Unmarshal(raw, &lq); err != nil {
		return "", err
	}
	lq.EnsureVersion()
	if lq.Limit <= 0 {
		lq.Limit = defaultSFTLimit
	}
	lq.EnsureGroupBySelected()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(&lq); err != nil {
		return "", err
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		return "", err
	}
	delete(m, "datasource_id")
	delete(m, "model_id")
	buf.Reset()
	enc = json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func normalizeSplitKey(question string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(question)), " "))
}

func splitForKey(key string, trainRatio, validationRatio float64) string {
	h := sha256.Sum256([]byte(key))
	n := float64(h[0]) / 255.0
	if n < trainRatio {
		return "train"
	}
	if n < trainRatio+validationRatio {
		return "validation"
	}
	return "hard_eval"
}

// renderGemma4SFTText formats messages for Unsloth Gemma 4 SFT (dataset_text_field="text").
// Matches Unsloth "gemma-4" chat template: <|turn>system/user/model (no <bos> — tokenizer adds it).
func renderGemma4SFTText(system, user, assistant string) string {
	var b strings.Builder
	b.WriteString("<|turn>system\n")
	b.WriteString(system)
	b.WriteString("<|turn>user\n")
	b.WriteString(user)
	b.WriteString("<|turn>model\n")
	b.WriteString(assistant)
	return b.String()
}

// WriteJSONL appends records to path (used in tests).
func WriteJSONL(path string, records []SFTRecord) error {
	f, err := os.Create(path) //nolint:gosec // helper writes to caller-provided test/export path
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	for _, rec := range records {
		line, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(line, '\n')); err != nil {
			return err
		}
	}
	return nil
}

// SplitBucket exposes split assignment for tests.
func SplitBucket(question string, trainRatio, validationRatio float64) string {
	return splitForKey(normalizeSplitKey(question), trainRatio, validationRatio)
}
