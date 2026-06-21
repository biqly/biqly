package pii

import (
	"context"
	"errors"
	"testing"

	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeColumnStore struct {
	columns    []pkgmetadata.Column
	listErr    error
	detections map[string]PIIResult // column ID -> persisted detection
	cleared    map[string]bool      // column ID -> annotation cleared
	updateErr  error
}

func newFakeColumnStore(cols ...pkgmetadata.Column) *fakeColumnStore {
	return &fakeColumnStore{
		columns:    cols,
		detections: make(map[string]PIIResult),
		cleared:    make(map[string]bool),
	}
}

func (f *fakeColumnStore) ListColumns(_ context.Context, _, _, _ string) ([]pkgmetadata.Column, error) {
	return f.columns, f.listErr
}

func (f *fakeColumnStore) UpdateColumnPIIDetection(_ context.Context, id, piiType string, confidence float64) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.detections[id] = PIIResult{Type: piiType, Confidence: confidence}
	return nil
}

func (f *fakeColumnStore) ClearColumnPII(_ context.Context, id string, _ string) error {
	f.cleared[id] = true
	return nil
}

func col(id, name, dataType string) pkgmetadata.Column {
	return pkgmetadata.Column{
		ID:         id,
		SchemaName: "public",
		TableName:  "customers",
		ColumnName: name,
		DataType:   dataType,
	}
}

func TestScanDatasource_DetectsAndPersists(t *testing.T) {
	store := newFakeColumnStore(
		col("c1", "email", "varchar"),
		col("c2", "phone", "varchar"),
		col("c3", "status", "varchar"),
	)
	sampleData := map[string][]string{
		"email":  {"a@example.com", "b@example.com"},
		"phone":  {"05551234567", "05557654321"},
		"status": {"active", "inactive"},
	}
	fetch := func(_ context.Context, c pkgmetadata.Column, _ int) ([]string, error) {
		return sampleData[c.ColumnName], nil
	}

	s := NewScanner(NewDetector(DefaultThreshold), store, 10)
	summary, err := s.ScanDatasource(context.Background(), "ds-1", fetch)
	require.NoError(t, err)

	assert.Equal(t, 3, summary.ScannedColumns)
	assert.Equal(t, map[string]int{TypeEmail: 1, TypePhone: 1}, summary.Detected)
	assert.Equal(t, TypeEmail, store.detections["c1"].Type)
	assert.Equal(t, TypePhone, store.detections["c2"].Type)
	assert.NotContains(t, store.detections, "c3")
}

func TestScanDatasource_SkipsReviewedColumns(t *testing.T) {
	reviewed := col("c1", "email", "varchar")
	reviewed.PIIReviewedBy = new("admin@biqly.com")

	store := newFakeColumnStore(reviewed)
	s := NewScanner(NewDetector(DefaultThreshold), store, 10)

	summary, err := s.ScanDatasource(context.Background(), "ds-1", nil)
	require.NoError(t, err)
	assert.Equal(t, 0, summary.ScannedColumns)
	assert.Empty(t, store.detections)
}

func TestScanDatasource_ClearsStaleAnnotations(t *testing.T) {
	stale := col("c1", "renamed_col", "varchar")
	stale.PIIType = new(TypeEmail) // previously detected, no longer matches

	store := newFakeColumnStore(stale)
	s := NewScanner(NewDetector(DefaultThreshold), store, 10)

	_, err := s.ScanDatasource(context.Background(), "ds-1", nil)
	require.NoError(t, err)
	assert.True(t, store.cleared["c1"])
}

func TestScanDatasource_SampleFetchFailureFallsBackToName(t *testing.T) {
	store := newFakeColumnStore(col("c1", "customer_email", "varchar"))
	fetch := func(_ context.Context, _ pkgmetadata.Column, _ int) ([]string, error) {
		return nil, errors.New("connection lost")
	}

	// Threshold below the name-only score: detection still succeeds from the
	// column name even though sampling failed.
	s := NewScanner(NewDetector(0.4), store, 10)
	summary, err := s.ScanDatasource(context.Background(), "ds-1", fetch)
	require.NoError(t, err)
	assert.Equal(t, map[string]int{TypeEmail: 1}, summary.Detected)
}

func TestScanDatasource_SkipsSamplingForNonTextTypes(t *testing.T) {
	store := newFakeColumnStore(col("c1", "created_at", "timestamptz"))
	fetchCalled := false
	fetch := func(_ context.Context, _ pkgmetadata.Column, _ int) ([]string, error) {
		fetchCalled = true
		return nil, nil
	}

	s := NewScanner(NewDetector(DefaultThreshold), store, 10)
	_, err := s.ScanDatasource(context.Background(), "ds-1", fetch)
	require.NoError(t, err)
	assert.False(t, fetchCalled)
}

func TestScanDatasource_ListError(t *testing.T) {
	store := newFakeColumnStore()
	store.listErr = errors.New("db down")

	s := NewScanner(NewDetector(DefaultThreshold), store, 10)
	_, err := s.ScanDatasource(context.Background(), "ds-1", nil)
	assert.Error(t, err)
}

func TestSampleWorthy(t *testing.T) {
	assert.True(t, sampleWorthy("varchar"))
	assert.True(t, sampleWorthy("text"))
	assert.True(t, sampleWorthy("bigint"))
	assert.False(t, sampleWorthy("boolean"))
	assert.False(t, sampleWorthy("timestamptz"))
	assert.False(t, sampleWorthy("jsonb"))
	assert.False(t, sampleWorthy("uuid"))
	assert.False(t, sampleWorthy("bytea"))
}

func TestNewScanner_ZeroSampleLimitDefaults(t *testing.T) {
	s := NewScanner(NewDetector(DefaultThreshold), newFakeColumnStore(), 0)
	assert.Equal(t, DefaultSampleLimit, s.sampleLimit)
}

func TestNewScanner_NegativeSampleLimitDefaults(t *testing.T) {
	s := NewScanner(NewDetector(DefaultThreshold), newFakeColumnStore(), -5)
	assert.Equal(t, DefaultSampleLimit, s.sampleLimit)
}

func TestNewScanner_PositiveSampleLimitPreserved(t *testing.T) {
	s := NewScanner(NewDetector(DefaultThreshold), newFakeColumnStore(), 25)
	assert.Equal(t, 25, s.sampleLimit)
}
