package pii

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
)

// DefaultSampleLimit is the number of non-NULL sample values fetched per
// column when scanning a datasource.
const DefaultSampleLimit = 50

// ColumnStore is the metadata persistence the scanner depends on.
type ColumnStore interface {
	ListColumns(ctx context.Context, datasourceID, schemaName, tableName string) ([]pkgmetadata.Column, error)
	UpdateColumnPIIDetection(ctx context.Context, id, piiType string, confidence float64) error
	ClearColumnPII(ctx context.Context, id string, reviewedBy string) error
}

// SampleFetcher returns up to limit non-NULL values for a column from the
// live datasource connection. A nil fetcher limits detection to column-name
// heuristics.
type SampleFetcher func(ctx context.Context, col pkgmetadata.Column, limit int) ([]string, error)

// ScanSummary reports the outcome of a datasource PII scan.
type ScanSummary struct {
	ScannedColumns int            `json:"scanned_columns"`
	Detected       map[string]int `json:"detected"`
}

// Scanner runs PII detection over every column of a datasource and persists
// the results as column annotations.
type Scanner struct {
	detector    *Detector
	store       ColumnStore
	sampleLimit int
}

// NewScanner creates a Scanner. Non-positive sampleLimit falls back to
// DefaultSampleLimit.
func NewScanner(detector *Detector, store ColumnStore, sampleLimit int) *Scanner {
	if sampleLimit <= 0 {
		sampleLimit = DefaultSampleLimit
	}
	return &Scanner{detector: detector, store: store, sampleLimit: sampleLimit}
}

// ScanDatasource detects PII across all columns of a datasource and updates
// their annotations. Manually reviewed columns are never overwritten; stale
// unreviewed annotations are cleared when detection no longer fires. Sample
// fetch failures degrade to name-only detection rather than aborting the scan.
func (s *Scanner) ScanDatasource(ctx context.Context, datasourceID string, fetch SampleFetcher) (*ScanSummary, error) {
	cols, err := s.store.ListColumns(ctx, datasourceID, "", "")
	if err != nil {
		return nil, fmt.Errorf("list columns for pii scan: %w", err)
	}

	summary := &ScanSummary{Detected: make(map[string]int)}
	for _, col := range cols {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if col.PIIReviewedBy != nil {
			continue // manual review wins over auto-detection
		}
		summary.ScannedColumns++

		var samples []string
		if fetch != nil && sampleWorthy(col.DataType) {
			samples, err = fetch(ctx, col, s.sampleLimit)
			if err != nil {
				slog.WarnContext(ctx, "pii sample fetch failed; falling back to name heuristics",
					"datasource_id", datasourceID,
					"column", col.SchemaName+"."+col.TableName+"."+col.ColumnName,
					"error", err)
				samples = nil
			}
		}

		results := s.detector.DetectFromColumn(col.ColumnName, samples)
		if len(results) == 0 {
			if col.PIIType != nil {
				if err := s.store.ClearColumnPII(ctx, col.ID, ""); err != nil {
					return nil, fmt.Errorf("clear stale pii annotation: %w", err)
				}
			}
			continue
		}

		best := results[0]
		if err := s.store.UpdateColumnPIIDetection(ctx, col.ID, best.Type, best.Confidence); err != nil {
			return nil, fmt.Errorf("persist pii detection: %w", err)
		}
		summary.Detected[best.Type]++
	}
	return summary, nil
}

// sampleWorthy reports whether fetching sample data can help detection for
// the given data type. Types that cannot hold free-form PII values are
// skipped to avoid useless queries against the customer database.
func sampleWorthy(dataType string) bool {
	t := strings.ToLower(dataType)
	for _, skip := range []string{"bool", "date", "time", "json", "byte", "uuid", "blob", "binary", "array", "xml", "enum"} {
		if strings.Contains(t, skip) {
			return false
		}
	}
	return true
}
