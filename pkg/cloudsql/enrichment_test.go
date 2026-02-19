package cloudsql

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchRecommendations_ReturnsItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
            "recommendations": [
                {
                    "description": "Reduce allocated RAM",
                    "primaryImpact": {"category": "PERFORMANCE"},
                    "priority": "P2",
                    "stateInfo": {"state": "ACTIVE"}
                }
            ]
        }`))
	}))
	defer srv.Close()

	recs, err := fetchRecommendations(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !recs.Available {
		t.Error("available: want true")
	}
	if len(recs.Items) != 1 {
		t.Fatalf("items: got %d want 1", len(recs.Items))
	}
	if recs.Items[0].Description != "Reduce allocated RAM" {
		t.Errorf("description: got %q", recs.Items[0].Description)
	}
	if recs.Items[0].Impact != "MEDIUM" { // P2 maps to MEDIUM
		t.Errorf("impact: got %q want MEDIUM", recs.Items[0].Impact)
	}
	if recs.Items[0].State != "ACTIVE" {
		t.Errorf("state: got %q want ACTIVE", recs.Items[0].State)
	}
}

func TestFetchRecommendations_Graceful403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	recs, err := fetchRecommendations(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("should not error on 403, got: %v", err)
	}
	if recs.Available {
		t.Error("available: want false on 403")
	}
}

func TestFetchRecommendations_Graceful404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	recs, err := fetchRecommendations(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("should not error on 404, got: %v", err)
	}
	if recs.Available {
		t.Error("available: want false on 404")
	}
}

func TestFetchRecommendations_GracefulOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	recs, err := fetchRecommendations(context.Background(), srv.Client(), srv.URL)
	if err != nil {
		t.Fatalf("should not error on 500, got: %v", err)
	}
	if recs.Available {
		t.Error("available: want false on 500")
	}
}

func TestParseQueryInsightsResponse_SortsByTotalTime(t *testing.T) {
	raw := []byte(`{
        "data": {
            "result": [
                {
                    "metric": {"database": "orders_db", "user": "app", "client_addr": "10.0.0.1"},
                    "values": [[1700000000, "50000"], [1700000060, "60000"]]
                },
                {
                    "metric": {"database": "users_db", "user": "writer", "client_addr": "10.0.0.2"},
                    "values": [[1700000000, "20000"], [1700000060, "25000"]]
                }
            ]
        }
    }`)

	qi := parseQueryInsightsResponse(raw, 5)
	if !qi.Available {
		t.Error("available: want true")
	}
	if len(qi.TopQueries) != 2 {
		t.Fatalf("top_queries: got %d want 2", len(qi.TopQueries))
	}
	// orders_db delta=60000-50000=10000µs=10ms, users_db delta=25000-20000=5000µs=5ms
	// orders_db should be first (larger delta)
	if qi.TopQueries[0].Database != "orders_db" {
		t.Errorf("top database: got %q want orders_db", qi.TopQueries[0].Database)
	}
	if qi.TopQueries[0].User != "app" {
		t.Errorf("user: got %q want app", qi.TopQueries[0].User)
	}
	// TotalTimeMS = delta(10000µs) / 1000 = 10ms
	if qi.TopQueries[0].TotalTimeMS != 10.0 {
		t.Errorf("total time ms: got %.1f want 10.0", qi.TopQueries[0].TotalTimeMS)
	}
	// AvgLatencyMS = 10ms / (2-1) intervals = 10ms
	if qi.TopQueries[0].AvgLatencyMS != 10.0 {
		t.Errorf("avg latency ms: got %.1f want 10.0", qi.TopQueries[0].AvgLatencyMS)
	}
	if qi.TopQueries[0].SampleCount != 2 {
		t.Errorf("sample count: got %d want 2", qi.TopQueries[0].SampleCount)
	}
}

func TestParseQueryInsightsResponse_EmptyReturnsUnavailable(t *testing.T) {
	raw := []byte(`{"data": {"result": []}}`)
	qi := parseQueryInsightsResponse(raw, 5)
	if qi.Available {
		t.Error("available: want false for empty result")
	}
}

func TestParseQueryInsightsResponse_TopNTruncates(t *testing.T) {
	raw := []byte(`{
        "data": {
            "result": [
                {"metric": {"database": "db1", "user": "a"}, "values": [[1, "10000"], [2, "40000"]]},
                {"metric": {"database": "db1", "user": "b"}, "values": [[1, "10000"], [2, "30000"]]},
                {"metric": {"database": "db1", "user": "c"}, "values": [[1, "10000"], [2, "20000"]]}
            ]
        }
    }`)
	qi := parseQueryInsightsResponse(raw, 2)
	if len(qi.TopQueries) != 2 {
		t.Errorf("top_queries: got %d want 2", len(qi.TopQueries))
	}
	// Should have the top 2 by total time: a (30ms) and b (20ms)
	if qi.TopQueries[0].User != "a" {
		t.Errorf("first: got %q want a", qi.TopQueries[0].User)
	}
	if qi.TopQueries[1].User != "b" {
		t.Errorf("second: got %q want b", qi.TopQueries[1].User)
	}
}

func TestPriorityToImpact(t *testing.T) {
	cases := []struct {
		priority string
		want     string
	}{
		{"P1", "HIGH"},
		{"P2", "MEDIUM"},
		{"P3", "LOW"},
		{"P4", "LOW"},
		{"", "UNKNOWN"},
	}
	for _, c := range cases {
		got := priorityToImpact(c.priority)
		if got != c.want {
			t.Errorf("priorityToImpact(%q) = %q, want %q", c.priority, got, c.want)
		}
	}
}
