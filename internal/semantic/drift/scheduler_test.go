package drift

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDriftsMatch(t *testing.T) {
	added := DriftItem{Type: DriftTypeColumnAdded, Field: "", ColumnRef: "public.t.new_col"}
	typeChanged := DriftItem{Type: DriftTypeTypeChanged, Field: "dim", ColumnRef: "public.t.col"}

	t.Run("same items match regardless of order and value fields", func(t *testing.T) {
		a := []DriftItem{added, typeChanged}
		b := []DriftItem{
			{Type: DriftTypeTypeChanged, Field: "dim", ColumnRef: "public.t.col", OldValue: "text", NewValue: "citext"},
			{Type: DriftTypeColumnAdded, Field: "", ColumnRef: "public.t.new_col", Description: "other wording"},
		}
		assert.True(t, DriftsMatch(a, b))
	})

	t.Run("different length does not match", func(t *testing.T) {
		assert.False(t, DriftsMatch([]DriftItem{added}, []DriftItem{added, typeChanged}))
	})

	t.Run("different column ref does not match", func(t *testing.T) {
		other := added
		other.ColumnRef = "public.t.other_col"
		assert.False(t, DriftsMatch([]DriftItem{added}, []DriftItem{other}))
	})

	t.Run("empty sets match", func(t *testing.T) {
		assert.True(t, DriftsMatch(nil, []DriftItem{}))
	})
}
