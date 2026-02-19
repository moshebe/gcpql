package cloudsql

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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
