// Package pii implements detection of personally identifiable information in
// column metadata and sample data, plus role-based masking of PII columns at
// the SQL compilation layer.
package pii

import (
	"sort"
	"strings"
)

// DefaultThreshold is the minimum combined confidence required to flag a
// column as PII.
const DefaultThreshold = 0.6

// Scoring weights: a column-name keyword match contributes a fixed base
// score, sample-data pattern matches contribute proportionally to the match
// ratio. Final confidence = nameBaseScore + ratio*sampleMaxScore.
const (
	nameBaseScore  = 0.5
	sampleMaxScore = 0.5
)

// PIIResult sources.
const (
	SourceName       = "name"
	SourceSample     = "sample"
	SourceNameSample = "name+sample"
	SourceManual     = "manual"
)

// PIIResult is a single PII detection finding for a column.
type PIIResult struct {
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
}

// Detector flags columns as PII using column-name heuristics and regex scans
// over sample data.
type Detector struct {
	threshold float64
}

// NewDetector creates a Detector with the given confidence threshold.
// Non-positive thresholds fall back to DefaultThreshold.
func NewDetector(threshold float64) *Detector {
	if threshold <= 0 {
		threshold = DefaultThreshold
	}
	return &Detector{threshold: threshold}
}

// Threshold returns the configured detection threshold.
func (d *Detector) Threshold() float64 { return d.threshold }

// DetectFromColumn scores every PII type against the column name and sample
// data, returning all findings at or above the threshold sorted by descending
// confidence. NULL-ish (empty / whitespace-only) samples are ignored.
func (d *Detector) DetectFromColumn(columnName string, sampleData []string) []PIIResult {
	samples := cleanSamples(sampleData)
	nameType := detectTypeFromName(columnName)

	var results []PIIResult
	for _, piiType := range AllTypes {
		confidence, source := score(piiType, piiType == nameType, samples)
		if source != "" && confidence >= d.threshold {
			results = append(results, PIIResult{Type: piiType, Confidence: confidence, Source: source})
		}
	}
	sort.SliceStable(results, func(i, j int) bool { return results[i].Confidence > results[j].Confidence })
	return results
}

// score computes the combined confidence for one PII type. Address is special:
// its sample check (long free text) is too weak to stand alone, so it only
// corroborates a name match.
func score(piiType string, nameMatched bool, samples []string) (confidence float64, source string) {
	if nameMatched {
		confidence = nameBaseScore
		source = SourceName
	}
	if piiType == TypeAddress && !nameMatched {
		return confidence, source
	}
	ratio := matchRatio(valueMatchers[piiType], samples)
	if ratio > 0 {
		confidence += ratio * sampleMaxScore
		if nameMatched {
			source = SourceNameSample
		} else {
			source = SourceSample
		}
	}
	return confidence, source
}

// matchRatio returns the fraction of samples matched by the matcher.
func matchRatio(match valueMatcher, samples []string) float64 {
	if match == nil || len(samples) == 0 {
		return 0
	}
	matched := 0
	for _, s := range samples {
		if match(s) {
			matched++
		}
	}
	return float64(matched) / float64(len(samples))
}

// cleanSamples drops empty and whitespace-only values so NULL-heavy columns
// do not dilute the match ratio.
func cleanSamples(samples []string) []string {
	cleaned := make([]string, 0, len(samples))
	for _, s := range samples {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return cleaned
}
