package ai

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"
	"github.com/biqly/biqly/internal/metadata"
)

// identRegex is the only shape of identifier we are willing to interpolate into
// a sample query. Anything else is rejected before we touch the source DB so a
// caller cannot smuggle SQL through schema/table/column names.
var identRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

func validIdent(s string) bool { return identRegex.MatchString(s) }

// DescribeService generates table/column descriptions from sampled rows using an LLM.
type DescribeService struct {
	client     *Client
	metaRepo   *metadata.Repository
	driverReg  *datasource.Registry
	sampleRows int
}

// NewDescribeService wires the dependencies needed to sample, prompt, and persist descriptions.
func NewDescribeService(client *Client, metaRepo *metadata.Repository, driverReg *datasource.Registry, sampleRows int) *DescribeService {
	if sampleRows <= 0 {
		sampleRows = 10
	}
	return &DescribeService{
		client:     client,
		metaRepo:   metaRepo,
		driverReg:  driverReg,
		sampleRows: sampleRows,
	}
}

// DescribeRequest captures the inputs for a single-table description run.
type DescribeRequest struct {
	DatasourceID string `json:"datasource_id"`
	Schema       string `json:"schema"`
	Table        string `json:"table"`
	SampleSize   int    `json:"sample_size,omitempty"`
	AutoApply    bool   `json:"auto_apply,omitempty"`
}

// DescribeResult is what the AI proposed (and optionally what was persisted).
type DescribeResult struct {
	Table       string             `json:"table"`
	Schema      string             `json:"schema"`
	Description string             `json:"description"`
	Columns     []ColumnDescription `json:"columns"`
	Applied     bool               `json:"applied"`
	SampleRows  int                `json:"sample_rows"`
}

// ColumnDescription is one column's AI-generated description.
type ColumnDescription struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Describe fetches a sample, asks the LLM, parses suggestions, and (optionally) saves them.
func (s *DescribeService) Describe(ctx context.Context, req DescribeRequest) (*DescribeResult, error) {
	if req.DatasourceID == "" || req.Table == "" {
		return nil, fmt.Errorf("datasource_id and table are required")
	}

	limit := req.SampleSize
	if limit <= 0 {
		limit = s.sampleRows
	}

	ds, err := s.metaRepo.GetDatasource(ctx, req.DatasourceID)
	if err != nil {
		return nil, fmt.Errorf("get datasource: %w", err)
	}

	driver, err := s.driverReg.Get(ds.Type)
	if err != nil {
		return nil, fmt.Errorf("driver: %w", err)
	}

	cols, err := s.metaRepo.ListColumns(ctx, ds.ID, req.Schema, req.Table)
	if err != nil {
		return nil, fmt.Errorf("list columns: %w", err)
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("no columns found for %s.%s — run sync-metadata first", req.Schema, req.Table)
	}

	db, err := driver.Open(ctx, ds.DSNEncrypted)
	if err != nil {
		return nil, fmt.Errorf("open datasource: %w", err)
	}
	defer func() { _ = db.Close() }()

	sample, err := s.fetchSample(ctx, db, driver.Dialect(), cols, req.Schema, req.Table, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch sample: %w", err)
	}

	prompt := buildDescribePrompt(req.Schema, req.Table, cols, sample)
	raw, err := s.client.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("ai generate: %w", err)
	}

	tableDesc, colDescs, err := parseDescribeResponse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse ai response: %w", err)
	}

	result := &DescribeResult{
		Schema:      req.Schema,
		Table:       req.Table,
		Description: tableDesc,
		Columns:     colDescs,
		SampleRows:  len(sample),
	}

	if req.AutoApply {
		if err := s.apply(ctx, cols, result); err != nil {
			return result, fmt.Errorf("apply descriptions: %w", err)
		}
		result.Applied = true
	}

	return result, nil
}

