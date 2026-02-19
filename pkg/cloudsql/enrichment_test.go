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
