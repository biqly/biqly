package handlers

// AIMetricsRecorder records in-process AI request counters for Prometheus /metrics.
type AIMetricsRecorder interface {
	RecordAIRequest(latencyMs int64, success bool, retryCount int, clarification bool)
}
