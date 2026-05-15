package ai

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

type memoryOrderRow struct {
	Country string
	Status  string
	Amount  float64
}

var defaultOrdersSeed = []memoryOrderRow{
	{Country: "TR", Status: "shipped", Amount: 100},
	{Country: "TR", Status: "pending", Amount: 50},
	{Country: "DE", Status: "shipped", Amount: 200},
	{Country: "DE", Status: "shipped", Amount: 300},
	{Country: "US", Status: "cancelled", Amount: 10},
}

// MemoryResultExecutor evaluates LogicalQueries against the built-in orders
// seed without a real database. Used for execution-accuracy golden tests.
type MemoryResultExecutor struct{}

func (MemoryResultExecutor) Execute(_ context.Context, model *semantic.SemanticModel, lq *query.LogicalQuery) (*query.Result, error) {
	if model == nil || lq == nil {
		return nil, fmt.Errorf("model and logical query are required")
	}
	if model.Name != "public.orders" && model.BaseTable != "orders" {
		return nil, fmt.Errorf("memory executor only supports the golden orders model")
	}
	rows := filterMemoryRows(defaultOrdersSeed, lq.Filters, model)
	if len(lq.GroupBy) == 0 {
		return aggregateMemoryRows(rows, lq.Select, model, nil)
	}
	return groupMemoryRows(rows, lq, model)
}

func filterMemoryRows(seed []memoryOrderRow, filters []query.Filter, model *semantic.SemanticModel) []memoryOrderRow {
	out := make([]memoryOrderRow, 0, len(seed))
rows:
	for _, r := range seed {
		for _, f := range filters {
			if !memoryFilterMatch(r, f, model) {
				continue rows
			}
		}
		out = append(out, r)
	}
	return out
}

func memoryFilterMatch(r memoryOrderRow, f query.Filter, model *semantic.SemanticModel) bool {
	col := memoryColumnForField(f.Field, model)
	val := memoryFieldValue(r, col)
	switch strings.ToLower(f.Operator) {
	case "eq", "=":
		return fmt.Sprint(val) == fmt.Sprint(f.Value)
	case "neq", "!=":
		return fmt.Sprint(val) != fmt.Sprint(f.Value)
	default:
		return fmt.Sprint(val) == fmt.Sprint(f.Value)
	}
}

func memoryColumnForField(field string, model *semantic.SemanticModel) string {
	for _, d := range model.Dimensions {
		if d.Name == field {
			parts := strings.Split(d.ColumnRef, ".")
			if len(parts) > 0 {
				return parts[len(parts)-1]
			}
		}
	}
	return field
}

func memoryFieldValue(r memoryOrderRow, col string) any {
	switch col {
	case "country":
		return r.Country
	case "status":
		return r.Status
	case "amount":
		return r.Amount
	default:
		return nil
	}
}

func groupMemoryRows(rows []memoryOrderRow, lq *query.LogicalQuery, model *semantic.SemanticModel) (*query.Result, error) {
	if len(lq.GroupBy) != 1 {
		return nil, fmt.Errorf("memory executor supports single-dimension group_by only")
	}
	gbField := lq.GroupBy[0].Field
	gbCol := memoryColumnForField(gbField, model)
	buckets := make(map[string][]memoryOrderRow)
	for _, r := range rows {
		key := fmt.Sprint(memoryFieldValue(r, gbCol))
		buckets[key] = append(buckets[key], r)
	}
	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	resultRows := make([][]any, 0, len(keys))
	for _, k := range keys {
		part, err := aggregateMemoryRows(buckets[k], lq.Select, model, map[string]any{gbField: k})
		if err != nil {
			return nil, err
		}
		if len(part.Rows) != 1 {
			return nil, fmt.Errorf("unexpected bucket row count for %q", k)
		}
		resultRows = append(resultRows, part.Rows[0])
	}
	cols, err := memoryResultColumns(lq.Select, model, gbField)
	if err != nil {
		return nil, err
	}
	return &query.Result{Columns: cols, Rows: resultRows}, nil
}

func aggregateMemoryRows(rows []memoryOrderRow, selectItems []query.SelectItem, model *semantic.SemanticModel, dimVals map[string]any) (*query.Result, error) {
	outRow := make([]any, 0, len(selectItems))
	cols := make([]query.ResultColumn, 0, len(selectItems))
	for _, item := range selectItems {
		switch item.Type {
		case "dimension":
			if dimVals != nil {
				outRow = append(outRow, dimVals[item.Name])
			} else {
				outRow = append(outRow, nil)
			}
			cols = append(cols, query.ResultColumn{Name: item.Name, Type: "text", SemanticType: query.SemanticTypeDimension})
		case "metric":
			val, err := memoryMetricValue(rows, item.Name, model)
			if err != nil {
				return nil, err
			}
			outRow = append(outRow, val)
			cols = append(cols, query.ResultColumn{Name: item.Name, Type: "number", SemanticType: query.SemanticTypeMetric})
		default:
			return nil, fmt.Errorf("unsupported select type %q in memory executor", item.Type)
		}
	}
	return &query.Result{Columns: cols, Rows: [][]any{outRow}}, nil
}

func memoryMetricValue(rows []memoryOrderRow, name string, model *semantic.SemanticModel) (float64, error) {
	var metric *semantic.Metric
	for i := range model.Metrics {
		if model.Metrics[i].Name == name {
			metric = &model.Metrics[i]
			break
		}
	}
	if metric == nil {
		return 0, fmt.Errorf("unknown metric %q", name)
	}
	switch strings.ToLower(metric.Aggregation) {
	case "count":
		if metric.Expression == "*" {
			return float64(len(rows)), nil
		}
		return float64(len(rows)), nil
	case "sum":
		var sum float64
		col := "amount"
		if parts := strings.Split(metric.Expression, "."); len(parts) > 0 {
			col = parts[len(parts)-1]
		}
		for _, r := range rows {
			if v, ok := memoryFieldValue(r, col).(float64); ok {
				sum += v
			}
		}
		return sum, nil
	default:
		return 0, fmt.Errorf("unsupported aggregation %q", metric.Aggregation)
	}
}

func memoryResultColumns(selectItems []query.SelectItem, model *semantic.SemanticModel, gbField string) ([]query.ResultColumn, error) {
	cols := make([]query.ResultColumn, 0, len(selectItems))
	for _, item := range selectItems {
		switch item.Type {
		case "dimension":
			cols = append(cols, query.ResultColumn{Name: item.Name, Type: "text", SemanticType: query.SemanticTypeDimension})
		case "metric":
			cols = append(cols, query.ResultColumn{Name: item.Name, Type: "number", SemanticType: query.SemanticTypeMetric})
		default:
			return nil, fmt.Errorf("unsupported select type %q", item.Type)
		}
	}
	_ = gbField
	_ = model
	return cols, nil
}

