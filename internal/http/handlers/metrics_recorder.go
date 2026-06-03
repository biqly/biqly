package handlers

// AIMetricsRecorder records in-process AI request counters for Prometheus /metrics.
type AIMetricsRecorder interface {
	RecordAIRequest(latencyMs int64, success bool, retryCount int, clarification bool)
	RecordLLMRequest(latencyMs int64, tokensUsed int, promptBuildMs int64)
	RecordAmbiguityAnalysis(latencyMs int64, source string, detected bool)
	RecordAmbiguityClarified()
	RecordAIRepair(success bool, attempts int, errorCodes []string)
}

// CatalogMetricsRecorder records Catalog Service process metrics.
type CatalogMetricsRecorder interface {
	RecordModelPublish(durationMs int64, success bool)
}

// QueryMetricsRecorder records Query Engine process metrics.
type QueryMetricsRecorder interface {
	RecordQueryCompile(durationMs int64, success bool)
	RecordQueryExecution(durationMs int64, success bool, rows int)
}
