package semantic

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

// ModelFileSchemaVersion is the current schema version of the portable
// semantic model file format.
const ModelFileSchemaVersion = "v1"

// ModelFile is the portable, git-friendly representation of a semantic model.
// It contains only definitional fields — no IDs, timestamps, or status — and
// serializes deterministically (stable field order, children sorted by name)
// so exports diff cleanly under version control.
type ModelFile struct {
	SchemaVersion   string               `yaml:"biqly_semantic_model" json:"biqly_semantic_model"`
	Name            string               `yaml:"name" json:"name"`
	Label           string               `yaml:"label,omitempty" json:"label,omitempty"`
	Description     string               `yaml:"description,omitempty" json:"description,omitempty"`
	BaseSchema      string               `yaml:"base_schema" json:"base_schema"`
	BaseTable       string               `yaml:"base_table" json:"base_table"`
	Synonyms        []string             `yaml:"synonyms,omitempty" json:"synonyms,omitempty"`
	ExcludedSchemas []string             `yaml:"excluded_schemas,omitempty" json:"excluded_schemas,omitempty"`
	Dimensions      []ModelFileDimension `yaml:"dimensions,omitempty" json:"dimensions,omitempty"`
	Metrics         []ModelFileMetric    `yaml:"metrics,omitempty" json:"metrics,omitempty"`
	Joins           []ModelFileJoin      `yaml:"joins,omitempty" json:"joins,omitempty"`
}

// ModelFileDimension is the portable form of a Dimension.
type ModelFileDimension struct {
	Name                 string               `yaml:"name" json:"name"`
	Label                string               `yaml:"label,omitempty" json:"label,omitempty"`
	ColumnRef            string               `yaml:"column_ref,omitempty" json:"column_ref,omitempty"`
	Type                 string               `yaml:"type" json:"type"`
	TimeGrain            string               `yaml:"time_grain,omitempty" json:"time_grain,omitempty"`
	Synonyms             []string             `yaml:"synonyms,omitempty" json:"synonyms,omitempty"`
	Description          string               `yaml:"description,omitempty" json:"description,omitempty"`
	CalculatedExpression string               `yaml:"calculated_expression,omitempty" json:"calculated_expression,omitempty"`
	EnumValues           []ModelFileEnumValue `yaml:"enum_values,omitempty" json:"enum_values,omitempty"`
}

