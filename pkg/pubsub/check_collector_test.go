package pubsub

import (
	"testing"

	"github.com/moshebe/gcpql/pkg/monitoring"
)

func TestComputeStatus(t *testing.T) {
	tests := []struct {
		name       string
		snap       SubscriptionSnapshot
		wantStatus Severity
	}{
		{
			name:       "oldest unacked > 1h → CRITICAL",
			snap:       SubscriptionSnapshot{OldestUnackedSec: 3700},
			wantStatus: SeverityCritical,
		},
		{
			name:       "DLQ has messages → CRITICAL",
			snap:       SubscriptionSnapshot{DLQCount: 5},
			wantStatus: SeverityCritical,
		},
		{
			name:       "oldest unacked > 10m → WARNING",
			snap:       SubscriptionSnapshot{OldestUnackedSec: 700},
			wantStatus: SeverityWarning,
		},
		{
			name:       "backlog > 10k → WARNING",
			snap:       SubscriptionSnapshot{Backlog: 11000},
			wantStatus: SeverityWarning,
		},
		{
			name:       "expired ack deadlines → WARNING",
			snap:       SubscriptionSnapshot{ExpiredAckCount: 3},
			wantStatus: SeverityWarning,
		},
		{
			name:       "all zero → INFO (OK)",
			snap:       SubscriptionSnapshot{},
			wantStatus: SeverityInfo,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := computeStatus(tc.snap)
			if status != tc.wantStatus {
				t.Errorf("got %s, want %s", status, tc.wantStatus)
			}
		})
	}
}

func TestExtractSeriesLast(t *testing.T) {
	resp := &monitoring.QueryTimeSeriesResponse{
		TimeSeries: []interface{}{
			map[string]interface{}{
				"metric": map[string]interface{}{"subscription_id": "sub-a"},
				"values": []interface{}{
					[]interface{}{1.0, "100"},
					[]interface{}{2.0, "200"},
				},
			},
			map[string]interface{}{
				"metric": map[string]interface{}{"subscription_id": "sub-b"},
				"values": []interface{}{
					[]interface{}{1.0, "50"},
				},
			},
		},
	}

	got := extractSeriesLast(resp, "subscription_id")
	if got["sub-a"] != 200 {
		t.Errorf("sub-a: got %v, want 200", got["sub-a"])
	}
	if got["sub-b"] != 50 {
		t.Errorf("sub-b: got %v, want 50", got["sub-b"])
	}
}

func TestExtractSeriesSums(t *testing.T) {
	resp := &monitoring.QueryTimeSeriesResponse{
		TimeSeries: []interface{}{
			map[string]interface{}{
				"metric": map[string]interface{}{"subscription_id": "sub-a"},
				"values": []interface{}{
					[]interface{}{1.0, "10"},
					[]interface{}{2.0, "20"},
					[]interface{}{3.0, "30"},
				},
			},
		},
	}

	got := extractSeriesSums(resp, "subscription_id")
	if got["sub-a"] != 60 {
		t.Errorf("sub-a: got %v, want 60", got["sub-a"])
	}
}

func TestSortSnapshots(t *testing.T) {
	input := []SubscriptionSnapshot{
		{Name: "ok", Status: SeverityInfo, OldestUnackedSec: 0},
		{Name: "warn-low", Status: SeverityWarning, OldestUnackedSec: 100},
		{Name: "crit", Status: SeverityCritical, OldestUnackedSec: 4000},
		{Name: "warn-high", Status: SeverityWarning, OldestUnackedSec: 700},
	}
	sortSnapshots(input)

	wantOrder := []string{"crit", "warn-high", "warn-low", "ok"}
	for i, want := range wantOrder {
		if input[i].Name != want {
			t.Errorf("position %d: got %q, want %q", i, input[i].Name, want)
		}
	}
}
