package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	routingGrainLabels   = []string{"year", "quarter", "month", "day", "hour", "none"}
	feedbackRatingLabels = []string{"positive", "negative"}
)

func registerTier3Metrics(f tier2MetricsFactory, m *Metrics) {
	m.routingGrainDetections = f.NewCounterVec(prometheus.CounterOpts{
		Name: "biqly_routing_grain_detections_total",
		Help: "Total time grain detections during query routing.",
	}, []string{"grain"})

	m.semanticgenModelsGenerated = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_semanticgen_models_generated_total",
		Help: "Total semantic models successfully generated.",
	})

	m.semanticgenDuration = f.NewHistogram(prometheus.HistogramOpts{
		Name:    "biqly_semanticgen_duration_seconds",
		Help:    "Semantic model generation latency in seconds.",
		Buckets: prometheus.DefBuckets,
	})

	m.semanticgenDimensionsGenerated = f.NewHistogram(prometheus.HistogramOpts{
		Name:    "biqly_semanticgen_dimensions_generated_histogram",
		Help:    "Distribution of dimensions count generated per model.",
		Buckets: []float64{10, 20, 50, 100, 200, 500},
	})

	m.semanticgenMetricsGenerated = f.NewHistogram(prometheus.HistogramOpts{
		Name:    "biqly_semanticgen_metrics_generated_histogram",
		Help:    "Distribution of metrics count generated per model.",
		Buckets: []float64{5, 10, 20, 50, 100},
	})

	m.feedbackSubmitted = f.NewCounterVec(prometheus.CounterOpts{
		Name: "biqly_feedback_submitted_total",
		Help: "Total AI feedback submissions by rating.",
	}, []string{"rating"})
}

// RecordRoutingGrainDetection records a detected time grain.
func (m *Metrics) RecordRoutingGrainDetection(grain string) {
	if m == nil {
		return
	}
	cleanGrain := BoundLabel(grain, routingGrainLabels, "none")
	m.routingGrainDetections.WithLabelValues(cleanGrain).Inc()
}

// RecordSemanticModelGenerated records semanticgen statistics.
func (m *Metrics) RecordSemanticModelGenerated(duration time.Duration, dimensions, metrics int) {
	if m == nil {
		return
	}
	m.semanticgenModelsGenerated.Inc()
	m.semanticgenDuration.Observe(duration.Seconds())
	m.semanticgenDimensionsGenerated.Observe(float64(dimensions))
	m.semanticgenMetricsGenerated.Observe(float64(metrics))
}

// RecordFeedbackSubmitted records user feedback rating.
func (m *Metrics) RecordFeedbackSubmitted(rating string) {
	if m == nil {
		return
	}
	cleanRating := BoundLabel(rating, feedbackRatingLabels, "negative")
	m.feedbackSubmitted.WithLabelValues(cleanRating).Inc()
}
