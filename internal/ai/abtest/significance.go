package abtest

import (
	"math"
)

// SignificanceResult holds the outputs of a statistical hypothesis test.
type SignificanceResult struct {
	IsSignificant bool    `json:"is_significant"`
	PValue        float64 `json:"p_value"`
	Confidence    float64 `json:"confidence"` // percentage: (1 - p_value) * 100
}

// CompareProportions performs a two-proportion z-test to compare two rates.
// n1, n2 are the sample sizes; p1, p2 are the observed rates (0.0 to 1.0).
func CompareProportions(n1, n2 int, p1, p2 float64, alpha float64) SignificanceResult {
	if n1 <= 0 || n2 <= 0 {
		return SignificanceResult{PValue: 1.0, Confidence: 0.0}
	}
	x1 := p1 * float64(n1)
	x2 := p2 * float64(n2)
	pPooled := (x1 + x2) / float64(n1+n2)
	if pPooled <= 0 || pPooled >= 1 {
		return SignificanceResult{PValue: 1.0, Confidence: 0.0}
	}
	se := math.Sqrt(pPooled * (1.0 - pPooled) * (1.0/float64(n1) + 1.0/float64(n2)))
	if se == 0 {
		return SignificanceResult{PValue: 1.0, Confidence: 0.0}
	}
	z := (p1 - p2) / se
	pValue := erfc(math.Abs(z) / math.Sqrt(2))
	confidence := (1.0 - pValue) * 100.0
	return SignificanceResult{
		IsSignificant: pValue < alpha,
		PValue:        pValue,
		Confidence:    confidence,
	}
}

// CompareMeans performs a two-sample t-test (approximated as z-test) to compare averages.
// n1, n2 are the sample sizes; mean1, mean2 are the averages; stdDev1, stdDev2 are the sample standard deviations.
func CompareMeans(n1, n2 int, mean1, mean2 float64, stdDev1, stdDev2 float64, alpha float64) SignificanceResult {
	if n1 <= 0 || n2 <= 0 {
		return SignificanceResult{PValue: 1.0, Confidence: 0.0}
	}
	// Avoid division by zero if both standard deviations are zero
	if stdDev1 == 0 && stdDev2 == 0 {
		if mean1 == mean2 {
			return SignificanceResult{PValue: 1.0, Confidence: 0.0}
		}
		return SignificanceResult{IsSignificant: true, PValue: 0.0, Confidence: 100.0}
	}
	se := math.Sqrt((stdDev1*stdDev1)/float64(n1) + (stdDev2*stdDev2)/float64(n2))
	if se == 0 {
		return SignificanceResult{PValue: 1.0, Confidence: 0.0}
	}
	t := (mean1 - mean2) / se
	pValue := erfc(math.Abs(t) / math.Sqrt(2))
	confidence := (1.0 - pValue) * 100.0
	return SignificanceResult{
		IsSignificant: pValue < alpha,
		PValue:        pValue,
		Confidence:    confidence,
	}
}

// erfc is the complementary error function.
func erfc(x float64) float64 {
	return 1.0 - math.Erf(x)
}
