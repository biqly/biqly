package handlers

import (
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/i18n"
	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
)

func relDetail(id, desc string) pkgmetadata.RelationDetail {
	return pkgmetadata.RelationDetail{
		Relation:    pkgmetadata.Relation{ID: id},
		Description: desc,
	}
}

func TestSelectRelations(t *testing.T) {
	rels := []pkgmetadata.RelationDetail{
		relDetail("a", ""),
		relDetail("b", "already described"),
		relDetail("c", ""),
	}

	ids := func(rs []pkgmetadata.RelationDetail) []string {
		out := make([]string, len(rs))
		for i, r := range rs {
			out[i] = r.ID
		}
		return out
	}

	t.Run("empty request selects all", func(t *testing.T) {
		got := ids(selectRelations(rels, describeRelationsRequest{}))
		if len(got) != 3 {
			t.Fatalf("got %v, want all three", got)
		}
	})

	t.Run("skip_existing drops described relations", func(t *testing.T) {
		got := ids(selectRelations(rels, describeRelationsRequest{SkipExisting: true}))
		if len(got) != 2 || got[0] != "a" || got[1] != "c" {
			t.Fatalf("got %v, want [a c]", got)
		}
	})

	t.Run("relation_ids restricts the set", func(t *testing.T) {
		got := ids(selectRelations(rels, describeRelationsRequest{RelationIDs: []string{"b"}}))
		if len(got) != 1 || got[0] != "b" {
			t.Fatalf("got %v, want [b]", got)
		}
	})

	t.Run("relation_ids combines with skip_existing", func(t *testing.T) {
		got := ids(selectRelations(rels, describeRelationsRequest{
			RelationIDs: []string{"a", "b"}, SkipExisting: true,
		}))
		if len(got) != 1 || got[0] != "a" {
			t.Fatalf("got %v, want [a]", got)
		}
	})
}

func TestBuildDescribeRelationsPromptLocale(t *testing.T) {
	rels := []pkgmetadata.RelationDetail{relDetail("r1", "")}

	en := buildDescribeRelationsPrompt(rels, i18n.LocaleEN)
	if strings.Contains(en, "localized") {
		t.Fatalf("english prompt must not request a localized field:\n%s", en)
	}

	tr := buildDescribeRelationsPrompt(rels, i18n.LocaleTR)
	if !strings.Contains(tr, "localized") || !strings.Contains(tr, "tr") {
		t.Fatalf("turkish prompt must request the localized field:\n%s", tr)
	}
}

func TestParseDescribeRelationsResponse(t *testing.T) {
	raw := "```json\n{\"relations\": [{\"id\": \"r1\", \"description\": \"Links orders to customers.\", \"localized\": \"Siparişleri müşterilere bağlar.\"}]}\n```"
	got, err := parseDescribeRelationsResponse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 || got[0].ID != "r1" || got[0].Localized == "" {
		t.Fatalf("unexpected parse result: %+v", got)
	}
}
