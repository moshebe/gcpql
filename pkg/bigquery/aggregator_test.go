package bigquery

import (
	"math"
	"testing"
)

func TestCalculateStats(t *testing.T) {
	tests := []struct {
		name     string
		points   []float64
		expected Stats
	}{
		{
			name:   "single point",
			points: []float64{50.0},
			expected: Stats{
				Current: 50.0,
				P50:     50.0,
				P99:     50.0,
				Max:     50.0,
				Min:     50.0,
				Avg:     50.0,
			},
		},
		{
			name:   "multiple points",
			points: []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
			expected: Stats{
				Current: 100.0,
				P50:     55.0,
				P99:     99.1,
				Max:     100.0,
				Min:     10.0,
				Avg:     55.0,
			},
		},
		{
			name:   "unsorted input",
			points: []float64{50, 10, 90, 30, 70},
			expected: Stats{
				Current: 70.0, // last value before sort
				P50:     50.0,
				P99:     89.2, // linear interpolation between 70 and 90
				Max:     90.0,
				Min:     10.0,
				Avg:     50.0,
			},
		},
		{
			name:     "empty points",
			points:   []float64{},
			expected: Stats{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateStats(tt.points)

			if !floatEqual(result.Current, tt.expected.Current) {
				t.Errorf("Current: got %.2f, want %.2f", result.Current, tt.expected.Current)
			}
			if !floatEqual(result.Avg, tt.expected.Avg) {
				t.Errorf("Avg: got %.2f, want %.2f", result.Avg, tt.expected.Avg)
			}
			if !floatEqual(result.P50, tt.expected.P50) {
				t.Errorf("P50: got %.2f, want %.2f", result.P50, tt.expected.P50)
			}
			if !floatEqual(result.P99, tt.expected.P99) {
				t.Errorf("P99: got %.2f, want %.2f", result.P99, tt.expected.P99)
			}
			if !floatEqual(result.Min, tt.expected.Min) {
				t.Errorf("Min: got %.2f, want %.2f", result.Min, tt.expected.Min)
			}
			if !floatEqual(result.Max, tt.expected.Max) {
				t.Errorf("Max: got %.2f, want %.2f", result.Max, tt.expected.Max)
			}
		})
	}
}

func floatEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.01
}
