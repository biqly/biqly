package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	routingConfidenceBuckets = []float64{0.1, 0.3, 0.5, 0.7, 0.8, 0.9, 0.95, 0.99}

	rankingMethods  = []string{"keyword", "hybrid", "manual", "semantic"}
	routingOutcomes = []string{"success", "clarification", "error"}

	llmProviders  = []string{"openai", "anthropic", "other"}
	llmErrorTypes = []string{"rate_limit", "network", "auth", "parse", "other"}

	embeddingOperations = []string{"route_recall", "memory_store", "metadata_embed", "other"}

	httpMethods     = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "HEAD", "other"}
	httpRouteGroups = []string{
		"/api/ai/query",
		"/api/ai/preview",
		"/api/ai/other",
		"/api/catalog",
		"/api/query",
		"/api/admin",
		"/api/auth",
		"/api/other",
		"/internal",
		"/health",
		"/metrics",
		"other",
	}
)

type tier1MetricsFactory interface {
	NewCounter(opts prometheus.CounterOpts) prometheus.Counter
	NewCounterVec(opts prometheus.CounterOpts, labelNames []string) *prometheus.CounterVec
	NewHistogram(opts prometheus.HistogramOpts) prometheus.Histogram
	NewHistogramVec(opts prometheus.HistogramOpts, labelNames []string) *prometheus.HistogramVec
}

func registerTier1Metrics(f tier1MetricsFactory, m *Metrics) {
	m.httpRequestDuration = f.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "biqly_http_request_duration_seconds",
		Help:    "HTTP request latency in seconds by method and bounded route group.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "route_group"})
	m.httpRequestsTotal = f.NewCounterVec(prometheus.CounterOpts{
		Name: "biqly_http_requests_total",
		Help: "Total HTTP requests by method and response status class.",
	}, []string{"method", "status_class"})

	m.llmErrorsTotal = f.NewCounterVec(prometheus.CounterOpts{
		Name: "biqly_llm_errors_total",
		Help: "Total LLM provider API errors by provider and error type.",
	}, []string{"provider", "error_type"})
	m.llmRetriesTotal = f.NewCounterVec(prometheus.CounterOpts{
		Name: "biqly_llm_retries_total",
		Help: "Total LLM provider HTTP retry attempts.",
	}, []string{"provider"})
	m.llmTokensPromptTotal = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_llm_tokens_prompt_total",
		Help: "Total LLM prompt tokens reported by providers.",
	})
	m.llmTokensCompletionTotal = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_llm_tokens_completion_total",
		Help: "Total LLM completion tokens reported by providers.",
	})

	m.routingConfidenceHistogram = f.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "biqly_routing_confidence_histogram",
		Help:    "Table routing confidence scores (0.0-1.0) by ranking method.",
		Buckets: routingConfidenceBuckets,
	}, []string{"ranking_method"})
	m.routingDecisionsTotal = f.NewCounterVec(prometheus.CounterOpts{
		Name: "biqly_routing_decisions_total",
		Help: "Total table routing decisions by method and outcome.",
	}, []string{"method", "outcome"})

	m.embeddingAPIDuration = f.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "biqly_embedding_api_duration_seconds",
		Help:    "Embedding API round-trip latency in seconds by operation.",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation"})
	m.embeddingAPIErrorsTotal = f.NewCounterVec(prometheus.CounterOpts{
		Name: "biqly_embedding_api_errors_total",
		Help: "Total embedding API errors by operation and error type.",
	}, []string{"operation", "error_type"})
}

// RecordHTTPRequest records request latency and status for one HTTP call.
func (m *Metrics) RecordHTTPRequest(method, routePattern string, status int, durationMs int64) {
	if m == nil || durationMs < 0 {
		return
	}
	cleanMethod := BoundLabel(method, httpMethods, "other")
	group := HTTPRouteGroup(routePattern)
	m.httpRequestDuration.WithLabelValues(cleanMethod, group).Observe(msToSeconds(durationMs))
	m.httpRequestsTotal.WithLabelValues(cleanMethod, HTTPStatusClass(status)).Inc()
}

// RecordLLMProviderError records a failed LLM provider API call.
func (m *Metrics) RecordLLMProviderError(provider, errorType string) {
	if m == nil {
		return
	}
	m.llmErrorsTotal.WithLabelValues(
		BoundLabel(provider, llmProviders, "other"),
		BoundLabel(errorType, llmErrorTypes, "other"),
	).Inc()
}

// RecordLLMProviderRetry records one LLM HTTP retry attempt.
func (m *Metrics) RecordLLMProviderRetry(provider string) {
	if m == nil {
		return
	}
	m.llmRetriesTotal.WithLabelValues(BoundLabel(provider, llmProviders, "other")).Inc()
}

// RecordLLMProviderTokens records prompt and completion tokens from a provider response.
func (m *Metrics) RecordLLMProviderTokens(promptTokens, completionTokens int) {
	if m == nil {
		return
	}
	if promptTokens > 0 {
		m.llmTokensPromptTotal.Add(float64(promptTokens))
	}
	if completionTokens > 0 {
		m.llmTokensCompletionTotal.Add(float64(completionTokens))
	}
}

// RecordRoutingResult records routing confidence and decision outcome.
func (m *Metrics) RecordRoutingResult(rankingMethod string, confidence float64, needsClarification bool, failed bool) {
	if m == nil {
		return
	}
	method := BoundLabel(rankingMethod, rankingMethods, "keyword")
	if method == "" {
		method = "keyword"
	}
	if confidence > 0 || rankingMethod != "" {
		m.routingConfidenceHistogram.WithLabelValues(method).Observe(confidence)
	}
	outcome := "success"
	switch {
	case failed:
		outcome = "error"
	case needsClarification:
		outcome = "clarification"
	}
	m.routingDecisionsTotal.WithLabelValues(method, BoundLabel(outcome, routingOutcomes, "error")).Inc()
}

// RecordEmbeddingAPI records embedding API latency and optional error.
func (m *Metrics) RecordEmbeddingAPI(operation string, durationMs int64, err error, httpStatus int) {
	if m == nil {
		return
	}
	op := BoundLabel(operation, embeddingOperations, "other")
	if durationMs >= 0 {
		m.embeddingAPIDuration.WithLabelValues(op).Observe(msToSeconds(durationMs))
	}
	if err != nil {
		m.embeddingAPIErrorsTotal.WithLabelValues(op, ClassifyProviderError(err, httpStatus)).Inc()
	}
}

// HTTPStatusClass maps an HTTP status code to a bounded class label.
func HTTPStatusClass(status int) string {
	class := "other"
	switch {
	case status >= 200 && status < 300:
		class = "2xx"
	case status >= 300 && status < 400:
		class = "3xx"
	case status >= 400 && status < 500:
		class = "4xx"
	case status >= 500:
		class = "5xx"
	}
	return BoundLabel(class, []string{"2xx", "3xx", "4xx", "5xx", "other"}, "other")
}
