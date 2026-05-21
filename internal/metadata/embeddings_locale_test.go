package metadata

import (
	"encoding/json"
	"testing"
)

func TestMergeEmbeddingPayloadMultiLocale(t *testing.T) {
	legacy, err := encodeEmbedding([]float32{1, 0})
	if err != nil {
		t.Fatal(err)
	}
	legacyModel := "embeddinggemma:300m"
	payload, display, err := mergeEmbeddingPayload([]byte(legacy), &legacyModel, "embeddinggemma:300m@tr", []float32{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	if display != "embeddinggemma:300m" {
		t.Fatalf("display model = %q", display)
	}
	var store multiLocaleEmbeddingPayload
	if err := json.Unmarshal([]byte(payload), &store); err != nil {
		t.Fatal(err)
	}
	if len(store.Locales) != 2 {
		t.Fatalf("locales = %d, want 2", len(store.Locales))
	}
	entries, err := expandTableEmbeddings("public", "tweets", &legacyModel, []byte(payload))
	if err != nil || len(entries) != 2 {
		t.Fatalf("expand: %v len=%d", err, len(entries))
	}
}
