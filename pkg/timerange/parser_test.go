package timerange

import (
	"testing"
	"time"
)

func TestParse_RelativeTime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantDiff time.Duration
	}{
		{"5 minutes", "5m", 5 * time.Minute},
		{"1 hour", "1h", 1 * time.Hour},
		{"24 hours", "24h", 24 * time.Hour},
		{"7 days", "7d", 7 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := Parse(tt.input, "")
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}

			// End should be approximately now
			now := time.Now()
			if end.After(now.Add(time.Second)) || end.Before(now.Add(-time.Second)) {
				t.Errorf("End time %v not close to now %v", end, now)
			}

			// Start should be approximately now - wantDiff
			gotDiff := end.Sub(start)
			if gotDiff < tt.wantDiff-time.Second || gotDiff > tt.wantDiff+time.Second {
				t.Errorf("Time difference = %v, want ~%v", gotDiff, tt.wantDiff)
			}
		})
	}
}

func TestParse_InvalidFormat(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"invalid unit", "5x"},
		{"no number", "h"},
		{"negative", "-5m"},
		{"zero", "0m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := Parse(tt.input, "")
			if err == nil {
				t.Error("Parse() expected error, got nil")
			}
		})
	}
}

func TestParse_DefaultToFiveMinutes(t *testing.T) {
	start, end, err := Parse("", "")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	now := time.Now()
	if end.After(now.Add(time.Second)) || end.Before(now.Add(-time.Second)) {
		t.Errorf("End time %v not close to now %v", end, now)
	}

	gotDiff := end.Sub(start)
	wantDiff := 5 * time.Minute
	if gotDiff < wantDiff-time.Second || gotDiff > wantDiff+time.Second {
		t.Errorf("Time difference = %v, want ~%v", gotDiff, wantDiff)
	}
}
