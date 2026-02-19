package cloudsql

import (
	"bytes"
	"encoding/json"
	"strings"
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

func TestFormatTable(t *testing.T) {
	result := &CheckResult{
		Instance:   "project:instance",
		Project:    "test-project",
		Region:     "us-central1",
		TimeWindow: "24h",
		InstanceSize: InstanceSize{
			VCPU:     4,
			MemoryGB: 16,
		},
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
			Memory: MemoryMetrics{
				Utilization: Stats{
					Current: 0.67,
					P50:     0.65,
					P99:     0.82,
					Max:     0.85,
					Unit:    "percent",
				},
			},
		},
		Connections: Connections{
			Count: Stats{
				Current: 45,
				P50:     42,
				P99:     89,
				Max:     95,
			},
			MaxConnections: 100,
		},
	}

	var buf bytes.Buffer
	err := FormatTable(&buf, result)
	if err != nil {
		t.Fatalf("FormatTable() error = %v", err)
	}

	output := buf.String()

	// Verify output contains key sections
	if !bytes.Contains([]byte(output), []byte("RESOURCES")) {
		t.Error("Table output missing RESOURCES section")
	}

	if !bytes.Contains([]byte(output), []byte("CONNECTIONS")) {
		t.Error("Table output missing CONNECTIONS section")
	}

	if !bytes.Contains([]byte(output), []byte("4 vCPU")) {
		t.Error("Table output missing instance size")
	}
}

func TestFormatTable_InstanceConfig(t *testing.T) {
	result := &CheckResult{}
	result.InstanceConfig = InstanceConfig{
		State:            "RUNNABLE",
		AvailabilityType: "REGIONAL",
		BackupEnabled:    true,
		BackupStartTime:  "03:00",
		PITREnabled:      true,
		StorageType:      "PD_SSD",
		Labels:           map[string]string{"env": "prod"},
	}

	var buf strings.Builder
	if err := FormatTable(&buf, result); err != nil {
		t.Fatalf("FormatTable: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "INSTANCE CONFIG") {
		t.Error("missing INSTANCE CONFIG section")
	}
	if !strings.Contains(out, "REGIONAL") {
		t.Error("missing REGIONAL in output")
	}
	if !strings.Contains(out, "env=prod") {
		t.Error("missing label in output")
	}
}

func TestFormatTable_Recommendations(t *testing.T) {
	result := &CheckResult{}
	result.Recommendations = Recommendations{
		Available: true,
		Items: []Recommendation{
			{Description: "Reduce RAM", Impact: "HIGH", State: "ACTIVE"},
		},
	}

	var buf strings.Builder
	if err := FormatTable(&buf, result); err != nil {
		t.Fatalf("FormatTable: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "RECOMMENDATIONS") {
		t.Error("missing RECOMMENDATIONS section")
	}
	if !strings.Contains(out, "Reduce RAM") {
		t.Error("missing recommendation text")
	}
}

func TestFormatTable_XIDInDerivedInsights(t *testing.T) {
	result := &CheckResult{}
	result.DerivedInsights.XIDWraparoundRisk = 72.5

	var buf strings.Builder
	if err := FormatTable(&buf, result); err != nil {
		t.Fatalf("FormatTable: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "XID Wraparound Risk") {
		t.Error("missing XID Wraparound Risk in DERIVED INSIGHTS")
	}
	if !strings.Contains(out, "72.5%") {
		t.Error("missing XID risk percentage")
	}
	// DATABASE HEALTH section should not appear when there are no deadlocks/vacuums
	if strings.Contains(out, "DATABASE HEALTH") {
		t.Error("DATABASE HEALTH should be hidden when nothing actionable")
	}
}

func TestFormatTable_DBHealthShownWhenDeadlocks(t *testing.T) {
	result := &CheckResult{}
	result.DBHealth.DeadlockCount = 5

	var buf strings.Builder
	if err := FormatTable(&buf, result); err != nil {
		t.Fatalf("FormatTable: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "DATABASE HEALTH") {
		t.Error("DATABASE HEALTH should appear when deadlocks > 0")
	}
	if !strings.Contains(out, "Deadlocks") {
		t.Error("deadlock row should appear")
	}
}

func TestFormatTable_QueryInsightsOmittedWhenEmpty(t *testing.T) {
	result := &CheckResult{}

	var buf strings.Builder
	if err := FormatTable(&buf, result); err != nil {
		t.Fatalf("FormatTable: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "QUERY INSIGHTS") {
		t.Error("QUERY INSIGHTS section should be omitted when not available")
	}
}

func TestFormatTable_QueryInsightsWithData(t *testing.T) {
	result := &CheckResult{}
	result.QueryInsights = QueryInsights{
		Available: true,
		TopQueries: []TopQuery{
			{Database: "orders", User: "app", ClientAddr: "10.0.0.1", SampleCount: 5, AvgLatencyMS: 45.0, TotalTimeMS: 225.0},
		},
	}

	var buf strings.Builder
	if err := FormatTable(&buf, result); err != nil {
		t.Fatalf("FormatTable: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "QUERY INSIGHTS") {
		t.Error("missing QUERY INSIGHTS section")
	}
	if !strings.Contains(out, "orders") {
		t.Error("missing database name")
	}
	if !strings.Contains(out, "app") {
		t.Error("missing user")
	}
}
