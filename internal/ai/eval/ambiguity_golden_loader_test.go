package eval

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDefaultAmbiguityGoldenCases(t *testing.T) {
	cases, err := LoadDefaultAmbiguityGoldenCases()
	require.NoError(t, err)
	require.NotEmpty(t, cases)

	ids := make(map[string]struct{}, len(cases))
	for _, c := range cases {
		ids[c.ID] = struct{}{}
		assert.NotEmpty(t, c.Question)
		assert.NotNil(t, c.Model)
	}
	assert.Contains(t, ids, "glossary-active-customers")
	assert.Contains(t, ids, "choice-net-revenue")
}

func TestLoadAmbiguityGoldenCases_RejectsUnknownModelRef(t *testing.T) {
	_, err := LoadAmbiguityGoldenCases([]byte(`[{"id":"x","question":"q","model_ref":"missing","expected_type":"pass"}]`))
	require.Error(t, err)
}
