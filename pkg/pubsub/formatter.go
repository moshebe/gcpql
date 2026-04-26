package pubsub

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/jedib0t/go-pretty/v6/table"
)

// FormatCheckJSON writes a CheckResult as indented JSON.
func FormatCheckJSON(w io.Writer, r *CheckResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// FormatCheckTable writes a CheckResult as a human-readable table.
func FormatCheckTable(w io.Writer, r *CheckResult, top int) error {
	fmt.Fprintf(w, "Project:   %s\n", r.Project)
	fmt.Fprintf(w, "Timestamp: %s\n\n", r.Timestamp.Format("2006-01-02 15:04:05 UTC"))

	if len(r.Subscriptions) == 0 {
		fmt.Fprintln(w, "No subscriptions found.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.SetTitle("SUBSCRIPTION HEALTH")
	t.AppendHeader(table.Row{"Subscription", "Backlog", "Oldest Unacked", "Exp. Ack", "DLQ", "Status"})

	for _, s := range r.Subscriptions {
		statusStr := string(s.Status)
		t.AppendRow(table.Row{
			s.Name,
			s.Backlog,
			fmtAge(s.OldestUnackedSec),
			s.ExpiredAckCount,
			s.DLQCount,
			statusStr,
		})
	}
	t.Render()

	// Top offenders section
	if top > 0 && len(r.Subscriptions) > 0 {
		worst := r.Subscriptions
		if len(worst) > top {
			worst = worst[:top]
		}
		hasIssues := false
		for _, s := range worst {
			if s.Status != SeverityInfo {
				hasIssues = true
				break
			}
		}
		if hasIssues {
			fmt.Fprintf(w, "\nTop offenders (worst %d):\n", len(worst))
			for i, s := range worst {
				if s.Status == SeverityInfo {
					break
				}
				fmt.Fprintf(w, "  %d. [%s] %s — %s\n", i+1, s.Status, s.Name, s.StatusReason)
			}
		}
	}

	fmt.Fprintf(w, "\n%d subscriptions checked (%d metrics collected, %d no data)\n",
		len(r.Subscriptions), r.Metadata.MetricsCollected, r.Metadata.MetricsNoData)
	return nil
}

// fmtAge converts seconds to a human-readable age string (e.g. "2h30m", "45s").
func fmtAge(sec float64) string {
	s := int64(sec)
	if s <= 0 {
		return "0s"
	}
	if s >= 3600 {
		return fmt.Sprintf("%dh%dm", s/3600, (s%3600)/60)
	}
	if s >= 60 {
		return fmt.Sprintf("%dm%ds", s/60, s%60)
	}
	return fmt.Sprintf("%ds", s)
}
