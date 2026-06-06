package abtest

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Recommendation contains the outcome of the automated decision helper.
type Recommendation struct {
	WinnerVariantID  string             `json:"winner_variant_id"`
	Reason           string             `json:"reason"`
	MinSampleReached bool               `json:"min_sample_reached"`
	SampleSize       int                `json:"sample_size"`
	Significance     SignificanceResult `json:"significance"`
}

// Recommender generates recommendations based on experiment metrics.
type Recommender struct {
	repo                   *Repository
	collector              *MetricsCollector
	minSampleSizeThreshold int
	significanceThreshold  float64
}

// NewRecommender creates a new Recommender.
func NewRecommender(repo *Repository, collector *MetricsCollector) *Recommender {
	return &Recommender{
		repo:                   repo,
		collector:              collector,
		minSampleSizeThreshold: minSampleSizeThresholdFromEnv(),
		significanceThreshold:  significanceThresholdFromEnv(),
	}
}

// Recommend analyzes metrics and recommends a winner variant or actions.
func (r *Recommender) Recommend(ctx context.Context, experimentID string) (*Recommendation, error) {
	exp, err := r.repo.GetExperiment(ctx, experimentID)
	if err != nil {
		return nil, fmt.Errorf("get experiment for recommendation: %w", err)
	}

	variants, err := r.repo.ListVariants(ctx, experimentID)
	if err != nil {
		return nil, fmt.Errorf("list variants for recommendation: %w", err)
	}
	if len(variants) < 2 {
		return &Recommendation{
			Reason: "At least two variants are required to make a recommendation.",
		}, nil
	}

	periodStart, periodEnd := r.getPeriod(exp)

	// Fetch metrics
	metrics, err := r.collector.ComputeMetrics(ctx, experimentID, periodStart, periodEnd)
	if err != nil {
		return nil, fmt.Errorf("compute metrics for recommendation: %w", err)
	}

	metricsMap := make(map[string]ExperimentMetrics)
	for i := range metrics {
		metricsMap[metrics[i].VariantID] = metrics[i]
	}

	_, controlMetrics, err := r.locateControl(variants, metricsMap)
	if err != nil {
		return nil, err
	}

	minSampleSize := r.getMinSampleSizeThreshold()
	alpha := r.getSignificanceThreshold()

	controlN := 0
	if controlMetrics != nil {
		controlN = controlMetrics.TotalQueries
	}

	minSampleReached, totalSampleCount := r.checkSampleSizes(variants, metricsMap, minSampleSize, controlN)
	if !minSampleReached {
		return &Recommendation{
			Reason:           fmt.Sprintf("Sample size threshold of %d per variant not met. Continue collecting data.", minSampleSize),
			MinSampleReached: false,
			SampleSize:       totalSampleCount,
		}, nil
	}

	controlP := 0.0
	if controlMetrics != nil {
		controlP = controlMetrics.SuccessRate
	}

	bestTreatment, bestDelta, bestTreatmentSig, worstTreatment, worstDelta, worstTreatmentSig := r.evaluateVariants(
		variants, metricsMap, controlN, controlP, alpha,
	)

	// If there's a significantly worse treatment, warn to stop the experiment
	if worstTreatment != nil {
		return &Recommendation{
			Reason:           fmt.Sprintf("Warning: Variant %q is performing significantly worse than the control (p=%.4f, delta=%.1f%%). Recommend stopping the experiment.", worstTreatment.Name, worstTreatmentSig.PValue, worstDelta*100.0),
			MinSampleReached: true,
			SampleSize:       totalSampleCount,
			Significance:     worstTreatmentSig,
		}, nil
	}

	// If there's a winner, recommend promoting it
	if bestTreatment != nil {
		return &Recommendation{
			WinnerVariantID:  bestTreatment.ID,
			Reason:           fmt.Sprintf("Variant %q is statistically significantly better than Control (p=%.4f) with an improvement of %.1f%% in success rate.", bestTreatment.Name, bestTreatmentSig.PValue, bestDelta*100.0),
			MinSampleReached: true,
			SampleSize:       totalSampleCount,
			Significance:     bestTreatmentSig,
		}, nil
	}

	// Default recommendation if no winner/loser is found
	return &Recommendation{
		Reason:           "No statistically significant difference detected. You can safely complete the experiment and choose either variant, or continue running if you expect long-term behavioral changes.",
		MinSampleReached: true,
		SampleSize:       totalSampleCount,
		Significance:     SignificanceResult{IsSignificant: false, PValue: 1.0, Confidence: 0.0},
	}, nil
}

