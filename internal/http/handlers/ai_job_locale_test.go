package handlers

import (
	"testing"

	"github.com/biqly/biqly/internal/ai"
	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/require"
)

func TestDescribeBatchSingleRequestPreservesLocale(t *testing.T) {
	req := ai.DescribeBatchRequest{
		DatasourceID: "ds-1",
		Locale:       "tr",
		SampleSize:   25,
		AutoApply:    true,
	}

	single := describeBatchSingleRequest(req, "public", "orders")

	require.Equal(t, ai.DescribeRequest{
		DatasourceID: "ds-1",
		Schema:       "public",
		Table:        "orders",
		Locale:       "tr",
		SampleSize:   25,
		AutoApply:    true,
	}, single)
}

func TestAIJobLocaleFromRequestPrefersDescribeRequestLocale(t *testing.T) {
	raw, err := sonic.Marshal(ai.DescribeBatchRequest{Locale: "tr"})
	require.NoError(t, err)

	require.Equal(t, "tr", aiJobLocaleFromRequest("describe_batch", raw, "en"))
}
