package cloudsql

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestFormatJSON(t *testing.T) {
	result := &CheckResult{
		Instance:   "project:instance",
		Project:    "test-project",
		Timestamp:  time.Date(2026, 2, 17, 10, 0, 0, 0, time.UTC),
		TimeWindow: "24h",
		Resources: Resources{
			CPU: CPUMetrics{
				Utilization: Stats{
					Current: 0.45,
					P50:     0.42,
					P99:     0.78,
					Max:     0.89,
					Unit:    "percent",
				},
				ReservedCores: 4,
			},
		},
		Metadata: Metadata{
			MetricsCollected: 10,
		},
	}

	var buf bytes.Buffer
	err := FormatJSON(&buf, result)
	if err != nil {
		t.Fatalf("FormatJSON() error = %v", err)
	}

	// Verify valid JSON
	var parsed CheckResult
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("Invalid JSON output: %v", err)
	}

	// Verify key fields
	if parsed.Instance != "project:instance" {
		t.Errorf("Instance = %v, want project:instance", parsed.Instance)
	}

	if parsed.Resources.CPU.ReservedCores != 4 {
		t.Errorf("CPU cores = %v, want 4", parsed.Resources.CPU.ReservedCores)
	}
}
