package handlers

import (
	"testing"

	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

func TestSelectJoins(t *testing.T) {
	joins := []pkgsemantic.Join{
		{ID: "a", IsActive: true, Description: ""},
		{ID: "b", IsActive: true, Description: "already described"},
		{ID: "c", IsActive: false, Description: ""},
	}

	ids := func(js []pkgsemantic.Join) []string {
		out := make([]string, len(js))
		for i, j := range js {
			out[i] = j.ID
		}
		return out
	}

	t.Run("empty request describes all active joins", func(t *testing.T) {
		got := ids(selectJoins(joins, describeJoinsRequest{}))
		want := []string{"a", "b"}
		if len(got) != len(want) || got[0] != "a" || got[1] != "b" {
			t.Fatalf("got %v, want %v", got, want)
		}
	})

	t.Run("only_missing skips joins that already have a description", func(t *testing.T) {
		got := ids(selectJoins(joins, describeJoinsRequest{OnlyMissing: true}))
		if len(got) != 1 || got[0] != "a" {
			t.Fatalf("got %v, want [a]", got)
		}
	})

	t.Run("join_ids restricts to the requested active joins", func(t *testing.T) {
		got := ids(selectJoins(joins, describeJoinsRequest{JoinIDs: []string{"b", "c"}}))
		// c is inactive, so only b survives.
		if len(got) != 1 || got[0] != "b" {
			t.Fatalf("got %v, want [b]", got)
		}
	})

	t.Run("join_ids combines with only_missing", func(t *testing.T) {
		got := ids(selectJoins(joins, describeJoinsRequest{JoinIDs: []string{"a", "b"}, OnlyMissing: true}))
		if len(got) != 1 || got[0] != "a" {
			t.Fatalf("got %v, want [a]", got)
		}
	})
}
