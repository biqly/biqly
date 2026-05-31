package errmsg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorMessageHelpers(t *testing.T) {
	t.Run("UnknownDimensionMsg", func(t *testing.T) {
		got := UnknownDimensionMsg("foo")
		assert.Equal(t, "unknown dimension: foo", got)
	})

	t.Run("UnknownMetricMsg", func(t *testing.T) {
		got := UnknownMetricMsg("bar")
		assert.Equal(t, "unknown metric: bar", got)
	})

	t.Run("UnknownFieldMsg", func(t *testing.T) {
		got := UnknownFieldMsg("baz")
		assert.Equal(t, "unknown field: baz", got)
	})
}

func TestErrorFactories(t *testing.T) {
	t.Run("ErrUnknownDimension", func(t *testing.T) {
		err := ErrUnknownDimension("foo")
		assert.Error(t, err)
		assert.Equal(t, "unknown dimension: foo", err.Error())
	})

	t.Run("ErrUnknownMetric", func(t *testing.T) {
		err := ErrUnknownMetric("bar")
		assert.Error(t, err)
		assert.Equal(t, "unknown metric: bar", err.Error())
	})

	t.Run("ErrUnknownField", func(t *testing.T) {
		err := ErrUnknownField("baz")
		assert.Error(t, err)
		assert.Equal(t, "unknown field: baz", err.Error())
	})

	t.Run("RowFilterUnknownField", func(t *testing.T) {
		err := RowFilterUnknownField("qux")
		assert.Error(t, err)
		assert.Equal(t, "row filter references unknown field: qux", err.Error())
	})
}
