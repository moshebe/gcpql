package bigquery

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/jedib0t/go-pretty/v6/table"
)

// FormatJSON outputs result as JSON
func FormatJSON(w io.Writer, result interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

// FormatCheckTable outputs check result as formatted tables
func FormatCheckTable(w io.Writer, result *CheckResult) error {
	// Header
	fmt.Fprintf(w, "Project: %s\n", result.Project)
	if result.Dataset != "" {
		fmt.Fprintf(w, "Dataset: %s\n", result.Dataset)
	}
	fmt.Fprintf(w, "Time: %s\n\n", result.Timestamp.Format("2006-01-02 15:04:05 MST"))

	// Slot Utilization
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.SetTitle("SLOT UTILIZATION")
	t.AppendHeader(table.Row{"Metric", "Value", "Status"})

	if result.Slots.Allocated > 0 {
		t.AppendRow(table.Row{"Allocated", fmt.Sprintf("%d slots", result.Slots.Allocated), "-"})
	}
	utilizationStatus := getSlotStatus(result.Slots.Utilization)
	t.AppendRow(table.Row{
		"Current Usage",
		fmt.Sprintf("%d slots (%.1f%%)", result.Slots.Current, result.Slots.Utilization),
		utilizationStatus,
	})
	if result.Slots.Peak > 0 {
		t.AppendRow(table.Row{"Peak Usage", fmt.Sprintf("%d slots", result.Slots.Peak), "-"})
	}
	if result.Slots.QueriesInFlight > 0 || result.Slots.QueriesQueued > 0 {
		queriesVal := fmt.Sprintf("%d running", result.Slots.QueriesInFlight)
		if result.Slots.QueriesQueued > 0 {
			queriesVal += fmt.Sprintf(", %d queued", result.Slots.QueriesQueued)
		}
		t.AppendRow(table.Row{"Queries", queriesVal, "-"})
	}
	t.Render()
	fmt.Fprintln(w)

	// Cost Indicators
	t = table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.SetTitle("COST INDICATORS")
	t.AppendHeader(table.Row{"Metric", "Value", "Status"})

	if result.Cost.StorageGB > 0 {
		storageStatus := getStorageCostStatus(result.Cost.StorageCostDaily)
		t.AppendRow(table.Row{
			"Storage",
			fmt.Sprintf("%.1f GB (%s/day)", result.Cost.StorageGB, formatCost(result.Cost.StorageCostDaily)),
			storageStatus,
		})
	} else {
		t.AppendRow(table.Row{"Storage", "no data", "-"})
	}
	if result.Jobs.TotalCost > 0 {
		queryCostStatus := getQueryCostStatus(result.Jobs.TotalCost)
		t.AppendRow(table.Row{"Est. Query Cost", formatCost(result.Jobs.TotalCost), queryCostStatus})
	}
	t.Render()
	fmt.Fprintln(w)

	// Query Summary
	if result.Jobs.TotalJobs > 0 {
		t = table.NewWriter()
		t.SetOutputMirror(w)
		t.SetStyle(table.StyleLight)
		t.SetTitle("QUERY SUMMARY")
		t.AppendHeader(table.Row{"Metric", "Value", "Status"})

		t.AppendRow(table.Row{"Total Queries", result.Jobs.TotalJobs, "-"})

		if result.Jobs.FailedJobs > 0 {
			pct := float64(result.Jobs.FailedJobs) / float64(result.Jobs.TotalJobs) * 100
			failStatus := getFailureRateStatus(pct)
			t.AppendRow(table.Row{
				"Failed",
				fmt.Sprintf("%d (%.1f%%)", result.Jobs.FailedJobs, pct),
				failStatus,
			})
		}

		cacheStatus := getCacheHitStatus(result.Jobs.CacheHitRate)
		t.AppendRow(table.Row{
			"Cache Hit Rate",
			fmt.Sprintf("%.1f%% (%d hits)", result.Jobs.CacheHitRate, result.Jobs.CacheHits),
			cacheStatus,
		})

		t.AppendRow(table.Row{"Bytes Scanned", formatBytes(result.Jobs.TotalBytes), "-"})

		t.Render()
		fmt.Fprintln(w)
	}

	// Top Expensive Queries
	if len(result.TopQueries) > 0 {
		t = table.NewWriter()
		t.SetOutputMirror(w)
		t.SetStyle(table.StyleLight)
		t.SetTitle("TOP EXPENSIVE QUERIES (by bytes processed)")
		t.AppendHeader(table.Row{"#", "User", "Bytes", "Cost", "Query"})
		t.SetColumnConfigs([]table.ColumnConfig{
			{Number: 5, WidthMax: 80},
		})

		for i, q := range result.TopQueries {
			queryText := truncate(normalizeQuery(q.Query), 80)
			if q.CacheHit {
				queryText += " [cached]"
			}
			t.AppendRow(table.Row{
				i + 1,
				q.UserEmail,
				formatBytes(q.BytesProcessed),
				formatCost(q.EstimatedCost),
				queryText,
			})
		}
		t.Render()
		fmt.Fprintln(w)
	}

	// Footer
	fmt.Fprintf(w, "Metrics: %d collected, %d no data | Collection time: %.1fs\n",
		result.Metadata.MetricsCollected,
		result.Metadata.MetricsNoData,
		float64(result.Metadata.CollectionDurationMS)/1000.0)

	return nil
}

// getSlotStatus returns emoji status for slot utilization
func getSlotStatus(utilization float64) string {
	if utilization < 70 {
		return "🟢"
	} else if utilization < 90 {
		return "🟡"
	}
	return "🔴"
}

// getStorageCostStatus returns emoji status for daily storage cost
func getStorageCostStatus(dailyCost float64) string {
	if dailyCost < 50 {
		return "🟢"
	} else if dailyCost < 200 {
		return "🟡"
	}
	return "🔴"
}

// getQueryCostStatus returns emoji status for total query cost
func getQueryCostStatus(totalCost float64) string {
	if totalCost < 100 {
		return "🟢"
	} else if totalCost < 500 {
		return "🟡"
	}
	return "🔴"
}

// getCacheHitStatus returns emoji status for cache hit rate
func getCacheHitStatus(rate float64) string {
	if rate >= 50 {
		return "🟢"
	} else if rate >= 20 {
		return "🟡"
	}
	return "🔴"
}

// getFailureRateStatus returns emoji status for query failure rate
func getFailureRateStatus(pct float64) string {
	if pct < 1 {
		return "🟢"
	} else if pct < 5 {
		return "🟡"
	}
	return "🔴"
}

// formatBytes converts bytes to human-readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// truncate truncates string to max length
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// normalizeQuery collapses SQL whitespace (newlines, tabs, extra spaces) into single spaces
func normalizeQuery(s string) string {
	return strings.Join(strings.FieldsFunc(s, unicode.IsSpace), " ")
}

// formatCost formats a dollar cost with precision scaled to magnitude
func formatCost(cost float64) string {
	switch {
	case cost >= 1.0:
		return fmt.Sprintf("$%.2f", cost)
	case cost >= 0.001:
		return fmt.Sprintf("$%.4f", cost)
	case cost > 0:
		return fmt.Sprintf("$%.6f", cost)
	default:
		return "$0.00"
	}
}
