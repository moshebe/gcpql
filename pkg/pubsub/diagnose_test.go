package pubsub

import (
	"testing"
	"time"
)

func TestDiagnose_OldestUnacked(t *testing.T) {
	tests := []struct {
		name      string
		oldest    float64
		wantSev   Severity
		wantTitle string
	}{
		{"critical: >1h", 3700, SeverityCritical, "Subscription Severely Backlogged"},
		{"warning: >10m", 700, SeverityWarning, "Consumer Falling Behind"},
		{"ok: <10m", 300, "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := DiagnoseData{
				Since: time.Hour,
				Sub:   SubMetrics{OldestUnackedSec: Stats{Current: tc.oldest}},
			}
			dr := Diagnose(data)
			if tc.wantSev == "" {
				for _, f := range dr.Findings {
					if f.Title == "Subscription Severely Backlogged" || f.Title == "Consumer Falling Behind" {
						t.Errorf("unexpected finding: %s", f.Title)
					}
				}
				return
			}
			found := findFinding(dr.Findings, tc.wantTitle)
			if found == nil {
				t.Fatalf("expected finding %q, got none (findings: %v)", tc.wantTitle, titles(dr.Findings))
			}
			if found.Severity != tc.wantSev {
				t.Errorf("severity: got %s, want %s", found.Severity, tc.wantSev)
			}
		})
	}
}

func TestDiagnose_DLQ(t *testing.T) {
	data := DiagnoseData{
		Since: time.Hour,
		Sub:   SubMetrics{DLQCount: 10},
	}
	dr := Diagnose(data)
	f := findFinding(dr.Findings, "Dead Letter Queue Has Messages")
	if f == nil {
		t.Fatal("expected DLQ finding, got none")
	}
	if f.Severity != SeverityCritical {
		t.Errorf("severity: got %s, want CRITICAL", f.Severity)
	}
}

func TestDiagnose_ExpiredAck(t *testing.T) {
	tests := []struct {
		name      string
		count     int64
		wantSev   Severity
		wantTitle string
	}{
		{"expired > 0 → warning", 50, SeverityWarning, "Consumers Missing Ack Deadline"},
		{"zero → no finding", 0, "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data := DiagnoseData{Since: time.Hour, Sub: SubMetrics{ExpiredAckCount: tc.count}}
			dr := Diagnose(data)
			if tc.wantSev == "" {
				for _, f := range dr.Findings {
					if f.Title == "Consumers Missing Ack Deadline" {
						t.Errorf("unexpected finding for count=0")
					}
				}
				return
			}
			f := findFinding(dr.Findings, tc.wantTitle)
			if f == nil {
				t.Fatalf("expected %q, got none", tc.wantTitle)
			}
			if f.Severity != tc.wantSev {
				t.Errorf("severity: got %s, want %s", f.Severity, tc.wantSev)
			}
		})
	}
}

func TestDiagnose_PushErrors(t *testing.T) {
	data := DiagnoseData{Since: time.Hour, Sub: SubMetrics{PushErrorRate: 0.05}}
	dr := Diagnose(data)
	f := findFinding(dr.Findings, "Push Delivery Errors")
	if f == nil {
		t.Fatal("expected push error finding, got none")
	}
	if f.Severity != SeverityWarning {
		t.Errorf("severity: got %s, want WARNING", f.Severity)
	}
}

func TestDiagnose_NoConsumer(t *testing.T) {
	data := DiagnoseData{
		Since: time.Hour,
		Sub:   SubMetrics{Backlog: Stats{Current: 500}, AckRatePerSec: 0},
	}
	dr := Diagnose(data)
	f := findFinding(dr.Findings, "No Active Consumer")
	if f == nil {
		t.Fatal("expected no-consumer finding, got none")
	}
}

func TestDiagnose_LargeMessages(t *testing.T) {
	data := DiagnoseData{
		Since: time.Hour,
		Topic: TopicMetrics{Available: true, AvgMessageSizeB: 600_000},
	}
	dr := Diagnose(data)
	f := findFinding(dr.Findings, "Large Messages Detected")
	if f == nil {
		t.Fatal("expected large-message finding, got none")
	}
}

// helpers

func findFinding(fs []Finding, title string) *Finding {
	for i := range fs {
		if fs[i].Title == title {
			return &fs[i]
		}
	}
	return nil
}

func titles(fs []Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Title)
	}
	return out
}
