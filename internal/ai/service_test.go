package ai

import (
	"testing"

	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

func TestParseAndValidateNormalizesLogicalQueryContext(t *testing.T) {
	service := &Service{validator: query.NewValidator(1000)}
	model := &semantic.SemanticModel{
		ID:           "model-uuid",
		DatasourceID: "datasource-uuid",
		Name:         "public.orders",
		Metrics: []semantic.Metric{
			{Name: "first_order_created_at", Expression: "orders.created_at", Aggregation: "min"},
		},
	}
	raw := `{"datasource_id":"","model_id":"","select":[{"type":"metric","name":"first_order_created_at"}],"limit":100}`

	got, _, err := service.parseAndValidate(raw, model)
	if err != nil {
		t.Fatalf("parseAndValidate(%s) error = %v, want nil", raw, err)
	}
	if got.DatasourceID != model.DatasourceID {
		t.Errorf("parseAndValidate(%s).DatasourceID = %q, want %q", raw, got.DatasourceID, model.DatasourceID)
	}
	if got.ModelID != model.Name {
		t.Errorf("parseAndValidate(%s).ModelID = %q, want %q", raw, got.ModelID, model.Name)
	}
}
