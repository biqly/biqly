package query

import "testing"

func TestRepairMisnamedCalendarGrainDimensions_createdAtTs(t *testing.T) {
	dims := []string{
		"deleted_at", "created_at_ts", "created_at_ts_year", "created_at_ts_month", "created_at_ts_day",
		"row_count",
	}
	lq := LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "created_at_month"},
			{Type: SelectTypeMetric, Name: "row_count"},
		},
		Filters: []Filter{
			{Field: "created_at_month", Operator: OpEq, Value: 4},
		},
		GroupBy: []GroupBy{{Field: "created_at_month"}},
		OrderBy: []OrderBy{{Field: "created_at_month", Direction: OrderAsc}},
		Limit:   100,
	}
	RepairMisnamedCalendarGrainDimensions(&lq, dims)

	want := "created_at_ts_month"
	if lq.Select[0].Name != want {
		t.Errorf("select dimension: got %q want %q", lq.Select[0].Name, want)
	}
	if lq.Filters[0].Field != want {
		t.Errorf("filter field: got %q want %q", lq.Filters[0].Field, want)
	}
	if lq.GroupBy[0].Field != want {
		t.Errorf("group_by: got %q want %q", lq.GroupBy[0].Field, want)
	}
	if lq.OrderBy[0].Field != want {
		t.Errorf("order_by: got %q want %q", lq.OrderBy[0].Field, want)
	}
}

func TestRepairMisnamedCalendarGrainDimensions_noOpWhenUnknown(t *testing.T) {
	dims := []string{"created_at", "created_at_year"}
	lq := LogicalQuery{
		Filters: []Filter{{Field: "unknown_month", Operator: OpEq, Value: 1}},
		Limit:   10,
	}
	RepairMisnamedCalendarGrainDimensions(&lq, dims)
	if lq.Filters[0].Field != "unknown_month" {
		t.Errorf("expected field unchanged, got %q", lq.Filters[0].Field)
	}
}

func TestRepairMisnamedCalendarGrainDimensions_shorthandOrderGrain(t *testing.T) {
	dims := []string{"order_date", "order_date_year", "order_date_month", "count"}
	lq := LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "order_year"},
			{Type: SelectTypeDimension, Name: "order_month"},
			{Type: SelectTypeMetric, Name: "count"},
		},
		GroupBy: []GroupBy{{Field: "order_year"}, {Field: "order_month"}},
		OrderBy: []OrderBy{{Field: "order_year", Direction: OrderAsc}, {Field: "order_month", Direction: OrderAsc}},
		Limit:   100,
	}
	RepairMisnamedCalendarGrainDimensions(&lq, dims)

	if lq.Select[0].Name != "order_date_year" {
		t.Errorf("select year: got %q", lq.Select[0].Name)
	}
	if lq.Select[1].Name != "order_date_month" {
		t.Errorf("select month: got %q", lq.Select[1].Name)
	}
	if lq.GroupBy[0].Field != "order_date_year" || lq.GroupBy[1].Field != "order_date_month" {
		t.Errorf("group_by = %#v", lq.GroupBy)
	}
	if lq.OrderBy[0].Field != "order_date_year" || lq.OrderBy[1].Field != "order_date_month" {
		t.Errorf("order_by = %#v", lq.OrderBy)
	}
}

func TestRepairMisnamedCalendarGrainDimensions_noOpWhenAlreadyValid(t *testing.T) {
	dims := []string{"created_at_ts_month", "created_at_month"}
	lq := LogicalQuery{
		Filters: []Filter{{Field: "created_at_month", Operator: OpEq, Value: 4}},
		Limit:   10,
	}
	RepairMisnamedCalendarGrainDimensions(&lq, dims)
	if lq.Filters[0].Field != "created_at_month" {
		t.Errorf("expected canonical name kept when both exist, got %q", lq.Filters[0].Field)
	}
}
