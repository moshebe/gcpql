package cloudsql

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/jedib0t/go-pretty/v6/table"
)

// getStatusIndicator returns color-coded status emoji
func getStatusIndicator(value, greenThreshold, yellowThreshold float64, higherIsBetter bool) string {
	if higherIsBetter {
		if value >= greenThreshold {
			return "🟢"
		} else if value >= yellowThreshold {
			return "🟡"
		}
		return "🔴"
	}
	// Lower is better
	if value <= greenThreshold {
		return "🟢"
	} else if value <= yellowThreshold {
		return "🟡"
	}
	return "🔴"
}

func getCacheHitStatus(ratio float64) string {
	return getStatusIndicator(ratio, 95.0, 90.0, true)
}

func getConnectionUtilStatus(pct float64) string {
	return getStatusIndicator(pct, 70.0, 85.0, false)
}

func getTransactionAgeStatus(age int64) string {
	ageFloat := float64(age)
	return getStatusIndicator(ageFloat, 60.0, 300.0, false)
}

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

	// Derived Insights table (most actionable metrics)
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetTitle("DERIVED INSIGHTS")
	t.AppendHeader(table.Row{"Metric", "Value", "Status"})

	// Connection utilization
	if result.DerivedInsights.ConnectionUtilizationPct > 0 {
		connUtil := result.DerivedInsights.ConnectionUtilizationPct
		status := getConnectionUtilStatus(connUtil)
		t.AppendRow(table.Row{
			"Connection Utilization",
			fmt.Sprintf("%.1f%%", connUtil),
			status,
		})
	}

	// Cache hit ratio
	if result.DerivedInsights.CacheHitRatio > 0 {
		cacheRatio := result.DerivedInsights.CacheHitRatio
		status := getCacheHitStatus(cacheRatio)
		t.AppendRow(table.Row{
			"Cache Hit Ratio",
			fmt.Sprintf("%.1f%%", cacheRatio),
			status,
		})
	}

	// Oldest transaction age
	if result.DerivedInsights.OldestTransactionAgeSec > 0 {
		status := getTransactionAgeStatus(result.DerivedInsights.OldestTransactionAgeSec)
		t.AppendRow(table.Row{
			"Oldest Transaction Age",
			fmt.Sprintf("%ds", result.DerivedInsights.OldestTransactionAgeSec),
			status,
		})
	}

	// Memory per connection
	if result.Connections.Count.Current > 0 {
		memPerConn := (result.InstanceSize.MemoryGB * 1024) / result.Connections.Count.Current
		t.AppendRow(table.Row{
			"Memory per Connection",
			fmt.Sprintf("%.1f MB", memPerConn),
			"-",
		})
	}

	t.Render()
	fmt.Fprintln(w)

	// Resources table
	t = table.NewWriter()
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
		formatPercent(result.Resources.Disk.Utilization.P50),
		formatPercent(result.Resources.Disk.Utilization.P99),
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

	// Cache Performance table
	t = table.NewWriter()
	t.SetOutputMirror(w)
	t.SetTitle("CACHE PERFORMANCE")
	t.AppendHeader(table.Row{"Metric", "Value", "Unit"})

	if result.Cache.HitRatio > 0 {
		t.AppendRow(table.Row{
			"Buffer Cache Hit Ratio",
			fmt.Sprintf("%.2f", result.Cache.HitRatio),
			"%",
		})
	}

	if result.Cache.BlocksHit.P50 > 0 {
		t.AppendRow(table.Row{
			"Cache Blocks Hit (P50)",
			formatFloat(result.Cache.BlocksHit.P50),
			"blocks/s",
		})
	}

	if result.Cache.BlocksRead.P50 > 0 {
		t.AppendRow(table.Row{
			"Disk Blocks Read (P50)",
			formatFloat(result.Cache.BlocksRead.P50),
			"blocks/s",
		})
	}

	t.Render()
	fmt.Fprintln(w)

	// Throughput table
	t = table.NewWriter()
	t.SetOutputMirror(w)
	t.SetTitle("THROUGHPUT")
	t.AppendHeader(table.Row{"Metric", "P50", "P99", "Unit"})

	if result.Throughput.TuplesReturned.P50 > 0 {
		t.AppendRow(table.Row{
			"Tuples Returned/sec",
			formatFloat(result.Throughput.TuplesReturned.P50),
			formatFloat(result.Throughput.TuplesReturned.P99),
			"tuples/s",
		})
	}

	if result.Throughput.TuplesFetched.P50 > 0 {
		t.AppendRow(table.Row{
			"Tuples Fetched/sec",
			formatFloat(result.Throughput.TuplesFetched.P50),
			formatFloat(result.Throughput.TuplesFetched.P99),
			"tuples/s",
		})
	}

	if result.Throughput.TuplesInserted.P50 > 0 {
		t.AppendRow(table.Row{
			"Tuples Inserted/sec",
			formatFloat(result.Throughput.TuplesInserted.P50),
			formatFloat(result.Throughput.TuplesInserted.P99),
			"tuples/s",
		})
	}

	if result.Throughput.TuplesUpdated.P50 > 0 {
		t.AppendRow(table.Row{
			"Tuples Updated/sec",
			formatFloat(result.Throughput.TuplesUpdated.P50),
			formatFloat(result.Throughput.TuplesUpdated.P99),
			"tuples/s",
		})
	}

	if result.Throughput.TuplesDeleted.P50 > 0 {
		t.AppendRow(table.Row{
			"Tuples Deleted/sec",
			formatFloat(result.Throughput.TuplesDeleted.P50),
			formatFloat(result.Throughput.TuplesDeleted.P99),
			"tuples/s",
		})
	}

	if result.Throughput.ReadWriteRatio > 0 {
		t.AppendRow(table.Row{
			"Read/Write Ratio",
			fmt.Sprintf("%.2f", result.Throughput.ReadWriteRatio),
			"-",
			"",
		})
	}

	t.Render()
	fmt.Fprintln(w)

	// DATABASE HEALTH table
	t = table.NewWriter()
	t.SetOutputMirror(w)
	t.SetTitle("DATABASE HEALTH")
	t.AppendHeader(table.Row{"Metric", "Current", "P50", "P99", "Unit"})

	txUtil := result.DBHealth.TransactionIDUtilization
	t.AppendRow(table.Row{
		"Transaction ID Utilization",
		formatPercent(txUtil.Current),
		formatPercent(txUtil.P50),
		formatPercent(txUtil.P99),
		"%",
	})

	t.AppendRow(table.Row{
		"Deadlock Count",
		result.DBHealth.DeadlockCount,
		"-", "-", "",
	})

	if result.DBHealth.OldestTransactionAgeSec > 0 {
		t.AppendRow(table.Row{
			"Oldest Transaction Age",
			fmt.Sprintf("%ds", result.DBHealth.OldestTransactionAgeSec),
			"-", "-", "s",
		})
	}

	if result.DBHealth.AutovacuumCount > 0 {
		t.AppendRow(table.Row{"Autovacuum Count", result.DBHealth.AutovacuumCount, "-", "-", ""})
	}
	if result.DBHealth.AnalyzeCount > 0 {
		t.AppendRow(table.Row{"Analyze Count", result.DBHealth.AnalyzeCount, "-", "-", ""})
	}
	if result.DBHealth.VacuumCount > 0 {
		t.AppendRow(table.Row{"Vacuum Count", result.DBHealth.VacuumCount, "-", "-", ""})
	}

	t.Render()
	fmt.Fprintln(w)

	// CHECKPOINTS table
	t = table.NewWriter()
	t.SetOutputMirror(w)
	t.SetTitle("CHECKPOINTS")
	t.AppendHeader(table.Row{"Metric", "P50", "P99", "Unit"})

	if result.Checkpoints.SyncLatencyMS.P50 > 0 || result.Checkpoints.SyncLatencyMS.P99 > 0 {
		t.AppendRow(table.Row{
			"Sync Latency",
			formatFloat(result.Checkpoints.SyncLatencyMS.P50),
			formatFloat(result.Checkpoints.SyncLatencyMS.P99),
			"ms",
		})
	}
	if result.Checkpoints.WriteLatencyMS.P50 > 0 || result.Checkpoints.WriteLatencyMS.P99 > 0 {
		t.AppendRow(table.Row{
			"Write Latency",
			formatFloat(result.Checkpoints.WriteLatencyMS.P50),
			formatFloat(result.Checkpoints.WriteLatencyMS.P99),
			"ms",
		})
	}

	t.Render()
	fmt.Fprintln(w)

	// REPLICATION table (skip if no data)
	if result.Replication.ReplicaLagBytes.P50 > 0 || result.Replication.ReplicaLagBytes.P99 > 0 ||
		result.Replication.ReplicaLagSeconds.P50 > 0 || result.Replication.ReplicaLagSeconds.P99 > 0 {
		t = table.NewWriter()
		t.SetOutputMirror(w)
		t.SetTitle("REPLICATION")
		t.AppendHeader(table.Row{"Metric", "P50", "P99", "Unit"})

		if result.Replication.ReplicaLagBytes.P50 > 0 || result.Replication.ReplicaLagBytes.P99 > 0 {
			t.AppendRow(table.Row{
				"Replica Lag",
				formatFloat(result.Replication.ReplicaLagBytes.P50),
				formatFloat(result.Replication.ReplicaLagBytes.P99),
				"bytes",
			})
		}
		if result.Replication.ReplicaLagSeconds.P50 > 0 || result.Replication.ReplicaLagSeconds.P99 > 0 {
			t.AppendRow(table.Row{
				"Replica Lag",
				formatFloat(result.Replication.ReplicaLagSeconds.P50),
				formatFloat(result.Replication.ReplicaLagSeconds.P99),
				"s",
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
