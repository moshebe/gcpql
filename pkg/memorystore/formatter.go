package memorystore

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

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

	if len(r.Instances) == 0 {
		fmt.Fprintln(w, "No Redis instances found.")
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.SetTitle("REDIS INSTANCE HEALTH")
	t.AppendHeader(table.Row{"Instance", "Memory %", "Clients", "Hit Ratio", "Keys", "Evicted", "Uptime", "Status"})

	for _, s := range r.Instances {
		t.AppendRow(table.Row{
			s.Name,
			fmt.Sprintf("%.1f%%", s.MemoryUsage*100),
			s.ConnectedClients,
			fmtHitRatio(s.CacheHitRatio, s.KeyCount),
			s.KeyCount,
			s.EvictedKeys,
			fmtUptime(s.UptimeSec),
			string(s.Status),
		})
	}
	t.Render()

	// Top offenders section
	if top > 0 && len(r.Instances) > 0 {
		worst := r.Instances
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

	fmt.Fprintf(w, "\n%d instances checked (%d metrics collected, %d no data)\n",
		len(r.Instances), r.Metadata.MetricsCollected, r.Metadata.MetricsNoData)
	return nil
}

// fmtHitRatio formats the cache hit ratio as a percentage, or "-" if no keys.
func fmtHitRatio(ratio float64, keys int64) string {
	if keys == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", ratio*100)
}

// fmtUptime formats seconds to a human-readable uptime string.
func fmtUptime(sec float64) string {
	if sec <= 0 {
		return "-"
	}
	d := time.Duration(sec) * time.Second
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if days > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh%dm", hours, int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}
