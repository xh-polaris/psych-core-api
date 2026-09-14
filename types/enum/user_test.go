package enum

import "testing"

func TestRiskLevelToIntSupportsV2NumericValues(t *testing.T) {
	for input, want := range map[string]int{
		"1": UserRiskLevelHigh,
		"2": UserRiskLevelMedium,
		"3": UserRiskLevelLow,
		"4": UserRiskLevelNormal,
	} {
		if got := RiskLevelToInt(input); got != want {
			t.Fatalf("RiskLevelToInt(%q) = %d, want %d", input, got, want)
		}
	}
}
