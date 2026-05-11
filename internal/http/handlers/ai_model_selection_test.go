package handlers

import (
	"testing"

	"github.com/biqly/biqly/internal/semantic"
)

func TestChooseSemanticModelForQuestion_PrefersOnlyActivePersistentModel(t *testing.T) {
	models := []semantic.SemanticModel{
		{Name: "orders", BaseTable: "orders", IsActive: true},
	}

	got, ok := chooseSemanticModelForQuestion(models, "completely unrelated")
	if !ok {
		t.Fatal("chooseSemanticModelForQuestion() ok = false, want true")
	}
	if got.Name != "orders" {
		t.Errorf("chooseSemanticModelForQuestion() = %q, want orders", got.Name)
	}
}

func TestChooseSemanticModelForQuestion_ScoresMultiplePersistentModels(t *testing.T) {
	customerDesc := "Customer orders and revenue by region"
	inventoryDesc := "Warehouse inventory and stock"
	models := []semantic.SemanticModel{
		{Name: "inventory", Description: &inventoryDesc, BaseTable: "stock", IsActive: true},
		{Name: "customer_orders", Description: &customerDesc, BaseTable: "orders", Synonyms: []string{"sales", "revenue"}, IsActive: true},
	}

	got, ok := chooseSemanticModelForQuestion(models, "show customer revenue")
	if !ok {
		t.Fatal("chooseSemanticModelForQuestion() ok = false, want true")
	}
	if got.Name != "customer_orders" {
		t.Errorf("chooseSemanticModelForQuestion() = %q, want customer_orders", got.Name)
	}
}

func TestChooseSemanticModelForQuestion_IgnoresInactiveAndAutoModels(t *testing.T) {
	models := []semantic.SemanticModel{
		{Name: "auto:public.orders", BaseTable: "orders", IsActive: true},
		{Name: "inactive_orders", BaseTable: "orders", IsActive: false},
		{Name: "draft_orders", BaseTable: "orders", IsActive: true, Status: semantic.ModelStatusDraft},
	}

	_, ok := chooseSemanticModelForQuestion(models, "show orders")
	if ok {
		t.Fatal("chooseSemanticModelForQuestion() ok = true, want false")
	}
}
