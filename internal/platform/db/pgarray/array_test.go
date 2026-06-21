package pgarray

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStrings(t *testing.T) {
	t.Run("nil slice", func(t *testing.T) {
		res := Strings(nil)
		assert.NotNil(t, res)
	})

	t.Run("empty slice", func(t *testing.T) {
		res := Strings([]string{})
		assert.NotNil(t, res)
	})

	t.Run("non-empty slice", func(t *testing.T) {
		res := Strings([]string{"a", "b", "c"})
		assert.NotNil(t, res)
	})

	t.Run("single element", func(t *testing.T) {
		res := Strings([]string{"hello"})
		assert.NotNil(t, res)
	})
}

func TestScan(t *testing.T) {
	t.Run("nil destination", func(t *testing.T) {
		res := Scan(nil)
		assert.NotNil(t, res)
	})

	t.Run("string slice pointer", func(t *testing.T) {
		var dst []string
		res := Scan(&dst)
		assert.NotNil(t, res)
	})

	t.Run("int slice pointer", func(t *testing.T) {
		var dst []int
		res := Scan(&dst)
		assert.NotNil(t, res)
	})
}
