package bigquery

import (
	"sort"
)

// Stats represents statistical aggregates
type Stats struct {
	Current float64 `json:"current,omitempty"`
	P50     float64 `json:"p50,omitempty"`
	P99     float64 `json:"p99,omitempty"`
	Max     float64 `json:"max,omitempty"`
	Min     float64 `json:"min,omitempty"`
	Avg     float64 `json:"avg,omitempty"`
}

// CalculateStats computes statistical aggregates from time series points
func CalculateStats(points []float64) Stats {
	if len(points) == 0 {
		return Stats{}
	}

	// Capture current (last value) before sorting
	current := points[len(points)-1]

	// Sort for percentile calculation
	sorted := make([]float64, len(points))
	copy(sorted, points)
	sort.Float64s(sorted)

	// Calculate aggregates
	var sum float64
	for _, v := range sorted {
		sum += v
	}

	stats := Stats{
		Current: current,
		Min:     sorted[0],
		Max:     sorted[len(sorted)-1],
		Avg:     sum / float64(len(sorted)),
	}

	// Percentiles
	stats.P50 = percentile(sorted, 0.50)
	stats.P99 = percentile(sorted, 0.99)

	return stats
}

// percentile returns the value at the given percentile (0.0 to 1.0)
// Requires: sorted must be sorted and non-empty, 0.0 <= p <= 1.0
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}

	rank := p * float64(len(sorted)-1)
	lower := int(rank)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	weight := rank - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}
