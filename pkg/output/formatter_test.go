package output

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func TestFormatJSON(t *testing.T) {
	result := QueryResult{
		Query:   "fetch cloudsql_database | metric 'cpu'",
		Project: "test-project",
		TimeRange: TimeRange{
			Start: time.Date(2026, 2, 16, 11, 0, 0, 0, time.UTC),
			End:   time.Date(2026, 2, 16, 12, 0, 0, 0, time.UTC),
		},
		TimeSeries: []interface{}{
			map[string]string{"test": "data"},
		},
	}

	var buf bytes.Buffer
	err := FormatJSON(&buf, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Verify fields
	if parsed["query"] != result.Query {
		t.Errorf("expected query '%s', got '%v'", result.Query, parsed["query"])
	}
	if parsed["project"] != result.Project {
		t.Errorf("expected project '%s', got '%v'", result.Project, parsed["project"])
	}
}

func TestFormatError(t *testing.T) {
	errResult := ErrorResult{
		Error:   "PERMISSION_DENIED",
		Message: "Permission denied",
		Query:   "fetch cloudsql",
	}

	var buf bytes.Buffer
	err := FormatError(&buf, &errResult)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify valid JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if parsed["error"] != errResult.Error {
		t.Errorf("expected error '%s', got '%v'", errResult.Error, parsed["error"])
	}
}
