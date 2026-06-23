package handlers

import (
	"testing"

	"github.com/biqly/biqly/internal/metadata"
	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

func TestApplyTranslationOverlay(t *testing.T) {
	dims := []pkgsemantic.Dimension{
		{ID: "d1", Name: "bookmark_count"},
		{ID: "d2", Name: "author_name", Label: new("Author Name"), Description: new("English desc")},
		{ID: "d3", Name: "created_at"},
	}
	tr := map[string]metadata.EntityTranslations{
		"d1": {metadata.TranslationFieldLabel: "Yer İmi Sayısı", metadata.TranslationFieldDescription: "Tweet'in kaç kez kaydedildiği."},
		// d2: label present, description blank → blank must NOT overwrite English.
		"d2": {metadata.TranslationFieldLabel: "Yazar Adı", metadata.TranslationFieldDescription: ""},
		// "d9" has no matching item and must be ignored.
		"d9": {metadata.TranslationFieldLabel: "Yok"},
	}

	applyTranslationOverlay(dims, tr,
		func(d *pkgsemantic.Dimension) string { return d.ID },
		func(d *pkgsemantic.Dimension, v string) { d.Label = new(v) },
		func(d *pkgsemantic.Dimension, v string) { d.Description = new(v) },
	)

	if got := deref(dims[0].Label); got != "Yer İmi Sayısı" {
		t.Errorf("d1 label = %q, want Turkish override", got)
	}
	if got := deref(dims[0].Description); got != "Tweet'in kaç kez kaydedildiği." {
		t.Errorf("d1 description = %q, want Turkish override", got)
	}
	if got := deref(dims[1].Label); got != "Yazar Adı" {
		t.Errorf("d2 label = %q, want Turkish override", got)
	}
	if got := deref(dims[1].Description); got != "English desc" {
		t.Errorf("d2 description = %q, want preserved English (blank override ignored)", got)
	}
	if dims[2].Label != nil {
		t.Errorf("d3 label = %v, want nil (no translation row)", *dims[2].Label)
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
