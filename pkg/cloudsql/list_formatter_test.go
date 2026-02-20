package cloudsql

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func makeTestListResult() *ListResult {
	cpu := 42.0
	mem := 61.0
	return &ListResult{
		Project:   "myproject",
		Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Items: []ListItem{
			{
				Instance:  "myproject:prod-db",
				State:     "RUNNABLE",
				DBVersion: "POSTGRES_15",
				Region:    "us-central1",
				VCPU:      4,
				MemoryGB:  15.0,
				CPUPct:    &cpu,
				MemPct:    &mem,
			},
			{
				Instance:  "myproject:old-db",
				State:     "STOPPED",
				DBVersion: "MYSQL_8_0",
				Region:    "us-west1",
				VCPU:      1,
				MemoryGB:  3.75,
				CPUPct:    nil,
				MemPct:    nil,
			},
		},
	}
}

func TestFormatListJSON(t *testing.T) {
	var buf bytes.Buffer
	result := makeTestListResult()
	if err := FormatListJSON(&buf, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}
	if out["project"] != "myproject" {
		t.Errorf("project = %v, want myproject", out["project"])
	}
	items := out["items"].([]interface{})
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2", len(items))
	}
	first := items[0].(map[string]interface{})
	if first["instance"] != "myproject:prod-db" {
		t.Errorf("items[0].instance = %v", first["instance"])
	}
	// nil CPUPct/MemPct should be omitted (omitempty)
	second := items[1].(map[string]interface{})
	if _, ok := second["cpu_pct"]; ok {
		t.Error("cpu_pct should be omitted for nil value")
	}
	if _, ok := second["mem_pct"]; ok {
		t.Error("mem_pct should be omitted for nil value")
	}
}

func TestFormatListTable(t *testing.T) {
	var buf bytes.Buffer
	result := makeTestListResult()
	if err := FormatListTable(&buf, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "myproject:prod-db") {
		t.Error("output missing prod-db instance")
	}
	if !strings.Contains(out, "myproject:old-db") {
		t.Error("output missing old-db instance")
	}
	if !strings.Contains(out, "POSTGRES_15") {
		t.Error("output missing db version")
	}
	if !strings.Contains(out, "STOPPED") {
		t.Error("output missing STOPPED state")
	}
	if !strings.Contains(out, "42%") {
		t.Error("output missing 42% CPU")
	}
	// old-db has nil CPU/mem — its table row should contain "-" placeholders
	found := false
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "old-db") && strings.Contains(line, "-") {
			found = true
			break
		}
	}
	if !found {
		t.Error("old-db row should contain - for nil CPU/mem metrics")
	}
}
