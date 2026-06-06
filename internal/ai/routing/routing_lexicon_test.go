package routing

import (
	"os"
	"testing"
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