func (s *DescribeService) fetchSample(ctx context.Context, db *sql.DB, d dialect.Dialect, cols []metadata.Column, schema, table string, limit int) ([]map[string]any, error) {
	// SQL placeholders cannot bind identifiers, so we hard-validate every
	// schema / table / column name against an allowlist regex before letting
	// it near the query. Names that survive this gate cannot encode SQL.
	if table != "" && !validIdent(table) {
		return nil, fmt.Errorf("invalid table identifier: %q", table)
	}
	if schema != "" && !validIdent(schema) {
		return nil, fmt.Errorf("invalid schema identifier: %q", schema)
	}
	colIdents := make([]string, 0, len(cols))
	for _, c := range cols {
		if !validIdent(c.ColumnName) {
			return nil, fmt.Errorf("invalid column identifier: %q", c.ColumnName)
		}
		colIdents = append(colIdents, d.QuoteIdent(c.ColumnName))
	}
	from := d.QuoteIdent(table)
	if schema != "" {
		from = d.QuoteIdent(schema) + "." + d.QuoteIdent(table)
	}
	// Identifiers are validated above; LimitOffset emits an integer literal.
	//nolint:gosec // identifiers validated against allowlist regex above
	query := fmt.Sprintf("SELECT %s FROM %s %s",
		strings.Join(colIdents, ", "),
		from,
		d.LimitOffset(limit, 0),
	)

	// nosemgrep
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	colNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var out []map[string]any
	for rows.Next() {
		holders := make([]any, len(colNames))
		ptrs := make([]any, len(colNames))
		for i := range holders {
			ptrs[i] = &holders[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(colNames))
		for i, name := range colNames {
			v := holders[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			row[name] = v
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *DescribeService) apply(ctx context.Context, cols []metadata.Column, result *DescribeResult) error {
	colByName := make(map[string]metadata.Column, len(cols))
	for _, c := range cols {
		colByName[c.ColumnName] = c
	}

	if result.Description != "" && len(cols) > 0 {
		desc := result.Description
		// Apply to the first matching table_id (all sampled cols share the same table).
		if err := s.metaRepo.UpdateTableDescription(ctx, cols[0].TableID, &desc); err != nil {
			return fmt.Errorf("update table description: %w", err)
		}
	}

	for _, cd := range result.Columns {
		if cd.Description == "" {
			continue
		}
		col, ok := colByName[cd.Name]
		if !ok {
			continue
		}
		desc := cd.Description
		if err := s.metaRepo.UpdateColumnDescription(ctx, col.ID, &desc); err != nil {
			return fmt.Errorf("update column %s: %w", cd.Name, err)
		}
	}
	return nil
}

func buildDescribePrompt(schema, table string, cols []metadata.Column, sample []map[string]any) string {
	var sb strings.Builder
	sb.WriteString("You are a data documentation assistant. Examine the table schema and sample rows, then write concise, business-friendly descriptions.\n\n")
	sb.WriteString("## Rules\n")
	sb.WriteString("- Output ONLY valid JSON matching the schema below. No markdown, no explanation.\n")
	sb.WriteString("- Keep descriptions under 200 characters each.\n")
	sb.WriteString("- Describe the business meaning, not the data type.\n")
	sb.WriteString("- If you cannot infer a column from the sample, leave its description empty.\n\n")

	fmt.Fprintf(&sb, "## Table: %s.%s\n", schema, table)
	sb.WriteString("### Columns\n")
	for _, c := range cols {
		line := fmt.Sprintf("- %s (%s", c.ColumnName, c.DataType)
		if c.IsPrimaryKey {
			line += ", primary key"
		}
		if c.IsForeignKey && c.ReferencedTable != nil {
			line += fmt.Sprintf(", FK -> %s.%s", *c.ReferencedTable, derefStr(c.ReferencedColumn))
		}
		line += ")"
		if c.Description != nil && *c.Description != "" {
			line += fmt.Sprintf(" — current: %q", *c.Description)
		}
		sb.WriteString(line + "\n")
	}

	sb.WriteString("\n### Sample Rows (JSON)\n")
	if data, err := json.MarshalIndent(sample, "", "  "); err == nil {
		sb.Write(data)
		sb.WriteString("\n")
	}

	sb.WriteString("\n## Output Schema\n")
	sb.WriteString(`{"table_description": "...", "columns": [{"name": "col_name", "description": "..."}]}`)

	return sb.String()
}

func parseDescribeResponse(raw string) (string, []ColumnDescription, error) {
	cleaned := raw
	if idx := strings.Index(cleaned, "```json"); idx >= 0 {
		cleaned = cleaned[idx+7:]
		if end := strings.Index(cleaned, "```"); end >= 0 {
			cleaned = cleaned[:end]
		}
	} else if idx := strings.Index(cleaned, "```"); idx >= 0 {
		cleaned = cleaned[idx+3:]
		if end := strings.Index(cleaned, "```"); end >= 0 {
			cleaned = cleaned[:end]
		}
	}
	cleaned = strings.TrimSpace(cleaned)

	var payload struct {
		TableDescription string              `json:"table_description"`
		Columns          []ColumnDescription `json:"columns"`
	}
	if err := json.Unmarshal([]byte(cleaned), &payload); err != nil {
		return "", nil, fmt.Errorf("invalid JSON from AI: %w", err)
	}
	return payload.TableDescription, payload.Columns, nil
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
