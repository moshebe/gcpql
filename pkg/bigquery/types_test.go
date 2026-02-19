package bigquery

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCheckResult_JSON(t *testing.T) {
	result := CheckResult{
		Project:   "my-project",
		Dataset:   "",
		Timestamp: time.Date(2026, 2, 19, 10, 0, 0, 0, time.UTC),
		Slots: SlotMetrics{
			Allocated:       1000,
			Current:         850,
			Peak:            950,
			Utilization:     85.0,
			QueriesInFlight: 12,
			QueriesQueued:   3,
		},
		Cost: CostMetrics{
			StorageGB:         1200.5,
			StorageCostDaily:  25.00,
			BytesScannedTotal: 450000000000,
			EstimatedCost:     2.25,
		},
		TopQueries: []ExpensiveQuery{
			{
				JobID:           "job_123",
				UserEmail:       "user@example.com",
				Query:           "SELECT * FROM logs",
				BytesProcessed:  820000000000,
				EstimatedCost:   4.10,
				DurationSeconds: 45.0,
			},
		},
		Metadata: Metadata{
			MetricsCollected:     5,
			MetricsNoData:        0,
			CollectionDurationMS: 1200,
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded CheckResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Slots.Current != 850 {
		t.Errorf("Expected Current=850, got %d", decoded.Slots.Current)
	}
}

func TestCheckResult_OmitEmpty(t *testing.T) {
	result := CheckResult{
		Project:   "test-project",
		Timestamp: time.Date(2026, 2, 19, 10, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	str := string(data)

	if strings.Contains(str, "dataset") {
		t.Error("Expected dataset to be omitted")
	}
	if strings.Contains(str, "top_queries") {
		t.Error("Expected top_queries to be omitted")
	}
}
