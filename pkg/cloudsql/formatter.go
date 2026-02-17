package cloudsql

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/jedib0t/go-pretty/v6/table"
)

// FormatJSON writes the check result as JSON
func FormatJSON(w io.Writer, result *CheckResult) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// FormatTable writes the check result as formatted tables
func FormatTable(w io.Writer, result *CheckResult) error {
	// Header
	fmt.Fprintf(w, "Instance: %s\n", result.Instance)
	fmt.Fprintf(w, "Region: %s\n", result.Region)
	fmt.Fprintf(w, "Time Window: %s\n", result.TimeWindow)
	fmt.Fprintf(w, "Instance Size: %d vCPU, %.0f GB RAM\n\n", result.InstanceSize.VCPU, result.InstanceSize.MemoryGB)

	// Resources table
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetTitle("RESOURCES")
	t.AppendHeader(table.Row{"Metric", "Current", "P50", "P99", "Max", "Unit"})

	t.AppendRow(table.Row{
		"CPU Utilization",
		formatPercent(result.Resources.CPU.Utilization.Current),
		formatPercent(result.Resources.CPU.Utilization.P50),
		formatPercent(result.Resources.CPU.Utilization.P99),
		formatPercent(result.Resources.CPU.Utilization.Max),
		"%",
	})

	t.AppendRow(table.Row{
		"Memory Usage",
		formatPercent(result.Resources.Memory.Utilization.Current),
		formatPercent(result.Resources.Memory.Utilization.P50),
		formatPercent(result.Resources.Memory.Utilization.P99),
		formatPercent(result.Resources.Memory.Utilization.Max),
		"%",
	})

	t.AppendRow(table.Row{
		"Disk Usage",
		formatPercent(result.Resources.Disk.Utilization.Current),
		"-",
		"-",
		formatPercent(result.Resources.Disk.Utilization.Max),
		"%",
	})

	if result.Resources.Disk.ReadOps.P50 > 0 {
		t.AppendRow(table.Row{
			"Disk Read Ops",
			"-",
			formatFloat(result.Resources.Disk.ReadOps.P50),
			formatFloat(result.Resources.Disk.ReadOps.P99),
			"-",
			"op/s",
		})
	}

	if result.Resources.Disk.WriteOps.P50 > 0 {
		t.AppendRow(table.Row{
			"Disk Write Ops",
			"-",
			formatFloat(result.Resources.Disk.WriteOps.P50),
			formatFloat(result.Resources.Disk.WriteOps.P99),
			"-",
			"op/s",
		})
	}

	t.Render()
	fmt.Fprintln(w)

	// Connections table
	t = table.NewWriter()
	t.SetOutputMirror(w)
	t.SetTitle("CONNECTIONS")
	t.AppendHeader(table.Row{"Metric", "Value", "Limit"})

	t.AppendRow(table.Row{
		"Total Connections",
		fmt.Sprintf("%.0f", result.Connections.Count.Current),
		result.Connections.MaxConnections,
	})

	if result.Connections.ByStatus.Active > 0 {
		t.AppendRow(table.Row{"  Active", result.Connections.ByStatus.Active, "-"})
	}
	if result.Connections.ByStatus.Idle > 0 {
		t.AppendRow(table.Row{"  Idle", result.Connections.ByStatus.Idle, "-"})
	}
	if result.Connections.ByStatus.IdleInTransaction > 0 {
		t.AppendRow(table.Row{"  Idle in Transaction", result.Connections.ByStatus.IdleInTransaction, "-"})
	}

	t.Render()
	fmt.Fprintln(w)

	// Query Performance table (if available)
	if result.QueryPerf.Available {
		t = table.NewWriter()
		t.SetOutputMirror(w)
		t.SetTitle("QUERY PERFORMANCE")
		t.AppendHeader(table.Row{"Metric", "P50", "P99", "Unit"})

		if result.QueryPerf.LatencyUS.P50 > 0 {
			t.AppendRow(table.Row{
				"Query Latency",
				formatFloat(result.QueryPerf.LatencyUS.P50 / 1000),
				formatFloat(result.QueryPerf.LatencyUS.P99 / 1000),
				"ms",
			})
		}

		if result.QueryPerf.IOTimeUS.P50 > 0 {
			t.AppendRow(table.Row{
				"I/O Wait Time",
				formatFloat(result.QueryPerf.IOTimeUS.P50 / 1000),
				formatFloat(result.QueryPerf.IOTimeUS.P99 / 1000),
				"ms",
			})
		}

		if result.QueryPerf.LockTimeUS.P50 > 0 {
			t.AppendRow(table.Row{
				"Lock Wait Time",
				formatFloat(result.QueryPerf.LockTimeUS.P50 / 1000),
				formatFloat(result.QueryPerf.LockTimeUS.P99 / 1000),
				"ms",
			})
		}

		t.Render()
		fmt.Fprintln(w)
	}

	return nil
}

func formatPercent(v float64) string {
	if v == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", v*100)
}

func formatFloat(v float64) string {
	if v == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f", v)
}