func (*Recommender) getPeriod(exp *Experiment) (time.Time, time.Time) {
	periodStart := exp.CreatedAt
	if exp.StartedAt != nil {
		periodStart = *exp.StartedAt
	}
	periodEnd := time.Now()
	if exp.EndedAt != nil {
		periodEnd = *exp.EndedAt
	}
	return periodStart, periodEnd
}

func (*Recommender) locateControl(variants []Variant, metricsMap map[string]ExperimentMetrics) (*Variant, *ExperimentMetrics, error) {
	var controlVariant *Variant
	var controlMetrics *ExperimentMetrics
	for i := range variants {
		if variants[i].IsControl {
			controlVariant = &variants[i]
			if m, ok := metricsMap[variants[i].ID]; ok {
				controlMetrics = &m
			}
			break
		}
	}
	if controlVariant == nil {
		return nil, nil, errors.New("control variant not found")
	}
	return controlVariant, controlMetrics, nil
}

func (r *Recommender) getMinSampleSizeThreshold() int {
	return r.minSampleSizeThreshold
}

func minSampleSizeThresholdFromEnv() int {
	minSampleSize := 100
	if val := os.Getenv("BI_AB_MIN_SAMPLE_SIZE"); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			minSampleSize = i
		}
	}
	return minSampleSize
}

func (r *Recommender) getSignificanceThreshold() float64 {
	return r.significanceThreshold
}

func significanceThresholdFromEnv() float64 {
	alpha := 0.05
	if val := os.Getenv("BI_AB_SIGNIFICANCE_THRESHOLD"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			alpha = f
		}
	}
	return alpha
}

func (*Recommender) checkSampleSizes(variants []Variant, metricsMap map[string]ExperimentMetrics, minSampleSize int, controlN int) (bool, int) {
	minSampleReached := controlN >= minSampleSize
	totalSampleCount := controlN

	for i := range variants {
		if variants[i].IsControl {
			continue
		}
		var n int
		if m, ok := metricsMap[variants[i].ID]; ok {
			n = m.TotalQueries
		}
		totalSampleCount += n
		if n < minSampleSize {
			minSampleReached = false
		}
	}
	return minSampleReached, totalSampleCount
}

func (*Recommender) evaluateVariants(
	variants []Variant,
	metricsMap map[string]ExperimentMetrics,
	controlN int,
	controlP float64,
	alpha float64,
) (
	bestTreatment *Variant,
	bestDelta float64,
	bestTreatmentSig SignificanceResult,
	worstTreatment *Variant,
	worstDelta float64,
	worstTreatmentSig SignificanceResult,
) {
	bestDelta = 0.0
	worstDelta = 0.0

	for i := range variants {
		v := &variants[i]
		if v.IsControl {
			continue
		}
		tm, ok := metricsMap[v.ID]
		if !ok || tm.TotalQueries == 0 {
			continue
		}

		sig := CompareProportions(controlN, tm.TotalQueries, controlP, tm.SuccessRate, alpha)
		delta := tm.SuccessRate - controlP

		// Check if it's statistically significant and better
		if sig.IsSignificant && delta >= 0.02 {
			if delta > bestDelta {
				bestDelta = delta
				bestTreatment = v
				bestTreatmentSig = sig
			}
		}

		// Check if it's statistically significant and worse
		if sig.IsSignificant && delta <= -0.02 {
			if delta < worstDelta {
				worstDelta = delta
				worstTreatment = v
				worstTreatmentSig = sig
			}
		}
	}
	return
}
