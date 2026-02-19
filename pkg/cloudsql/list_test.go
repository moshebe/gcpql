package cloudsql

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gcp-metrics/gcp-metrics/pkg/monitoring"
)

func TestParseTier(t *testing.T) {
	tests := []struct {
		tier      string
		wantVCPU  int
		wantMemGB float64
	}{
		{"db-custom-4-15360", 4, 15.0},
		{"db-custom-2-7680", 2, 7.5},
		{"db-custom-1-3840", 1, 3.75},
		{"db-custom-4-15360-hc", 0, 0},
		{"db-n1-standard-4", 0, 0},
		{"", 0, 0},
		{"unknown", 0, 0},
	}
	for _, tc := range tests {
		vcpu, memGB := parseTier(tc.tier)
		if vcpu != tc.wantVCPU {
			t.Errorf("parseTier(%q) vcpu = %d, want %d", tc.tier, vcpu, tc.wantVCPU)
		}
		if memGB != tc.wantMemGB {
			t.Errorf("parseTier(%q) memGB = %f, want %f", tc.tier, memGB, tc.wantMemGB)
		}
	}
}

func TestFetchAllInstancesFromURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
            "items": [
                {
                    "name": "prod-db",
                    "state": "RUNNABLE",
                    "databaseVersion": "POSTGRES_15",
                    "region": "us-central1",
                    "settings": {"tier": "db-custom-4-15360"}
                },
                {
                    "name": "staging-db",
                    "state": "RUNNABLE",
                    "databaseVersion": "POSTGRES_14",
                    "region": "us-east1",
                    "settings": {"tier": "db-custom-2-7680"}
                }
            ]
        }`))
	}))
	defer srv.Close()

	records, err := fetchAllInstancesFromURL(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2", len(records))
	}
	if records[0].name != "prod-db" {
		t.Errorf("records[0].name = %q, want prod-db", records[0].name)
	}
	if records[0].vcpu != 4 {
		t.Errorf("records[0].vcpu = %d, want 4", records[0].vcpu)
	}
	if records[0].memoryGB != 15.0 {
		t.Errorf("records[0].memoryGB = %f, want 15.0", records[0].memoryGB)
	}
	if records[1].name != "staging-db" {
		t.Errorf("records[1].name = %q, want staging-db", records[1].name)
	}
	if records[1].vcpu != 2 {
		t.Errorf("records[1].vcpu = %d, want 2", records[1].vcpu)
	}
}

func TestFetchAllInstancesFromURL_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	records, err := fetchAllInstancesFromURL(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("got %d records, want 0", len(records))
	}
}

func TestFetchAllInstancesFromURL_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "project not found"}`))
	}))
	defer srv.Close()

	_, err := fetchAllInstancesFromURL(context.Background(), srv.Client(), srv.URL)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestFetchAllInstancesFromURL_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "internal error"}`))
	}))
	defer srv.Close()

	_, err := fetchAllInstancesFromURL(context.Background(), srv.Client(), srv.URL)
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}
}

func TestNormalizeDBID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"myproject:us-central1:myinstance", "myproject:myinstance"},
		{"myproject:myinstance", "myproject:myinstance"},
		{"single", ""},  // 1-part: unrecognized, returns empty
		{"a:b:c:d", ""},  // 4-part: unrecognized, returns empty
	}
	for _, tc := range tests {
		got := normalizeDBID(tc.input)
		if got != tc.want {
			t.Errorf("normalizeDBID(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFetchBulkUtilization(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
            "status": "success",
            "data": {
                "resultType": "matrix",
                "result": [
                    {
                        "metric": {
                            "__name__": "cloudsql.googleapis.com/database/cpu/utilization",
                            "database_id": "myproject:us-central1:prod-db"
                        },
                        "values": [[1700000000, "0.42"], [1700000060, "0.45"]]
                    },
                    {
                        "metric": {
                            "__name__": "cloudsql.googleapis.com/database/cpu/utilization",
                            "database_id": "myproject:us-east1:staging-db"
                        },
                        "values": [[1700000000, "0.08"]]
                    }
                ]
            }
        }`))
	}))
	defer srv.Close()

	monClient := monitoring.NewClientForTesting(srv.Client(), srv.URL)
	result, err := fetchBulkUtilization(context.Background(), monClient, "myproject", 5*time.Minute,
		"cloudsql.googleapis.com/database/cpu/utilization")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3-part database_id should be normalized to 2-part; last value taken
	if v, ok := result["myproject:prod-db"]; !ok {
		t.Error("missing myproject:prod-db")
	} else if v != 0.45 {
		t.Errorf("myproject:prod-db = %f, want 0.45", v)
	}
	if v, ok := result["myproject:staging-db"]; !ok {
		t.Error("missing myproject:staging-db")
	} else if v != 0.08 {
		t.Errorf("myproject:staging-db = %f, want 0.08", v)
	}
}

