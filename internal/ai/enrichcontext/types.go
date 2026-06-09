package enrichcontext

// GapKind classifies a missing or conflicting piece of AI context.
type GapKind string

const (
	GapColumnMissingDescription    GapKind = "column_missing_description"
	GapDimensionMissingDescription GapKind = "dimension_missing_description"
	GapMetricMissingDescription    GapKind = "metric_missing_description"
	GapGlossaryMissingDefinition   GapKind = "glossary_missing_definition"
	GapEnumMissingLabel            GapKind = "enum_missing_label"
	GapSynonymCollision            GapKind = "synonym_collision"
)

// Gap is one detected context hole or conflict.
type Gap struct {
	ID        string            `json:"id"`
	Kind      GapKind           `json:"kind"`
	Summary   string            `json:"summary"`
	Detail    string            `json:"detail,omitempty"`
	Entity    map[string]string `json:"entity,omitempty"`
	Applyable bool              `json:"applyable"`
}

// Suggestion is an optional AI- or heuristic-proposed fill for a gap.
type Suggestion struct {
	GapID string `json:"gap_id"`
	Text  string `json:"text"`
}

// AnalyzeRequest drives gap detection for a datasource + semantic model.
type AnalyzeRequest struct {
	DatasourceID string `json:"datasource_id"`
	ModelID      string `json:"model_id"`
	Suggest      bool   `json:"suggest,omitempty"`
}

// AnalyzeResult is the gap report plus optional suggestions.
type AnalyzeResult struct {
	DatasourceID string       `json:"datasource_id"`
	ModelID      string       `json:"model_id"`
	ModelName    string       `json:"model_name"`
	Gaps         []Gap        `json:"gaps"`
	Suggestions  []Suggestion `json:"suggestions,omitempty"`
	SampleRows   int          `json:"sample_rows,omitempty"`
}

// ApplyItem is one user-approved enrichment.
type ApplyItem struct {
	GapID string `json:"gap_id"`
	Value string `json:"value"`
}

// ApplyRequest applies approved enrichments.
type ApplyRequest struct {
	DatasourceID string      `json:"datasource_id"`
	ModelID      string      `json:"model_id"`
	Items        []ApplyItem `json:"items"`
}

// ApplyResult summarizes what was written.
type ApplyResult struct {
	Applied int      `json:"applied"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors,omitempty"`
}
