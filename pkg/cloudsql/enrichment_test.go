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

	recs, err := fetchRecommendations(context.Background(), srv.Client(), "myproject", "us-central1", srv.URL)
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

	recs, err := fetchRecommendations(context.Background(), srv.Client(), "myproject", "us-central1", srv.URL)
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

	recs, err := fetchRecommendations(context.Background(), srv.Client(), "myproject", "us-central1", srv.URL)
	if err != nil {
		t.Fatalf("should not error on 404, got: %v", err)
	}
	if recs.Available {
		t.Error("available: want false on 404")
	}
}

func TestFetchRecommendations_ErrorOnOtherStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := fetchRecommendations(context.Background(), srv.Client(), "myproject", "us-central1", srv.URL)
	if err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}

func TestParseQueryInsightsResponse_SortsByTotalTime(t *testing.T) {
	raw := []byte(`{
        "data": {
            "result": [
                {
                    "metric": {"query_hash": "abc123", "query_string": "SELECT * FROM orders"},
                    "values": [[1700000000, "50000"], [1700000060, "60000"]]
                },
                {
                    "metric": {"query_hash": "def456", "query_string": "UPDATE users SET last_seen"},
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
	// abc123 has 110000µs total, def456 has 45000µs — abc123 should be first
	if qi.TopQueries[0].QueryHash != "abc123" {
		t.Errorf("top query hash: got %q want abc123", qi.TopQueries[0].QueryHash)
	}
	if qi.TopQueries[0].QueryText != "SELECT * FROM orders" {
		t.Errorf("query text: got %q", qi.TopQueries[0].QueryText)
	}
	// TotalTimeMS = 110000µs / 1000 = 110ms
	if qi.TopQueries[0].TotalTimeMS != 110.0 {
		t.Errorf("total time ms: got %.1f want 110.0", qi.TopQueries[0].TotalTimeMS)
	}
	// AvgLatencyMS = 110ms / 2 values = 55ms
	if qi.TopQueries[0].AvgLatencyMS != 55.0 {
		t.Errorf("avg latency ms: got %.1f want 55.0", qi.TopQueries[0].AvgLatencyMS)
	}
	if qi.TopQueries[0].CallCount != 2 {
		t.Errorf("call count: got %d want 2", qi.TopQueries[0].CallCount)
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
                {"metric": {"query_hash": "a"}, "values": [[1, "30000"]]},
                {"metric": {"query_hash": "b"}, "values": [[1, "20000"]]},
                {"metric": {"query_hash": "c"}, "values": [[1, "10000"]]}
            ]
        }
    }`)
	qi := parseQueryInsightsResponse(raw, 2)
	if len(qi.TopQueries) != 2 {
		t.Errorf("top_queries: got %d want 2", len(qi.TopQueries))
	}
	// Should have the top 2 by total time: a (30ms) and b (20ms)
	if qi.TopQueries[0].QueryHash != "a" {
		t.Errorf("first: got %q want a", qi.TopQueries[0].QueryHash)
	}
	if qi.TopQueries[1].QueryHash != "b" {
		t.Errorf("second: got %q want b", qi.TopQueries[1].QueryHash)
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
