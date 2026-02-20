package cloudsql

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/jedib0t/go-pretty/v6/table"
)

// FormatListJSON writes the list result as indented JSON.
func FormatListJSON(w io.Writer, result *ListResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// FormatListTable writes the list result as a human-readable table.
func FormatListTable(w io.Writer, result *ListResult) error {
	fmt.Fprintf(w, "Project: %s  (%d instances)\n\n", result.Project, len(result.Items))

	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"INSTANCE", "STATE", "VERSION", "REGION", "CPU", "MEM", "vCPU", "RAM"})

	for _, item := range result.Items {
		cpuStr := "-"
		if item.CPUPct != nil {
			cpuStr = formatListPct(*item.CPUPct)
		}
		memStr := "-"
		if item.MemPct != nil {
			memStr = formatListPct(*item.MemPct)
		}
		vcpuStr := "-"
		if item.VCPU > 0 {
			vcpuStr = fmt.Sprintf("%d", item.VCPU)
		}
		ramStr := "-"
		if item.MemoryGB > 0 {
			ramStr = fmt.Sprintf("%.0fGB", item.MemoryGB)
		}

		t.AppendRow(table.Row{
			item.Instance,
			item.State,
			item.DBVersion,
			item.Region,
			cpuStr,
			memStr,
			vcpuStr,
			ramStr,
		})
	}

	t.Render()
	return nil
}

// formatListPct formats a percentage with a status indicator.
// ≥90% → red, ≥70% → yellow, <70% → green
func formatListPct(pct float64) string {
	indicator := getStatusIndicator(pct, 70, 90, false)
	return fmt.Sprintf("%s %.0f%%", indicator, pct)
}
