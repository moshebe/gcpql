package bigquery

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestFormatJSON(t *testing.T) {
	result := &CheckResult{
		Project:   "test-project",
		Timestamp: time.Now(),
		Slots: SlotMetrics{
			Allocated:   100,
			Current:     75,
			Utilization: 75.0,
		},
		Cost: CostMetrics{
			StorageGB: 1000,
		},
	}

	var buf bytes.Buffer
	err := FormatJSON(&buf, result)
	if err != nil {
		t.Fatalf("FormatJSON failed: %v", err)
	}

	// Verify valid JSON
	var decoded CheckResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}

	if decoded.Project != result.Project {
		t.Errorf("Project mismatch: got %s, want %s", decoded.Project, result.Project)
	}
}

func TestFormatCheckTable(t *testing.T) {
	result := &CheckResult{
		Project:   "test-project",
		Dataset:   "test-dataset",
		Timestamp: time.Now(),
		Slots: SlotMetrics{
			Allocated:        100,
			Current:          85,
			Utilization:      85.0,
			Peak:             95,
			QueriesInFlight:  5,
			QueriesQueued:    2,
		},
		Cost: CostMetrics{
			StorageGB:          1000.5,
			StorageCostDaily:   20.01,
			BytesScannedTotal:  1024 * 1024 * 1024 * 10, // 10 GB
			EstimatedCost:      5.0,
		},
		Jobs: JobsSummary{
			TotalJobs:    100,
			FailedJobs:   3,
			CacheHits:    20,
			CacheHitRate: 20.0,
			TotalBytes:   1024 * 1024 * 1024 * 10,
			TotalCost:    5.0,
		},
		TopQueries: []ExpensiveQuery{
			{
				Query:           "SELECT * FROM table WHERE condition = true",
				BytesProcessed:  1024 * 1024 * 1024,
				EstimatedCost:   0.5,
				UserEmail:       "user@example.com",
			},
		},
		Metadata: Metadata{
			MetricsCollected:     10,
			MetricsNoData:        2,
			CollectionDurationMS: 1500,
		},
	}

	var buf bytes.Buffer
	err := FormatCheckTable(&buf, result)
	if err != nil {
		t.Fatalf("FormatCheckTable failed: %v", err)
	}

	output := buf.String()

	// Verify key sections present
	sections := []string{
		"Project: test-project",
		"Dataset: test-dataset",
		"SLOT UTILIZATION",
		"COST INDICATORS",
		"QUERY SUMMARY",
		"TOP EXPENSIVE QUERIES",
		"Metrics:",
	}

	for _, section := range sections {
		if !strings.Contains(output, section) {
			t.Errorf("Output missing section: %s", section)
		}
	}

	// Verify values present
	if !strings.Contains(output, "100 slots") {
		t.Error("Missing allocated slots")
	}
	if !strings.Contains(output, "85.0%") {
		t.Error("Missing utilization percentage")
	}
	if !strings.Contains(output, "5 running") {
		t.Error("Missing queries in flight")
	}
	if !strings.Contains(output, "2 queued") {
		t.Error("Missing queued queries")
	}
	if !strings.Contains(output, "1000.5 GB") {
		t.Error("Missing storage size")
	}
	if !strings.Contains(output, "user@example.com") {
		t.Error("Missing user email")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1536 * 1024 * 1024, "1.5 GB"},
	}

	for _, tt := range tests {
		got := formatBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("formatBytes(%d) = %s, want %s", tt.bytes, got, tt.want)
		}
	}
}

func TestNormalizeQuery(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"SELECT *\nFROM foo", "SELECT * FROM foo"},
		{"  SELECT\t*\t\nFROM\n  bar  ", "SELECT * FROM bar"},
		{"single line", "single line"},
	}
	for _, tt := range tests {
		got := normalizeQuery(tt.input)
		if got != tt.want {
			t.Errorf("normalizeQuery(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatCost(t *testing.T) {
	tests := []struct {
		cost float64
		want string
	}{
		{0, "$0.00"},
		{0.000001, "$0.000001"},
		{0.005, "$0.0050"},
		{0.5, "$0.5000"},
		{1.5, "$1.50"},
		{100.0, "$100.00"},
	}
	for _, tt := range tests {
		got := formatCost(tt.cost)
		if got != tt.want {
			t.Errorf("formatCost(%v) = %q, want %q", tt.cost, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"short", 10, "short"},
		{"exactly ten!", 12, "exactly ten!"},
		{"this is a very long string", 10, "this is..."},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.max)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
	}
}

func TestGetSlotStatus(t *testing.T) {
	tests := []struct {
		utilization float64
		want        string
	}{
		{50.0, "🟢"},
		{69.9, "🟢"},
		{70.0, "🟡"},
		{89.9, "🟡"},
		{90.0, "🔴"},
		{100.0, "🔴"},
	}

	for _, tt := range tests {
		got := getSlotStatus(tt.utilization)
		if got != tt.want {
			t.Errorf("getSlotStatus(%.1f) = %s, want %s", tt.utilization, got, tt.want)
		}
	}
}

func TestGetStorageCostStatus(t *testing.T) {
	tests := []struct {
		daily float64
		want  string
	}{
		{0.0, "🟢"},
		{49.9, "🟢"},
		{50.0, "🟡"},
		{199.9, "🟡"},
		{200.0, "🔴"},
	}

	for _, tt := range tests {
		got := getStorageCostStatus(tt.daily)
		if got != tt.want {
			t.Errorf("getStorageCostStatus(%.1f) = %s, want %s", tt.daily, got, tt.want)
		}
	}
}

func TestGetQueryCostStatus(t *testing.T) {
	tests := []struct {
		total float64
		want  string
	}{
		{0.0, "🟢"},
		{99.9, "🟢"},
		{100.0, "🟡"},
		{499.9, "🟡"},
		{500.0, "🔴"},
	}

	for _, tt := range tests {
		got := getQueryCostStatus(tt.total)
		if got != tt.want {
			t.Errorf("getQueryCostStatus(%.1f) = %s, want %s", tt.total, got, tt.want)
		}
	}
}
