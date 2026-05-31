package prompt

import (
	"bytes"
	"fmt"
	"strings"
)

// normalizeDialectName maps datasource driver types and few-shot dialect labels
// to a canonical key used for prompt example selection.
func normalizeDialectName(name string) string {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "postgres", "postgresql", "pg":
		return "postgres"
	case "mysql", "mariadb":
		return "mysql"
	case "sqlserver", "mssql", "sql_server":
		return "sqlserver"
	case "clickhouse", "ch":
		return "clickhouse"
	default:
		return "postgres"
	}
}

func (b *PromptBuilder) writeDialectCompilationGuide(sb *bytes.Buffer, targetDialect string) {
	key := normalizeDialectName(targetDialect)
	if key == "" {
		key = "postgres"
	}

	sb.WriteString("\n\n## Datasource SQL Dialect\n")
	fmt.Fprintf(sb, "Target engine: **%s**. You output **LogicalQuery JSON only** — never raw SQL. The backend compiles your JSON to %s-specific SQL (parameterized, quoted identifiers).\n\n",
		key, key)

	sb.WriteString("### Same LogicalQuery, different compiled SQL\n")
	sb.WriteString("Illustrative fragments for one filter + one monthly grain dimension (catalog has `order_date_month`, `order_date`, `revenue` metric on `amount`):\n\n")
	sb.WriteString("LogicalQuery (always emit this shape):\n")
	sb.WriteString(`{"select":[{"type":"dimension","name":"order_date_month"},{"type":"metric","name":"revenue"}],"filters":[{"field":"order_date","operator":"gte","value":"2024-01-01"}],"group_by":[{"field":"order_date_month"}],"limit":100}`)
	sb.WriteString("\n\n")

	type dialectRow struct {
		key   string
		label string
		month string
		gte   string
		like  string
		limit string
	}
	rows := []dialectRow{
		{
			key:   "postgres",
			label: "PostgreSQL",
			month: `CAST(EXTRACT(MONTH FROM "order_date") AS INTEGER) AS "order_month"`,
			gte:   `"order_date" >= $1`,
			like:  `"name" ILIKE $1`,
			limit: `LIMIT 100`,
		},
		{
			key:   "mysql",
			label: "MySQL",
			month: `MONTH(` + "`order_date`" + `) AS ` + "`order_month`",
			gte:   "`order_date` >= ?",
			like:  "LOWER(`name`) LIKE LOWER(?)",
			limit: "LIMIT 100",
		},
		{
			key:   "sqlserver",
			label: "SQL Server",
			month: `MONTH([order_date]) AS [order_month]`,
			gte:   `[order_date] >= @p1`,
			like:  `[name] LIKE @p1`,
			limit: "OFFSET 0 ROWS FETCH NEXT 100 ROWS ONLY",
		},
		{
			key:   "clickhouse",
			label: "ClickHouse",
			month: "toMonth(`order_date`) AS `order_month`",
			gte:   "`order_date` >= ?",
			like:  "lowerUTF8(`name`) LIKE lowerUTF8(?)",
			limit: "LIMIT 100",
		},
	}

	for _, r := range rows {
		marker := ""
		if r.key == key {
			marker = " ← **this datasource**"
		}
		fmt.Fprintf(sb, "**%s**%s\n", r.label, marker)
		fmt.Fprintf(sb, "- Month grain in SELECT/GROUP BY: %s\n", r.month)
		fmt.Fprintf(sb, "- Date filter (gte): %s\n", r.gte)
		fmt.Fprintf(sb, "- `contains` on text: %s\n", r.like)
		fmt.Fprintf(sb, "- Row cap: %s\n\n", r.limit)
	}

	sb.WriteString("Use catalog dimension/metric **names** in LogicalQuery; the compiler applies the dialect rules above.\n")
}