func TestFetchBulkUtilization_EmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
	}))
	defer srv.Close()

	monClient := monitoring.NewClientForTesting(srv.Client(), srv.URL)
	result, err := fetchBulkUtilization(context.Background(), monClient, "myproject", 5*time.Minute,
		"cloudsql.googleapis.com/database/cpu/utilization")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("got %d entries, want 0", len(result))
	}
}

func TestFetchBulkUtilization_EmptyValues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"status": "success",
			"data": {
				"resultType": "matrix",
				"result": [
					{
						"metric": {"database_id": "myproject:us-central1:prod-db"},
						"values": []
					}
				]
			}
		}`))
	}))
	defer srv.Close()

	monClient := monitoring.NewClientForTesting(srv.Client(), srv.URL)
	result, err := fetchBulkUtilization(context.Background(), monClient, "myproject", 5*time.Minute,
		"cloudsql.googleapis.com/database/cpu/utilization")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Series with empty values should be skipped
	if _, ok := result["myproject:prod-db"]; ok {
		t.Error("entry with empty values should not appear in result")
	}
}

func TestListInstances(t *testing.T) {
	// Admin API server
	adminSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
            "items": [
                {
                    "name": "prod-db",
                    "state": "RUNNABLE",
                    "databaseVersion": "POSTGRES_15",
                    "region": "us-central1",
                    "settings": {"tier": "db-custom-4-15360"}
                }
            ]
        }`))
	}))
	defer adminSrv.Close()

	// Monitoring server (same mock for both cpu + mem queries)
	monSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
            "status": "success",
            "data": {
                "resultType": "matrix",
                "result": [
                    {
                        "metric": {"database_id": "myproject:us-central1:prod-db"},
                        "values": [[1700000000, "0.42"]]
                    }
                ]
            }
        }`))
	}))
	defer monSrv.Close()

	monClient := monitoring.NewClientForTesting(monSrv.Client(), monSrv.URL)

	result, err := listInstancesWithURL(context.Background(), adminSrv.Client(), adminSrv.URL,
		monClient, "myproject", 5*time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.Instance != "myproject:prod-db" {
		t.Errorf("Instance = %q, want myproject:prod-db", item.Instance)
	}
	if item.VCPU != 4 {
		t.Errorf("VCPU = %d, want 4", item.VCPU)
	}
	if item.MemoryGB != 15.0 {
		t.Errorf("MemoryGB = %f, want 15.0", item.MemoryGB)
	}
	if item.CPUPct == nil {
		t.Error("CPUPct is nil, want value")
	} else if *item.CPUPct != 42.0 {
		t.Errorf("CPUPct = %f, want 42.0", *item.CPUPct)
	}
	if item.MemPct == nil {
		t.Error("MemPct is nil, want value")
	} else if *item.MemPct != 42.0 {
		t.Errorf("MemPct = %f, want 42.0", *item.MemPct)
	}

	if item.State != "RUNNABLE" {
		t.Errorf("State = %q, want RUNNABLE", item.State)
	}
	if item.DBVersion != "POSTGRES_15" {
		t.Errorf("DBVersion = %q, want POSTGRES_15", item.DBVersion)
	}
	if item.Region != "us-central1" {
		t.Errorf("Region = %q, want us-central1", item.Region)
	}
}

func TestListInstances_MonitoringError(t *testing.T) {
	adminSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
            "items": [
                {
                    "name": "prod-db",
                    "state": "RUNNABLE",
                    "databaseVersion": "POSTGRES_15",
                    "region": "us-central1",
                    "settings": {"tier": "db-custom-4-15360"}
                }
            ]
        }`))
	}))
	defer adminSrv.Close()

	// Monitoring returns 500 — should not cause ListInstances to fail
	monSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer monSrv.Close()

	monClient := monitoring.NewClientForTesting(monSrv.Client(), monSrv.URL)

	result, err := listInstancesWithURL(context.Background(), adminSrv.Client(), adminSrv.URL,
		monClient, "myproject", 5*time.Minute)
	if err != nil {
		t.Fatalf("expected no error despite monitoring failure, got: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(result.Items))
	}
	item := result.Items[0]
	if item.CPUPct != nil {
		t.Errorf("CPUPct should be nil when monitoring fails, got %f", *item.CPUPct)
	}
	if item.MemPct != nil {
		t.Errorf("MemPct should be nil when monitoring fails, got %f", *item.MemPct)
	}
}
