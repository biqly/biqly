package routing

import (
	"os"
	"slices"
	"testing"

	"github.com/biqly/biqly/internal/ai/lexicon"
)

func TestRoutingLexicon_ExpandTokenSynonyms(t *testing.T) {
	got := expandToken("musteri")
	want := false
	for _, tok := range got {
		if tok == "customer" {
			want = true
		}
	}
	if !want {
		t.Fatalf("expandToken(musteri) = %v, want customer via synonyms", got)
	}
}

func TestRoutingWeights_ApplyTableBoosts(t *testing.T) {
	w := activeRoutingWeights()
	lex := activeRoutingLexicon()
	tokens := map[string]struct{}{"urun": {}, "adet": {}}
	score := w.ApplyTableBoosts("salesorderdetail", tokens, 0, lex)
	if score < 10 {
		t.Fatalf("catalog+quantity orderdetail boost = %v, want >= 10", score)
	}
}

func TestRoutingLexicon_LoadOverride(t *testing.T) {
	path := t.TempDir() + "/lex.json"
	override := `{"token_synonyms":{"foo":["bar"]}}`
	if err := writeTestFile(path, override); err != nil {
		t.Fatal(err)
	}
	if err := InitRoutingLexicon(path); err != nil {
		t.Fatal(err)
	}
	if syns := activeRoutingLexicon().ExpandTokenSynonyms("foo"); len(syns) != 1 || syns[0] != "bar" {
		t.Fatalf("override synonyms = %v", syns)
	}
	if err := InitRoutingLexicon(""); err != nil {
		t.Fatal(err)
	}
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

type stubOverlayStore struct {
	tokens  map[string][]string
	metrics map[string][]string
}

func (stubOverlayStore) TemporalPhrases() []lexicon.TemporalPhrase { return nil }
func (s stubOverlayStore) Terms(domain, key string) []string       { return s.DomainTerms(domain)[key] }
func (s stubOverlayStore) DomainTerms(domain string) map[string][]string {
	switch domain {
	case lexicon.DomainTokenSynonym:
		return s.tokens
	case lexicon.DomainMetricSynonym:
		return s.metrics
	default:
		return nil
	}
}
func (stubOverlayStore) Invalidate() {}

// DB-managed token/metric synonyms overlay the file/embedded routing lexicon
// without a pod restart (DİL-2 acceptance): per-key replace, base keys intact.
func TestActiveRoutingLexiconAppliesDBOverlay(t *testing.T) {
	prev := lexicon.SetActive(stubOverlayStore{
		tokens:  map[string][]string{"kunde": {"customer", "client"}},
		metrics: map[string][]string{"min_numeric": {"kleinste"}},
	})
	t.Cleanup(func() {
		lexicon.SetActive(prev)
		InvalidateRoutingLexicon()
	})
	InvalidateRoutingLexicon()

	lex, err := ActiveRoutingLexicon()
	if err != nil {
		t.Fatal(err)
	}
	if got := lex.ExpandTokenSynonyms("kunde"); !slices.Equal(got, []string{"customer", "client"}) {
		t.Fatalf("overlay token synonyms = %v", got)
	}
	if got := lex.MetricSynonymList("min_numeric"); !slices.Equal(got, []string{"kleinste"}) {
		t.Fatalf("overlay metric synonyms = %v", got)
	}
	if got := lex.ExpandTokenSynonyms("musteri"); !slices.Contains(got, "customer") {
		t.Fatalf("base token synonyms must survive the overlay, got %v", got)
	}
	if len(lex.RowCountSynonyms) == 0 {
		t.Fatal("non-overlaid lexicon fields must keep their base values")
	}

	// Dropping the overlay (after invalidation) restores the base entry.
	lexicon.SetActive(prev)
	InvalidateRoutingLexicon()
	lex, err = ActiveRoutingLexicon()
	if err != nil {
		t.Fatal(err)
	}
	if got := lex.ExpandTokenSynonyms("kunde"); got != nil {
		t.Fatalf("overlay leaked after invalidation: %v", got)
	}
}
