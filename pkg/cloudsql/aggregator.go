package cloudsql

import "sort"

// CalculateStats calculates statistical aggregates from time series points
func CalculateStats(points []float64, unit string) Stats {
	if len(points) == 0 {
		return Stats{Unit: unit}
	}

	// Sort for percentile calculations
	sorted := make([]float64, len(points))
	copy(sorted, points)
	sort.Float64s(sorted)

	return Stats{
		Current: sorted[len(sorted)-1], // Most recent (assumes sorted by time originally)
		P50:     percentile(sorted, 0.50),
		P99:     percentile(sorted, 0.99),
		Max:     sorted[len(sorted)-1],
		Min:     sorted[0],
		Avg:     average(sorted),
		Unit:    unit,
	}
}

// percentile calculates the percentile from a sorted slice
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}

	if len(sorted) == 1 {
		return sorted[0]
	}

	// Calculate position with linear interpolation
	pos := p * float64(len(sorted)+1)

	// Handle edge cases
	if pos <= 1 {
		return sorted[0]
	}
	if pos >= float64(len(sorted)) {
		return sorted[len(sorted)-1]
	}

	// Get the two values to interpolate between
	lower := int(pos) - 1
	upper := lower + 1

	// Linear interpolation
	fraction := pos - float64(int(pos))
	return sorted[lower] + fraction*(sorted[upper]-sorted[lower])
}

// average calculates the mean of a slice
func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}

	return sum / float64(len(values))
}
