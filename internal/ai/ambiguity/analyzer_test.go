package ambiguity

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/semantic"
)

func TestAmbiguityResultJSON(t *testing.T) {
	result := Result{
		IsAmbiguous: true,
		Ambiguities: []Item{
			{
				Term: "aktif müşteri",
				Type: "semantic",
				Interpretations: []Interpretation{
					{
						Label:       "Son 30 günde sipariş veren müşteri",
						Description: "Müşterinin yakın zamanda sipariş vermiş olması",
						SemanticMapping: SemanticMapping{
							Type: "dimension",
							Name: "last_order_date",
						},
						Confidence: 0.92,
					},
				},
			},
		},
		ResolvedQuestion: "Son 30 günde sipariş veren müşterileri göster",
	}

	got, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal(%#v) error = %v", result, err)
	}
	want := `{"is_ambiguous":true,"ambiguities":[{"term":"aktif müşteri","type":"semantic","interpretations":[{"label":"Son 30 günde sipariş veren müşteri","description":"Müşterinin yakın zamanda sipariş vermiş olması","semantic_mapping":{"type":"dimension","name":"last_order_date"},"confidence":0.92}]}],"resolved_question":"Son 30 günde sipariş veren müşterileri göster"}`
	if string(got) != want {
		t.Errorf("json.Marshal(%#v) = %s, want %s", result, got, want)
	}
}

func TestAnalyze_MergesRuleBasedAmbiguities(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "customer_segment", Synonyms: []string{"ciro"}},
		},
		Metrics: []semantic.Metric{
			{Name: "revenue", Synonyms: []string{"ciro"}},
		},
	}
	glossary := []prompt.GlossaryEntry{
		{Term: "ciro", Definition: "Brüt gelir", MapsToType: "metric", MapsToName: "revenue"},
		{Term: "ciro", Definition: "Net gelir", MapsToType: "metric", MapsToName: "net_revenue"},
	}

	got := Analyze(context.Background(), "Ciro göster", model, glossary, 0)
	want := Result{
		IsAmbiguous: true,
		Ambiguities: []Item{
			{
				Term: "ciro",
				Type: "semantic",
				Interpretations: []Interpretation{
					{
						Label:       "Brüt gelir",
						Description: "Brüt gelir",
						SemanticMapping: SemanticMapping{
							Type: "metric",
							Name: "revenue",
						},
						Confidence: 1,
					},
					{
						Label:       "Net gelir",
						Description: "Net gelir",
						SemanticMapping: SemanticMapping{
							Type: "metric",
							Name: "net_revenue",
						},
						Confidence: 1,
					},
					{
						Label:       "customer_segment",
						Description: "dimension customer_segment",
						SemanticMapping: SemanticMapping{
							Type: "dimension",
							Name: "customer_segment",
						},
						Confidence: 1,
					},
				},
			},
		},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("Analyze() = %#v, want %#v", got, want)
	}
}

func TestFilterAmbiguities_RequiresTwoInterpretationsAboveThreshold(t *testing.T) {
	ambiguities := []Item{
		{
			Term: "ciro",
			Type: "semantic",
			Interpretations: []Interpretation{
				{Label: "low confidence", Confidence: 0.69},
				{Label: "high confidence", Confidence: 0.90},
			},
		},
	}

	got := filterAmbiguities(ambiguities, defaultConfidenceThreshold)
	if len(got) != 0 {
		t.Errorf("filterAmbiguities() = %#v, want no ambiguities", got)
	}
}

func TestAnalyze_QuestionExamples(t *testing.T) {
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "order_date", Type: string(semantic.DimensionTypeDate)},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count"},
			{Name: "revenue"},
		},
	}
	glossary := []prompt.GlossaryEntry{
		{Term: "aktif müşteriler", Definition: "Yakın dönemde sipariş veren müşteriler", MapsToType: "dimension", MapsToName: "recent_customer"},
		{Term: "aktif müşteriler", Definition: "Hesabı kapatılmamış müşteriler", MapsToType: "dimension", MapsToName: "enabled_customer"},
	}

	tests := []struct {
		name      string
		question  string
		ambiguous bool
	}{
		{name: "glossary ambiguity", question: "aktif müşteriler", ambiguous: true},
		{name: "scope ambiguity", question: "büyük siparişler", ambiguous: true},
		{name: "temporal ambiguity", question: "son zamanlarda sipariş veren müşteriler", ambiguous: true},
		{name: "specific time range", question: "son 30 günde sipariş veren müşteriler", ambiguous: false},
		{name: "empty question", question: "", ambiguous: false},
		{name: "very short question", question: "x", ambiguous: false},
		{name: "exact metric match", question: "yüksek revenue", ambiguous: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Analyze(context.Background(), tt.question, model, glossary, 0)
			if got.IsAmbiguous != tt.ambiguous {
				t.Fatalf("Analyze(%q).IsAmbiguous = %t, want %t; result = %#v", tt.question, got.IsAmbiguous, tt.ambiguous, got)
			}
		})
	}
}

func TestAnalyzeWithDetectorsRunsDetectorsInParallel(t *testing.T) {
	started := make(chan struct{}, 2)
	release := make(chan struct{})
	detector := func() []Item {
		started <- struct{}{}
		<-release
		return []Item{}
	}

	done := make(chan struct{})
	go func() {
		analyzeWithDetectors(context.Background(), []func() []Item{detector, detector}, 0, time.Second)
		close(done)
	}()

	for range 2 {
		select {
		case <-started:
		case <-time.After(100 * time.Millisecond):
			t.Fatal("detectors did not start in parallel")
		}
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("analyzeWithDetectors() did not finish")
	}
}

func TestAnalyzeWithDetectorsStopsWaitingAtTimeout(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })

	start := time.Now()
	got := analyzeWithDetectors(
		context.Background(),
		[]func() []Item{
			func() []Item {
				<-release
				return []Item{}
			},
		},
		0,
		10*time.Millisecond,
	)
	if got.IsAmbiguous {
		t.Fatalf("analyzeWithDetectors() = %#v, want no ambiguity", got)
	}
	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("analyzeWithDetectors() elapsed = %s, want <= 200ms", elapsed)
	}
}
