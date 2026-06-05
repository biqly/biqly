package abtest

import (
	"strconv"
	"testing"
)

func TestSelectVariantForUserIsDeterministic(t *testing.T) {
	variants := []Variant{
		{ID: "control", TrafficPct: 50, IsControl: true},
		{ID: "treatment", TrafficPct: 50},
	}

	first, err := SelectVariantForUser("user_123", "experiment_456", variants)
	if err != nil {
		t.Fatalf("SelectVariantForUser() error = %v", err)
	}
	for range 20 {
		got, err := SelectVariantForUser("user_123", "experiment_456", variants)
		if err != nil {
			t.Fatalf("SelectVariantForUser() repeat error = %v", err)
		}
		if got.ID != first.ID {
			t.Fatalf("SelectVariantForUser() = %q, want deterministic %q", got.ID, first.ID)
		}
	}
}

func TestSelectVariantForUserRespectsTrafficRanges(t *testing.T) {
	variants := []Variant{
		{ID: "control", TrafficPct: 30, IsControl: true},
		{ID: "treatment_a", TrafficPct: 30},
		{ID: "treatment_b", TrafficPct: 40},
	}

	seen := map[string]bool{}
	for i := range 500 {
		got, err := SelectVariantForUser("user_"+strconv.Itoa(i), "experiment_456", variants)
		if err != nil {
			t.Fatalf("SelectVariantForUser() error = %v", err)
		}
		seen[got.ID] = true
	}

	for _, id := range []string{"control", "treatment_a", "treatment_b"} {
		if !seen[id] {
			t.Fatalf("variant %q was never selected; seen=%v", id, seen)
		}
	}
}

func TestSelectVariantForUserSkipsZeroTrafficVariants(t *testing.T) {
	variants := []Variant{
		{ID: "control", TrafficPct: 100, IsControl: true},
		{ID: "paused_treatment", TrafficPct: 0},
	}

	for i := range 100 {
		got, err := SelectVariantForUser("user_"+strconv.Itoa(i), "experiment_456", variants)
		if err != nil {
			t.Fatalf("SelectVariantForUser() error = %v", err)
		}
		if got.ID != "control" {
			t.Fatalf("SelectVariantForUser() = %q, want control", got.ID)
		}
	}
}

func TestSelectVariantForBucketUsesCumulativeBoundaries(t *testing.T) {
	variants := []Variant{
		{ID: "control", TrafficPct: 30, IsControl: true},
		{ID: "treatment_a", TrafficPct: 30},
		{ID: "treatment_b", TrafficPct: 40},
	}

	tests := []struct {
		name   string
		bucket int
		wantID string
	}{
		{name: "first bucket", bucket: 0, wantID: "control"},
		{name: "last control bucket", bucket: 29, wantID: "control"},
		{name: "first treatment a bucket", bucket: 30, wantID: "treatment_a"},
		{name: "last treatment a bucket", bucket: 59, wantID: "treatment_a"},
		{name: "first treatment b bucket", bucket: 60, wantID: "treatment_b"},
		{name: "last bucket", bucket: 99, wantID: "treatment_b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectVariantForBucket(tt.bucket, variants)
			if err != nil {
				t.Fatalf("selectVariantForBucket() error = %v", err)
			}
			if got.ID != tt.wantID {
				t.Fatalf("selectVariantForBucket(%d) = %q, want %q", tt.bucket, got.ID, tt.wantID)
			}
		})
	}
}

func TestValidateVariantsForAllocation(t *testing.T) {
	tests := []struct {
		name     string
		variants []Variant
		wantErr  bool
	}{
		{
			name: "valid",
			variants: []Variant{
				{ID: "control", TrafficPct: 50, IsControl: true},
				{ID: "treatment", TrafficPct: 50},
			},
		},
		{
			name: "traffic must sum to 100",
			variants: []Variant{
				{ID: "control", TrafficPct: 60, IsControl: true},
				{ID: "treatment", TrafficPct: 30},
			},
			wantErr: true,
		},
		{
			name: "requires one control",
			variants: []Variant{
				{ID: "a", TrafficPct: 50},
				{ID: "b", TrafficPct: 50},
			},
			wantErr: true,
		},
		{
			name: "rejects more than one control",
			variants: []Variant{
				{ID: "a", TrafficPct: 50, IsControl: true},
				{ID: "b", TrafficPct: 50, IsControl: true},
			},
			wantErr: true,
		},
		{
			name: "rejects negative traffic",
			variants: []Variant{
				{ID: "control", TrafficPct: 101, IsControl: true},
				{ID: "treatment", TrafficPct: -1},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVariantsForAllocation(tt.variants)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateVariantsForAllocation() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
