package prompt

// TableSample carries a small set of rows from a single table to be embedded
// in the prompt as concrete examples of the data the LLM is querying.
type TableSample struct {
	Schema string
	Table  string
	Rows   []map[string]any
}
