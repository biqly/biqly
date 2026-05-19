package ai

import (
	"context"
	"strconv"
	"testing"

	"github.com/biqly/biqly/internal/metadata"
)

func TestRankColumnsForTable_KeywordMatchWithoutEmbeddings(t *testing.T) {
	cols := []metadata.Column{
		{SchemaName: "sales", TableName: "orders", ColumnName: "order_id", DataType: "int", IsPrimaryKey: true},
		{SchemaName: "sales", TableName: "orders", ColumnName: "shipment_date", DataType: "date"},
	}
	for i := 0; i < 40; i++ {
		cols = append(cols, metadata.Column{
			SchemaName: "sales",
			TableName:  "orders",
			ColumnName: "noise_" + strconv.Itoa(i),
			DataType:   "text",
		})
	}
	tokens := tokenSet("show shipment date by order")
	ranked := rankColumnsForTable(cols, nil, nil, tokens, maxRankedColumnsPerTable)
	if !columnNamesContain(ranked, "shipment_date") {
		t.Fatalf("expected shipment_date in ranked columns, got %v", columnNames(ranked))
	}
	if columnNamesContain(ranked, "noise_39") {
		t.Fatalf("expected tail noise columns filtered; got %v", columnNames(ranked))
	}
}

func TestTableRouter_ColumnKeywordRankingWhenEmbeddingsIncomplete(t *testing.T) {
	reader := fakeMetadataReader{
		tables: []metadata.Table{
			{DatasourceID: "ds1", SchemaName: "sales", TableName: "salesorderheader", TableType: "BASE TABLE"},
		},
	}
	reader.columns = append(reader.columns,
		metadata.Column{DatasourceID: "ds1", SchemaName: "sales", TableName: "salesorderheader", ColumnName: "salesorderid", DataType: "int", IsPrimaryKey: true},
		metadata.Column{DatasourceID: "ds1", SchemaName: "sales", TableName: "salesorderheader", ColumnName: "shipment_date", DataType: "date"},
	)
	for i := 0; i < 40; i++ {
		reader.columns = append(reader.columns, metadata.Column{
			DatasourceID: "ds1",
			SchemaName:   "sales",
			TableName:    "salesorderheader",
			ColumnName:   "noise_" + strconv.Itoa(i),
			DataType:     "text",
		})
	}
	columnEmbeddings := []metadata.ColumnEmbedding{
		{SchemaName: "sales", TableName: "salesorderheader", ColumnName: "noise_1", Model: "fake", Embedding: []float32{1, 0}},
	}
	router := NewTableRouterWithEmbeddings(
		reader,
		&fakeEmbedder{model: "fake", fallback: []float32{1, 0}},
		&fakeEmbeddingReader{columnEmbeddings: columnEmbeddings},
		30.0,
	)

	model, _, err := router.Route(context.Background(), "ds1", "shipment date by sales order", []string{"sales.salesorderheader"}, true, true)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}
	if !hasDimension(model.Dimensions, "shipment_date", "salesorderheader.shipment_date") {
		t.Fatalf("keyword column ranking should keep shipment_date; dims=%v", dimNames(model.Dimensions))
	}
	if hasDimension(model.Dimensions, "noise_39", "salesorderheader.noise_39") {
		t.Fatalf("keyword column ranking should filter unrelated noise columns; dims=%v", dimNames(model.Dimensions))
	}
}

func columnNames(cols []metadata.Column) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.ColumnName
	}
	return out
}

func columnNamesContain(cols []metadata.Column, name string) bool {
	for _, c := range cols {
		if c.ColumnName == name {
			return true
		}
	}
	return false
}
