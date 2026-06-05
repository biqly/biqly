package abtest

import (
	"math"
	"testing"
)

func TestCompareProportions(t *testing.T) {
	tests := []struct {
		name          string
		n1, n2        int
		p1, p2        float64
		alpha         float64
		wantSig       bool
		minP          float64
		maxP          float64
	}{
		{
			name:    "invalid sample size n1",
			n1:      0,
			n2:      100,
			p1:      0.5,
			p2:      0.5,
			alpha:   0.05,
			wantSig: false,
			minP:    1.0,
			maxP:    1.0,
		},
		{
			name:    "invalid sample size n2",
			n1:      100,
			n2:      -5,
			p1:      0.5,
			p2:      0.5,
			alpha:   0.05,
			wantSig: false,
			minP:    1.0,
			maxP:    1.0,
		},
		{
			name:    "identical rates",
			n1:      100,
			n2:      100,
			p1:      0.6,
			p2:      0.6,
			alpha:   0.05,
			wantSig: false,
			minP:    0.99,
			maxP:    1.0,
		},
		{
			name:    "highly significant difference",
			n1:      200,
			n2:      200,
			p1:      0.85,
			p2:      0.50,
			alpha:   0.05,
			wantSig: true,
			minP:    0.0,
			maxP:    0.001,
		},
		{
			name:    "marginally non-significant difference",
			n1:      50,
			n2:      50,
			p1:      0.60,
			p2:      0.55,
			alpha:   0.05,
			wantSig: false,
			minP:    0.5,
			maxP:    0.8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := CompareProportions(tt.n1, tt.n2, tt.p1, tt.p2, tt.alpha)
			if res.IsSignificant != tt.wantSig {
				t.Errorf("CompareProportions() IsSignificant = %v, want %v", res.IsSignificant, tt.wantSig)
			}
			if res.PValue < tt.minP || res.PValue > tt.maxP {
				t.Errorf("CompareProportions() PValue = %v, want between [%v, %v]", res.PValue, tt.minP, tt.maxP)
			}
			expectedConf := (1.0 - res.PValue) * 100.0
			if math.Abs(res.Confidence-expectedConf) > 1e-9 {
				t.Errorf("CompareProportions() Confidence = %v, want %v", res.Confidence, expectedConf)
			}
		})
	}
}

func TestCompareMeans(t *testing.T) {
	tests := []struct {
		name             string
		n1, n2           int
		mean1, mean2     float64
		stdDev1, stdDev2 float64
		alpha            float64
		wantSig          bool
		minP             float64
		maxP             float64
	}{
		{
			name:             "invalid sample size",
			n1:               0,
			n2:               10,
			mean1:            5.0,
			mean2:            5.0,
			stdDev1:          1.0,
			stdDev2:          1.0,
			alpha:            0.05,
			wantSig:          false,
			minP:             1.0,
			maxP:             1.0,
		},
		{
			name:             "both standard deviations zero, means equal",
			n1:               10,
			n2:               10,
			mean1:            5.0,
			mean2:            5.0,
			stdDev1:          0.0,
			stdDev2:          0.0,
			alpha:            0.05,
			wantSig:          false,
			minP:             1.0,
			maxP:             1.0,
		},
		{
			name:             "both standard deviations zero, means different",
			n1:               10,
			n2:               10,
			mean1:            5.0,
			mean2:            6.0,
			stdDev1:          0.0,
			stdDev2:          0.0,
			alpha:            0.05,
			wantSig:          true,
			minP:             0.0,
			maxP:             0.0,
		},
		{
			name:             "no significant difference",
			n1:               30,
			n2:               30,
			mean1:            10.0,
			mean2:            10.5,
			stdDev1:          2.0,
			stdDev2:          2.0,
			alpha:            0.05,
			wantSig:          false,
			minP:             0.3,
			maxP:             0.4,
		},
		{
			name:             "highly significant difference",
			n1:               100,
			n2:               100,
			mean1:            50.0,
			mean2:            40.0,
			stdDev1:          5.0,
			stdDev2:          5.0,
			alpha:            0.05,
			wantSig:          true,
			minP:             0.0,
			maxP:             0.000001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := CompareMeans(tt.n1, tt.n2, tt.mean1, tt.mean2, tt.stdDev1, tt.stdDev2, tt.alpha)
			if res.IsSignificant != tt.wantSig {
				t.Errorf("CompareMeans() IsSignificant = %v, want %v", res.IsSignificant, tt.wantSig)
			}
			if res.PValue < tt.minP || res.PValue > tt.maxP {
				t.Errorf("CompareMeans() PValue = %v, want between [%v, %v]", res.PValue, tt.minP, tt.maxP)
			}
		})
	}
}
