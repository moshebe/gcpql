package bigquery

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
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

// FormatCheckTable outputs check result as formatted table
func FormatCheckTable(w io.Writer, result *CheckResult) error {
	var b strings.Builder

	// Header
	b.WriteString(fmt.Sprintf("BigQuery Health Check: %s\n", result.Project))
	if result.Dataset != "" {
		b.WriteString(fmt.Sprintf("Dataset: %s\n", result.Dataset))
	}
	b.WriteString(fmt.Sprintf("Time: %s\n\n", result.Timestamp.Format("2006-01-02 15:04:05 MST")))

	// Slot Utilization Section
	slotStatus := getSlotStatus(result.Slots.Utilization)
	b.WriteString(fmt.Sprintf("SLOT UTILIZATION%50s%s\n", "", slotStatus))
	b.WriteString(fmt.Sprintf("  Allocated         %d slots\n", result.Slots.Allocated))
	b.WriteString(fmt.Sprintf("  Current           %d slots (%.1f%%)\n",
		result.Slots.Current, result.Slots.Utilization))
	if result.Slots.Peak > 0 {
		b.WriteString(fmt.Sprintf("  Peak              %d slots\n", result.Slots.Peak))
	}
	if result.Slots.QueriesInFlight > 0 || result.Slots.QueriesQueued > 0 {
		b.WriteString(fmt.Sprintf("  Queries           %d running", result.Slots.QueriesInFlight))
		if result.Slots.QueriesQueued > 0 {
			b.WriteString(fmt.Sprintf(", %d queued", result.Slots.QueriesQueued))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Cost Indicators Section
	costStatus := getCostStatus(result.Cost.StorageCostDaily, result.Cost.EstimatedCost)
	b.WriteString(fmt.Sprintf("COST INDICATORS%50s%s\n", "", costStatus))
	if result.Cost.StorageGB > 0 {
		b.WriteString(fmt.Sprintf("  Storage           %.1f GB ($%.2f/day)\n",
			result.Cost.StorageGB, result.Cost.StorageCostDaily))
	}
	if result.Cost.BytesScannedTotal > 0 {
		b.WriteString(fmt.Sprintf("  Bytes Scanned     %s\n",
			formatBytes(result.Cost.BytesScannedTotal)))
		b.WriteString(fmt.Sprintf("  Est. Query Cost   $%.2f\n", result.Cost.EstimatedCost))
	}
	b.WriteString("\n")

	// Top Queries Section
	if len(result.TopQueries) > 0 {
		b.WriteString("TOP EXPENSIVE QUERIES (by bytes processed)\n")
		for i, q := range result.TopQueries {
			b.WriteString(fmt.Sprintf("  %d. %s   %s   $%.2f   %s\n",
				i+1,
				truncate(q.Query, 50),
				formatBytes(q.BytesProcessed),
				q.EstimatedCost,
				q.UserEmail))
		}
		b.WriteString("\n")
	}

	// Metadata
	b.WriteString(fmt.Sprintf("Metrics: %d collected, %d no data | Collection time: %.1fs\n",
		result.Metadata.MetricsCollected,
		result.Metadata.MetricsNoData,
		float64(result.Metadata.CollectionDurationMS)/1000.0))

	_, err := w.Write([]byte(b.String()))
	return err
}

// getSlotStatus returns status indicator for slot utilization
func getSlotStatus(utilization float64) string {
	if utilization < 70 {
		return "🟢"
	} else if utilization < 90 {
		return "🟡"
	}
	return "🔴"
}

// getCostStatus returns status indicator for costs
func getCostStatus(storageCostDaily, queryCost float64) string {
	totalDaily := storageCostDaily + queryCost
	if totalDaily < 100 {
		return "🟢"
	} else if totalDaily < 500 {
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
