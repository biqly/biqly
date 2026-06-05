package abtest

import (
	"errors"
	"fmt"
	"hash/fnv"
	"time"
)

// ExperimentStatus is the lifecycle state for a prompt A/B test.
type ExperimentStatus string

const (
	ExperimentStatusDraft     ExperimentStatus = "draft"
	ExperimentStatusRunning   ExperimentStatus = "running"
	ExperimentStatusPaused    ExperimentStatus = "paused"
	ExperimentStatusCompleted ExperimentStatus = "completed"
)

// Experiment describes one prompt-template A/B test.
type Experiment struct {
	ID           string
	Name         string
	Description  string
	TemplateName string
	Locale       string
	Status       ExperimentStatus
	StartedAt    *time.Time
	EndedAt      *time.Time
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Variant maps a traffic percentage to a specific prompt template version.
type Variant struct {
	ID              string
	ExperimentID    string
	Name            string
	TemplateVersion int
	TrafficPct      int
	IsControl       bool
}

// ExperimentMetrics contains aggregated performance metrics for one variant.
type ExperimentMetrics struct {
	ExperimentID         string
	VariantID            string
	PeriodStart          time.Time
	PeriodEnd            time.Time
	TotalQueries         int
	SuccessRate          float64
	ValidatorPassRate    float64
	AvgConfidence        float64
	UserCorrectionRate   float64
	PositiveFeedbackRate float64
	ExecutionSuccessRate float64
	AvgCostUSD           float64
	AvgLatencyMs         float64
	TotalTokens          int
}

// ValidateVariantsForAllocation verifies the traffic allocation invariants.
func ValidateVariantsForAllocation(variants []Variant) error {
	if len(variants) == 0 {
		return errors.New("at least one variant is required")
	}
	totalTraffic := 0
	controlCount := 0
	for _, variant := range variants {
		if variant.TrafficPct < 0 || variant.TrafficPct > 100 {
			return fmt.Errorf("variant %q traffic_pct must be between 0 and 100", variant.ID)
		}
		totalTraffic += variant.TrafficPct
		if variant.IsControl {
			controlCount++
		}
	}
	if totalTraffic != 100 {
		return fmt.Errorf("variant traffic_pct must sum to 100, got %d", totalTraffic)
	}
	if controlCount != 1 {
		return fmt.Errorf("exactly one control variant is required, got %d", controlCount)
	}
	return nil
}

// SelectVariantForUser deterministically assigns a user to a variant.
func SelectVariantForUser(userID, experimentID string, variants []Variant) (Variant, error) {
	if err := ValidateVariantsForAllocation(variants); err != nil {
		return Variant{}, err
	}
	return selectVariantForBucket(trafficBucket(userID, experimentID), variants)
}

func selectVariantForBucket(bucket int, variants []Variant) (Variant, error) {
	if bucket < 0 || bucket > 99 {
		return Variant{}, fmt.Errorf("traffic bucket must be between 0 and 99, got %d", bucket)
	}
	cumulative := 0
	for _, variant := range variants {
		if variant.TrafficPct == 0 {
			continue
		}
		cumulative += variant.TrafficPct
		if bucket < cumulative {
			return variant, nil
		}
	}
	return Variant{}, errors.New("no variant selected")
}

func trafficBucket(userID, experimentID string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(userID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(experimentID))
	return int(h.Sum32() % 100)
}
