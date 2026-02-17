package cloudsql

import (
	"testing"
)

func TestCalculateStats(t *testing.T) {
	tests := []struct {
		name   string
		points []float64
		want   Stats
	}{
		{
			name:   "normal distribution",
			points: []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
			want: Stats{
				Current: 100,
				P50:     55,
				P99:     100,
				Max:     100,
				Min:     10,
				Avg:     55,
			},
		},
		{
			name:   "single value",
			points: []float64{42},
			want: Stats{
				Current: 42,
				P50:     42,
				P99:     42,
				Max:     42,
				Min:     42,
				Avg:     42,
			},
		},
		{
			name:   "empty slice",
			points: []float64{},
			want:   Stats{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateStats(tt.points, "unit")

			if got.Current != tt.want.Current {
				t.Errorf("Current = %v, want %v", got.Current, tt.want.Current)
			}
			if got.P50 != tt.want.P50 {
				t.Errorf("P50 = %v, want %v", got.P50, tt.want.P50)
			}
			if got.P99 != tt.want.P99 {
				t.Errorf("P99 = %v, want %v", got.P99, tt.want.P99)
			}
			if got.Max != tt.want.Max {
				t.Errorf("Max = %v, want %v", got.Max, tt.want.Max)
			}
			if got.Min != tt.want.Min {
				t.Errorf("Min = %v, want %v", got.Min, tt.want.Min)
			}
			if got.Avg != tt.want.Avg {
				t.Errorf("Avg = %v, want %v", got.Avg, tt.want.Avg)
			}
		})
	}
}

func TestPercentile(t *testing.T) {
	tests := []struct {
		name   string
		sorted []float64
		p      float64
		want   float64
	}{
		{
			name:   "p50 of 10 values",
			sorted: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			p:      0.50,
			want:   5.5,
		},
		{
			name:   "p99 of 100 values",
			sorted: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20,
				21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40,
				41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55, 56, 57, 58, 59, 60,
				61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79, 80,
				81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97, 98, 99, 100},
			p:    0.99,
			want: 99.99,
		},
		{
			name:   "single value",
			sorted: []float64{42},
			p:      0.99,
			want:   42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentile(tt.sorted, tt.p)
			if got != tt.want {
				t.Errorf("percentile() = %v, want %v", got, tt.want)
			}
		})
	}
}
