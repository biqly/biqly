package ai

type DescribeBatchTable struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
}

type DescribeBatchRequest struct {
	DatasourceID string               `json:"datasource_id"`
	Tables       []DescribeBatchTable `json:"tables"`
	Locale       string               `json:"locale,omitempty"`
	SampleSize   int                  `json:"sample_size,omitempty"`
	AutoApply    bool                 `json:"auto_apply,omitempty"`
	SkipExisting bool                 `json:"skip_existing,omitempty"`
}

type DescribeBatchEntryResult struct {
	Schema  string          `json:"schema"`
	Table   string          `json:"table"`
	Status  string          `json:"status"`
	Message string          `json:"message,omitempty"`
	Result  *DescribeResult `json:"result,omitempty"`
}

type DescribeBatchResult struct {
	Entries []DescribeBatchEntryResult `json:"entries"`
	OK      int                        `json:"ok"`
	Error   int                        `json:"error"`
	Skipped int                        `json:"skipped"`
}