func (b *PromptBuilder) writeFailureExamples(sb *bytes.Buffer) {
	sb.WriteString("\n\n## Examples — Common Mistakes (do NOT do this)\n")
	sb.WriteString("Output must be corrected LogicalQuery JSON. These pairs show frequent errors.\n\n")

	pairs := []struct {
		title string
		bad   string
		good  string
		note  string
	}{
		{
			title: "Raw SQL instead of LogicalQuery",
			bad:   "SELECT country, COUNT(*) FROM orders GROUP BY country",
			good:  `{"select":[{"type":"dimension","name":"country"},{"type":"metric","name":"row_count"}],"group_by":[{"field":"country"}],"limit":100}`,
			note:  "Never return SQL, markdown fences, or prose — JSON object only.",
		},
		{
			title: "Invented or qualified field names",
			bad:   `{"filters":[{"field":"orders.customer_name","operator":"eq","value":"Acme"}]}`,
			good:  `{"filters":[{"field":"name","operator":"eq","value":"Acme"}]}`,
			note:  "Use exact dimension/metric names from the catalog — not table.column unless that name is listed.",
		},
		{
			title: "SQL expressions inside JSON field slots",
			bad:   `{"select":[{"type":"dimension","name":"year(order_date)"}]}`,
			good:  `{"select":[{"type":"dimension","name":"order_date_year"}],"group_by":[{"field":"order_date_year"}]}`,
			note:  "No functions in `name`/`field` — pick the exact listed grain dimension (e.g. order_date_year, order_date_month).",
		},
		{
			title: "Year filter on raw timestamp column",
			bad:   `{"filters":[{"field":"order_date","operator":"eq","value":2024}]}`,
			good:  `{"filters":[{"field":"order_date_year","operator":"eq","value":2024}]}`,
			note:  "Compare integers to `*_year` / `*_month` dimensions, or ISO strings to raw date columns.",
		},
		{
			title: "Aggregate threshold in filters (pre-aggregation)",
			bad:   `{"filters":[{"field":"order_count","operator":"gt","value":10}]}`,
			good:  `{"having":[{"field":"order_count","operator":"gt","value":10}]}`,
			note:  "Post-aggregate conditions belong in `having`, not `filters`.",
		},
		{
			title: "Group-by dimension missing from select",
			bad:   `{"select":[{"type":"metric","name":"revenue"}],"group_by":[{"field":"country"}]}`,
			good:  `{"select":[{"type":"dimension","name":"country"},{"type":"metric","name":"revenue"}],"group_by":[{"field":"country"}]}`,
			note:  "Every `group_by` field needs a matching dimension in `select`.",
		},
		{
			title: "Unquoted JSON keys (invalid JSON)",
			bad:   `{select:[{type:metric,name:row_count}],limit:100}`,
			good:  `{"select":[{"type":"metric","name":"row_count"}],"limit":100}`,
			note:  "RFC 8259: all property names must be double-quoted.",
		},
	}

	for i, p := range pairs {
		fmt.Fprintf(sb, "%d. **%s**\n", i+1, p.title)
		fmt.Fprintf(sb, "Wrong: %s\n", p.bad)
		fmt.Fprintf(sb, "Right: %s\n", p.good)
		if p.note != "" {
			fmt.Fprintf(sb, "Why: %s\n", p.note)
		}
		sb.WriteString("\n")
	}
}

func (b *PromptBuilder) writePlanningSteps(sb *bytes.Buffer) {
	sb.WriteString("\n\n## Planning Steps (chain-of-thought — follow in order)\n")
	sb.WriteString("Work through every step below using **only** the semantic catalog above, then produce the LogicalQuery JSON.\n")
	sb.WriteString("You may echo these steps in an optional `## Reasoning` block immediately before the JSON (see Output Format).\n\n")

	steps := []struct {
		title string
		body  string
	}{
		{
			title: "1. Parse the question",
			body:  "Identify intent: aggregate (how many/total/average), list/detail, trend over time, ranking (top/highest/most), comparison, or filter-only. Note Turkish/English time phrases and entity words (müşteri, sipariş, silinen, aylık, …).",
		},
		{
			title: "2. Map entities to tables",
			body:  "Start from the base table. Add joins from **Available Joins** only when the question needs columns from another table. Do not invent tables or join paths.",
		},
		{
			title: "3. Select metrics",
			body:  "Pick metric names from **Available Metrics** (match synonyms). Use `row_count` for row counts; sum/avg/min/max/count_distinct only when the question asks for that measure. Never invent metrics.",
		},
		{
			title: "4. Select dimensions",
			body:  "For readable labels use **Display Dimensions**. For time breakdowns use exact listed grain dimensions like `order_date_year`, `order_date_month`, `order_date_day`; do not shorten them to `order_year` or `order_month`. If no listed grain exists, use the raw date dimension with `time_grain` on `group_by` as instructed in Rules. Avoid `*_id` columns unless the user asks for ids/codes.",
		},
		{
			title: "5. Decide filters vs group_by",
			body:  "Period constraints (“in 2026”, “May 2024”, “last quarter”) → `filters` on grain or ISO date dimensions. Breakdown triggers (“by month”, “per customer”, “bazında”, “aylık”) → matching dimensions in **both** `select` and `group_by`. Soft-delete wording → deletion-indicator filters per Rules.",
		},
		{
			title: "6. Post-aggregation and windows",
			body:  "Thresholds on aggregates (“more than 10 orders”) → `having` on metric names. Running totals, ranks, moving averages → `window` select items per Rules.",
		},
		{
			title: "7. Order, limit, and sort",
			body:  "Top-N / highest → `order_by` metric `desc` and small `limit`. Time series → `order_by` grain dimensions `asc` (coarsest to finest when multiple grains).",
		},
		{
			title: "8. Build and verify JSON",
			body:  "Assemble LogicalQuery: every `field`/`name` is an exact catalog dimension or metric name; RFC 8259 double-quoted keys; include every column the question asks for; empty `select` only if the catalog cannot answer.",
		},
	}

	for _, step := range steps {
		fmt.Fprintf(sb, "**%s** — %s\n\n", step.title, step.body)
	}
}
