package metadata

import (
	"database/sql"
	"testing"

	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConnectionParams(t *testing.T) {
	tests := []struct {
		name string
		cp   []byte
		want []byte
	}{
		{name: "nil", cp: nil, want: []byte("{}")},
		{name: "empty", cp: []byte{}, want: []byte("{}")},
		{name: "non-empty", cp: []byte(`{"key":"val"}`), want: []byte(`{"key":"val"}`)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := defaultConnectionParams(tt.cp)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDerefStringOrEmpty(t *testing.T) {
	s := "hello"
	tests := []struct {
		name string
		p    *string
		want any
	}{
		{name: "nil", p: nil, want: ""},
		{name: "valid", p: &s, want: "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := derefStringOrEmpty(tt.p)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNullableJSON(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		got, err := nullableJSON(nil)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})
	t.Run("valid", func(t *testing.T) {
		got, err := nullableJSON(map[string]string{"key": "val"})
		assert.NoError(t, err)
		require.NotNil(t, got)
		assert.JSONEq(t, `{"key":"val"}`, *got)
	})
	t.Run("marshal error", func(t *testing.T) {
		// A channel is not JSON-serializable
		_, err := nullableJSON(make(chan int))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "marshal json")
	})
}

func TestNullableLocale(t *testing.T) {
	tests := []struct {
		name   string
		locale string
		want   any
	}{
		{name: "empty", locale: "", want: nil},
		{name: "whitespace", locale: "  ", want: nil},
		{name: "non-empty", locale: "en", want: "en"},
		{name: "trimmed", locale: "  tr  ", want: "tr"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nullableLocale(tt.locale)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNullStringPtr(t *testing.T) {
	t.Run("invalid", func(t *testing.T) {
		got := nullStringPtr(sql.NullString{Valid: false})
		assert.Nil(t, got)
	})
	t.Run("valid", func(t *testing.T) {
		got := nullStringPtr(sql.NullString{Valid: true, String: "hello"})
		require.NotNil(t, got)
		assert.Equal(t, "hello", *got)
	})
}

func TestMarshalGlossaryAIContext(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		got, err := marshalGlossaryAIContext(nil)
		assert.NoError(t, err)
		assert.Nil(t, got)
	})
	t.Run("zero value", func(t *testing.T) {
		got, err := marshalGlossaryAIContext(&pkgmetadata.GlossaryAIContext{})
		assert.NoError(t, err)
		assert.Nil(t, got)
	})
	t.Run("valid", func(t *testing.T) {
		ctx := &pkgmetadata.GlossaryAIContext{
			Synonyms: []string{"syn1"},
		}
		got, err := marshalGlossaryAIContext(ctx)
		assert.NoError(t, err)
		require.NotNil(t, got)
		raw, ok := got.([]byte)
		require.True(t, ok)
		var decoded pkgmetadata.GlossaryAIContext
		err = sonic.Unmarshal(raw, &decoded)
		assert.NoError(t, err)
		assert.Equal(t, []string{"syn1"}, decoded.Synonyms)
	})
}

func TestEncodeEmbedding(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		got, err := encodeEmbedding(nil)
		assert.NoError(t, err)
		assert.Equal(t, "null", got)
	})
	t.Run("valid", func(t *testing.T) {
		got, err := encodeEmbedding([]float32{0.1, 0.2, 0.3})
		assert.NoError(t, err)
		assert.JSONEq(t, `[0.1,0.2,0.3]`, got)
	})
}

func TestDecodeEmbedding(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		got, err := decodeEmbedding(nil)
		assert.NoError(t, err)
		assert.Nil(t, got)
		got, err = decodeEmbedding([]byte{})
		assert.NoError(t, err)
		assert.Nil(t, got)
	})
	t.Run("valid", func(t *testing.T) {
		got, err := decodeEmbedding([]byte(`[0.1,0.2,0.3]`))
		assert.NoError(t, err)
		assert.Equal(t, []float32{0.1, 0.2, 0.3}, got)
	})
	t.Run("invalid json", func(t *testing.T) {
		_, err := decodeEmbedding([]byte(`not-json`))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "decode embedding")
	})
}

func TestUniqueLangs(t *testing.T) {
	tests := []struct {
		name  string
		langs []string
		want  []string
	}{
		{name: "empty", langs: nil, want: []string{}},
		{name: "skips empty", langs: []string{"en", "", "fr", ""}, want: []string{"en", "fr"}},
		{name: "deduplicates", langs: []string{"en", "fr", "en", "tr", "fr"}, want: []string{"en", "fr", "tr"}},
		{name: "all empty", langs: []string{"", "", ""}, want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uniqueLangs(tt.langs...)
			assert.Equal(t, tt.want, got)
		})
	}
}
