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
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	TemplateName string           `json:"template_name"`
	Locale       string           `json:"locale"`
	Status       ExperimentStatus `json:"status"`
	StartedAt    *time.Time       `json:"started_at,omitempty"`
	EndedAt      *time.Time       `json:"ended_at,omitempty"`
	CreatedBy    string           `json:"created_by"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

// Variant maps a traffic percentage to a specific prompt template version.
type Variant struct {
	ID              string `json:"id"`
	ExperimentID    string `json:"experiment_id"`
	Name            string `json:"name"`
	TemplateVersion int    `json:"template_version"`
	TrafficPct      int    `json:"traffic_pct"`
	IsControl       bool   `json:"is_control"`
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
	StdDevCostUSD        float64
	StdDevLatencyMs      float64
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
	if _, err := h.Write([]byte(userID)); err != nil {
		return 0
	}
	if _, err := h.Write([]byte{0}); err != nil {
		return 0
	}
	if _, err := h.Write([]byte(experimentID)); err != nil {
		return 0
	}
	return int(h.Sum32() % 100)
}
