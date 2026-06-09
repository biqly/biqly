package observability

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

const (
	// DefaultMaxLabelValues is the fallback ceiling for undocumented *_Vec labels.
	DefaultMaxLabelValues = 32
	// MaxMetricSeriesWarn is the per-metric series count surfaced on the Grafana dashboard.
	MaxMetricSeriesWarn = 100
)

// ForbiddenMetricLabels must never appear on Prometheus metrics — they are
// unbounded per user/request and will blow Prometheus memory if used as labels.
var ForbiddenMetricLabels = []string{
	"user_id",
	"query_id",
	"datasource_id",
	"model_id",
	"job_id",
	"session_id",
	"request_id",
	"email",
}

// VecLabelLimits documents the maximum distinct values allowed per label on each
// *_Vec metric owned by this package. Update when adding a new labeled metric.
var VecLabelLimits = map[string]map[string]int{
	"biqly_ambiguity_by_source":          {"source": 3},
	"biqly_ambiguity_tier":               {"tier": 4},
	"bi_ai_repair_by_error_code_total":   {"code": 16},
	"biqly_memory_recall_feedback_total": {"recall": 2, "rating": 2},
}

var cardinalityRegistered sync.Map

type cardinalityCollector struct {
	gatherer  prometheus.Gatherer
	gatherMu  sync.Mutex
	gathering bool
}

func newCardinalityCollector(gatherer prometheus.Gatherer) *cardinalityCollector {
	return &cardinalityCollector{gatherer: gatherer}
}

func (*cardinalityCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- prometheus.NewDesc(
		"bi_prom_metric_series_total",
		"Number of Prometheus time series per metric family (excluding bi_prom_* introspection metrics).",
		[]string{"metric"},
		nil,
	)
	ch <- prometheus.NewDesc(
		"bi_prom_label_cardinality",
		"Number of distinct label values per metric family and label name.",
		[]string{"metric", "label"},
		nil,
	)
}

func (c *cardinalityCollector) Collect(ch chan<- prometheus.Metric) {
	c.gatherMu.Lock()
	if c.gathering {
		c.gatherMu.Unlock()
		return
	}
	c.gathering = true
	c.gatherMu.Unlock()
	defer func() {
		c.gatherMu.Lock()
		c.gathering = false
		c.gatherMu.Unlock()
	}()

	mfs, err := c.gatherer.Gather()
	if err != nil {
		return
	}
	seriesByMetric, labelCard := countMetricCardinality(mfs)
	for metric, count := range seriesByMetric {
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc("bi_prom_metric_series_total", "", []string{"metric"}, nil),
			prometheus.GaugeValue,
			float64(count),
			metric,
		)
	}
	for metric, labels := range labelCard {
		for label, count := range labels {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc("bi_prom_label_cardinality", "", []string{"metric", "label"}, nil),
				prometheus.GaugeValue,
				float64(count),
				metric, label,
			)
		}
	}
}

func countMetricCardinality(mfs []*dto.MetricFamily) (map[string]int, map[string]map[string]int) {
	seriesByMetric := make(map[string]int)
	labelCard := make(map[string]map[string]int)
	for _, mf := range mfs {
		name := mf.GetName()
		if strings.HasPrefix(name, "bi_prom_") {
			continue
		}
		seriesByMetric[name] = len(mf.GetMetric())
	}
	// Distinct label values per (metric, label).
	distinct := make(map[string]map[string]map[string]struct{})
	for _, mf := range mfs {
		name := mf.GetName()
		if strings.HasPrefix(name, "bi_prom_") {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, lp := range m.GetLabel() {
				labelName := lp.GetName()
				if distinct[name] == nil {
					distinct[name] = make(map[string]map[string]struct{})
				}
				if distinct[name][labelName] == nil {
					distinct[name][labelName] = make(map[string]struct{})
				}
				distinct[name][labelName][lp.GetValue()] = struct{}{}
			}
		}
	}
	for metric, labels := range distinct {
		if labelCard[metric] == nil {
			labelCard[metric] = make(map[string]int)
		}
		for label, values := range labels {
			labelCard[metric][label] = len(values)
		}
	}
	return seriesByMetric, labelCard
}

func registerCardinalityCollector(reg prometheus.Registerer, gatherer prometheus.Gatherer) {
	if gatherer == nil {
		return
	}
	if _, loaded := cardinalityRegistered.LoadOrStore(gatherer, true); loaded {
		return
	}
	c := newCardinalityCollector(gatherer)
	if err := reg.Register(c); err != nil {
		var already prometheus.AlreadyRegisteredError
		if !errors.As(err, &already) {
			cardinalityRegistered.Delete(gatherer)
		}
	}
}

// BoundLabel returns value when it is listed in allowed; otherwise fallback.
// Use for every *_Vec label value to keep cardinality bounded.
func BoundLabel(value string, allowed []string, fallback string) string {
	if slices.Contains(allowed, value) {
		return value
	}
	return fallback
}

// CheckGatheredCardinality returns an error when gathered metrics exceed
// VecLabelLimits or use forbidden label names.
func CheckGatheredCardinality(gatherer prometheus.Gatherer) error {
	mfs, err := gatherer.Gather()
	if err != nil {
		return err
	}
	_, labelCard := countMetricCardinality(mfs)
	for metric, labels := range labelCard {
		for label, count := range labels {
			if slices.Contains(ForbiddenMetricLabels, label) {
				return &cardinalityError{metric: metric, label: label, msg: "forbidden high-cardinality label name"}
			}
			limit := labelLimit(metric, label)
			if count > limit {
				return &cardinalityError{metric: metric, label: label, got: count, limit: limit}
			}
		}
	}
	return nil
}

func labelLimit(metric, label string) int {
	if limits, ok := VecLabelLimits[metric]; ok {
		if maxVals, ok := limits[label]; ok {
			return maxVals
		}
	}
	return DefaultMaxLabelValues
}

type cardinalityError struct {
	metric, label, msg string
	got, limit         int
}

func (e *cardinalityError) Error() string {
	if e.msg != "" {
		return "metric " + e.metric + ": label " + e.label + ": " + e.msg
	}
	return "metric " + e.metric + ": label " + e.label + " cardinality " + strconv.Itoa(e.got) + " exceeds limit " + strconv.Itoa(e.limit)
}