// ModelFileEnumValue is the portable form of an EnumMapping.
type ModelFileEnumValue struct {
	RawValue    string `yaml:"raw_value" json:"raw_value"`
	Label       string `yaml:"label" json:"label"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	SortOrder   int    `yaml:"sort_order,omitempty" json:"sort_order,omitempty"`
}

// ModelFileMetric is the portable form of a Metric.
type ModelFileMetric struct {
	Name         string   `yaml:"name" json:"name"`
	Label        string   `yaml:"label,omitempty" json:"label,omitempty"`
	Expression   string   `yaml:"expression" json:"expression"`
	Aggregation  string   `yaml:"aggregation" json:"aggregation"`
	Format       string   `yaml:"format,omitempty" json:"format,omitempty"`
	Synonyms     []string `yaml:"synonyms,omitempty" json:"synonyms,omitempty"`
	Description  string   `yaml:"description,omitempty" json:"description,omitempty"`
	RateBehavior string   `yaml:"rate_behavior,omitempty" json:"rate_behavior,omitempty"`
}

// ModelFileJoin is the portable form of a Join.
type ModelFileJoin struct {
	Name         string `yaml:"name" json:"name"`
	FromSchema   string `yaml:"from_schema,omitempty" json:"from_schema,omitempty"`
	FromTable    string `yaml:"from_table" json:"from_table"`
	FromColumn   string `yaml:"from_column" json:"from_column"`
	ToSchema     string `yaml:"to_schema,omitempty" json:"to_schema,omitempty"`
	ToTable      string `yaml:"to_table" json:"to_table"`
	ToColumn     string `yaml:"to_column" json:"to_column"`
	JoinType     string `yaml:"join_type" json:"join_type"`
	Relationship string `yaml:"relationship" json:"relationship"`
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// NewModelFile converts a full semantic model into its portable file form.
// Only active dimensions, metrics, and joins are included; children are
// sorted by name for deterministic output.
func NewModelFile(m *SemanticModel) ModelFile {
	f := ModelFile{
		SchemaVersion:   ModelFileSchemaVersion,
		Name:            m.Name,
		Label:           deref(m.Label),
		Description:     deref(m.Description),
		BaseSchema:      m.BaseSchema,
		BaseTable:       m.BaseTable,
		Synonyms:        m.Synonyms,
		ExcludedSchemas: m.ExcludedSchemas,
	}
	for i := range m.Dimensions {
		d := &m.Dimensions[i]
		if !d.IsActive {
			continue
		}
		fd := ModelFileDimension{
			Name:                 d.Name,
			Label:                deref(d.Label),
			ColumnRef:            d.ColumnRef,
			Type:                 d.Type,
			TimeGrain:            d.TimeGrain,
			Synonyms:             d.Synonyms,
			Description:          deref(d.Description),
			CalculatedExpression: d.CalculatedExpression,
		}
		for _, ev := range d.EnumValues {
			fd.EnumValues = append(fd.EnumValues, ModelFileEnumValue{
				RawValue:    ev.RawValue,
				Label:       ev.Label,
				Description: deref(ev.Description),
				SortOrder:   ev.SortOrder,
			})
		}
		f.Dimensions = append(f.Dimensions, fd)
	}
	for i := range m.Metrics {
		mt := &m.Metrics[i]
		if !mt.IsActive {
			continue
		}
		f.Metrics = append(f.Metrics, ModelFileMetric{
			Name:         mt.Name,
			Label:        deref(mt.Label),
			Expression:   mt.Expression,
			Aggregation:  mt.Aggregation,
			Format:       deref(mt.Format),
			Synonyms:     mt.Synonyms,
			Description:  deref(mt.Description),
			RateBehavior: mt.RateBehavior,
		})
	}
	for i := range m.Joins {
		j := &m.Joins[i]
		if !j.IsActive {
			continue
		}
		f.Joins = append(f.Joins, ModelFileJoin{
			Name:         j.Name,
			FromSchema:   j.FromSchema,
			FromTable:    j.FromTable,
			FromColumn:   j.FromColumn,
			ToSchema:     j.ToSchema,
			ToTable:      j.ToTable,
			ToColumn:     j.ToColumn,
			JoinType:     j.JoinType,
			Relationship: j.Relationship,
		})
	}
	slices.SortFunc(f.Dimensions, func(a, b ModelFileDimension) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(f.Metrics, func(a, b ModelFileMetric) int { return strings.Compare(a.Name, b.Name) })
	slices.SortFunc(f.Joins, func(a, b ModelFileJoin) int { return strings.Compare(a.Name, b.Name) })
	return f
}

// MarshalModelFile serializes a model file to deterministic YAML.
func MarshalModelFile(f ModelFile) ([]byte, error) {
	out, err := yaml.Marshal(f)
	if err != nil {
		return nil, fmt.Errorf("marshal model file: %w", err)
	}
	return out, nil
}

// ParseModelFile parses YAML (or JSON, a YAML subset) into a ModelFile and
// checks the schema version and required fields.
func ParseModelFile(data []byte) (*ModelFile, error) {
	var f ModelFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse model file: %w", err)
	}
	if f.SchemaVersion != ModelFileSchemaVersion {
		return nil, fmt.Errorf("unsupported biqly_semantic_model version %q (expected %q)", f.SchemaVersion, ModelFileSchemaVersion)
	}
	if strings.TrimSpace(f.Name) == "" {
		return nil, errors.New("model file: name is required")
	}
	if strings.TrimSpace(f.BaseTable) == "" {
		return nil, errors.New("model file: base_table is required")
	}
	for i := range f.Metrics {
		if !pkgsemantic.IsValidRateBehavior(f.Metrics[i].RateBehavior) {
			return nil, fmt.Errorf("model file: metric %q has invalid rate_behavior %q", f.Metrics[i].Name, f.Metrics[i].RateBehavior)
		}
	}
	return &f, nil
}

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// Model builds an in-memory SemanticModel from the file for the given
// datasource. IDs are left empty — the caller assigns them before persisting.
func (f *ModelFile) Model(datasourceID string) *SemanticModel {
	m := &SemanticModel{
		DatasourceID:    datasourceID,
		Name:            f.Name,
		Label:           optStr(f.Label),
		Description:     optStr(f.Description),
		BaseSchema:      f.BaseSchema,
		BaseTable:       f.BaseTable,
		Synonyms:        f.Synonyms,
		ExcludedSchemas: f.ExcludedSchemas,
		IsActive:        true,
	}
	for _, fd := range f.Dimensions {
		d := Dimension{
			Name:                 fd.Name,
			Label:                optStr(fd.Label),
			ColumnRef:            fd.ColumnRef,
			Type:                 fd.Type,
			TimeGrain:            fd.TimeGrain,
			Synonyms:             fd.Synonyms,
			Description:          optStr(fd.Description),
			CalculatedExpression: fd.CalculatedExpression,
			IsActive:             true,
		}
		for _, ev := range fd.EnumValues {
			d.EnumValues = append(d.EnumValues, EnumMapping{
				RawValue:    ev.RawValue,
				Label:       ev.Label,
				Description: optStr(ev.Description),
				SortOrder:   ev.SortOrder,
			})
		}
		m.Dimensions = append(m.Dimensions, d)
	}
	for _, fm := range f.Metrics {
		m.Metrics = append(m.Metrics, Metric{
			Name:         fm.Name,
			Label:        optStr(fm.Label),
			Expression:   fm.Expression,
			Aggregation:  fm.Aggregation,
			Format:       optStr(fm.Format),
			Synonyms:     fm.Synonyms,
			Description:  optStr(fm.Description),
			IsActive:     true,
			RateBehavior: fm.RateBehavior,
		})
	}
	for _, fj := range f.Joins {
		m.Joins = append(m.Joins, Join{
			Name:         fj.Name,
			FromSchema:   fj.FromSchema,
			FromTable:    fj.FromTable,
			FromColumn:   fj.FromColumn,
			ToSchema:     fj.ToSchema,
			ToTable:      fj.ToTable,
			ToColumn:     fj.ToColumn,
			JoinType:     fj.JoinType,
			Relationship: fj.Relationship,
			IsActive:     true,
		})
	}
	hydrateExpressionASTs(m)
	return m
}
