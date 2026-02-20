package monitoring

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{30 * time.Minute, "30m"},
		{1 * time.Hour, "1h"},
		{3 * time.Hour, "3h"},
		{24 * time.Hour, "1d"},
		{48 * time.Hour, "2d"},
		{7 * 24 * time.Hour, "7d"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestHasRangeSelector(t *testing.T) {
	tests := []struct {
		query    string
		expected bool
	}{
		{"metric_name[5m]", true},
		{"metric_name{label=\"value\"}[1h]", true},
		{"metric_name", false},
		{"metric_name{label=\"value\"}", false},
		{"rate(metric[5m])", true},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := hasRangeSelector(tt.query)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestInjectRangeSelector(t *testing.T) {
	tests := []struct {
		query    string
		duration string
		expected string
	}{
		{"metric_name", "5m", "metric_name[5m]"},
		{"metric_name{label=\"value\"}", "1h", "metric_name{label=\"value\"}[1h]"},
		{"cloudsql_database:database/cpu/utilization", "5m", "cloudsql_database:database/cpu/utilization[5m]"},
		{"{__name__=\"cloudsql.googleapis.com/database/cpu/utilization\"}", "5m", "{__name__=\"cloudsql.googleapis.com/database/cpu/utilization\"}[5m]"},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := injectRangeSelector(tt.query, tt.duration)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestNormalizeMetricQuery(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "simple metric with dots",
			query:    "cloudsql.googleapis.com/database/cpu/utilization",
			expected: `{__name__="cloudsql.googleapis.com/database/cpu/utilization"}`,
		},
		{
			name:     "simple metric with slashes",
			query:    "compute/instance/cpu/utilization",
			expected: `{__name__="compute/instance/cpu/utilization"}`,
		},
		{
			name:     "already wrapped with __name__",
			query:    `{__name__="cloudsql.googleapis.com/database/cpu/utilization"}`,
			expected: `{__name__="cloudsql.googleapis.com/database/cpu/utilization"}`,
		},
		{
			name:     "with label selectors",
			query:    `{__name__="cloudsql.googleapis.com/database/cpu/utilization",database_id="foo"}`,
			expected: `{__name__="cloudsql.googleapis.com/database/cpu/utilization",database_id="foo"}`,
		},
		{
			name:     "simple metric without dots or slashes",
			query:    "metric_name",
			expected: "metric_name",
		},
		{
			name:     "query with trailing spaces",
			query:    "  cloudsql.googleapis.com/database/cpu/utilization  ",
			expected: `{__name__="cloudsql.googleapis.com/database/cpu/utilization"}`,
		},
		{
			name:     "complex PromQL expression",
			query:    `rate({__name__="metric"}[5m])`,
			expected: `rate({__name__="metric"}[5m])`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeMetricQuery(tt.query)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}
