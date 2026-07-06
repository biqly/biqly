package semantic

import "testing"

func TestIsValidRateBehavior(t *testing.T) {
	valid := []string{"", RateBehaviorRatioOfSums, RateBehaviorAverageOfCustomerRates, RateBehaviorWeightedAverage, RateBehaviorLatestValue}
	for _, v := range valid {
		if !IsValidRateBehavior(v) {
			t.Errorf("expected %q to be a valid rate behavior", v)
		}
	}
	for _, v := range []string{"bogus", "RATIO_OF_SUMS", "ratio of sums"} {
		if IsValidRateBehavior(v) {
			t.Errorf("expected %q to be rejected", v)
		}
	}
}
