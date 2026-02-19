package cloudsql

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

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

func getXIDWraparoundStatus(pct float64) string {
	return getStatusIndicator(pct, 50.0, 80.0, false)
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

	// Instance Config table
	if result.InstanceConfig.State != "" || result.InstanceConfig.AvailabilityType != "" {
		t := table.NewWriter()
		t.SetOutputMirror(w)
		t.SetTitle("INSTANCE CONFIG")
		t.SetStyle(table.StyleLight)
		t.AppendHeader(table.Row{"Property", "Value"})
		t.SetColumnConfigs([]table.ColumnConfig{
			{Number: 2, WidthMax: 70},
		})

		if result.InstanceConfig.State != "" {
			t.AppendRow(table.Row{"State", result.InstanceConfig.State})
		}
		if result.InstanceConfig.AvailabilityType != "" {
			t.AppendRow(table.Row{"Availability", result.InstanceConfig.AvailabilityType})
		}
		if result.InstanceConfig.ConnectionName != "" {
			t.AppendRow(table.Row{"Connection Name", result.InstanceConfig.ConnectionName})
		}

		backupVal := "disabled"
		if result.InstanceConfig.BackupEnabled {
			pitr := "PITR: off"
			if result.InstanceConfig.PITREnabled {
				pitr = "PITR: on"
			}
			backupVal = fmt.Sprintf("enabled (%s UTC, %s)", result.InstanceConfig.BackupStartTime, pitr)
		}
		t.AppendRow(table.Row{"Backup", backupVal})

		if result.InstanceConfig.StorageType != "" {
			storageVal := result.InstanceConfig.StorageType
			if result.InstanceConfig.StorageAutoResize && result.InstanceConfig.StorageAutoResizeGB > 0 {
				storageVal = fmt.Sprintf("%s, auto-resize up to %d GB", storageVal, result.InstanceConfig.StorageAutoResizeGB)
			} else if result.InstanceConfig.StorageAutoResize {
				storageVal = fmt.Sprintf("%s, auto-resize enabled", storageVal)
			}
			t.AppendRow(table.Row{"Storage", storageVal})
		}

		if result.InstanceConfig.DeletionProtection {
			t.AppendRow(table.Row{"Deletion Protection", "enabled"})
		}

		if len(result.InstanceConfig.Labels) > 0 {
			labelParts := make([]string, 0, len(result.InstanceConfig.Labels))
			for k, v := range result.InstanceConfig.Labels {
				labelParts = append(labelParts, fmt.Sprintf("%s=%s", k, v))
			}
			sort.Strings(labelParts)
			t.AppendRow(table.Row{"Labels", strings.Join(labelParts, ", ")})
		}

		if result.InstanceConfig.QueryInsightsEnabled {
			t.AppendRow(table.Row{"Query Insights", "enabled"})
		}

		t.Render()

		// DB Flags rendered as a sorted list — too long for table cells
		if len(result.InstanceConfig.DatabaseFlags) > 0 {
			flags := make([]DBFlag, len(result.InstanceConfig.DatabaseFlags))
			copy(flags, result.InstanceConfig.DatabaseFlags)
			sort.Slice(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })
			fmt.Fprintf(w, "\nDB Flags (%d):\n", len(flags))
			for _, f := range flags {
				fmt.Fprintf(w, "  %-44s %s\n", f.Name, f.Value)
			}
		}
		fmt.Fprintln(w)
	}

	// Derived Insights table (most actionable metrics)
	t := table.NewWriter()
	t.SetOutputMirror(w)
	t.SetStyle(table.StyleLight)
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

	// XID wraparound risk (PostgreSQL 32-bit transaction ID exhaustion)
	if result.DerivedInsights.XIDWraparoundRisk > 0 {
		xid := result.DerivedInsights.XIDWraparoundRisk
		t.AppendRow(table.Row{
			"XID Wraparound Risk",
			fmt.Sprintf("%.1f%%", xid),
			getXIDWraparoundStatus(xid),
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
	t.SetStyle(table.StyleLight)
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
	t.SetStyle(table.StyleLight)
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
		t.SetStyle(table.StyleLight)
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

	// Cache Performance table (skip if no data)
	if result.Cache.HitRatio > 0 || result.Cache.BlocksHit.P50 > 0 || result.Cache.BlocksRead.P50 > 0 {
		t = table.NewWriter()
		t.SetOutputMirror(w)
		t.SetStyle(table.StyleLight)
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
	}

	// Throughput table (skip if no data)
	tp := result.Throughput
	if tp.TuplesReturned.P50 > 0 || tp.TuplesFetched.P50 > 0 || tp.TuplesInserted.P50 > 0 ||
		tp.TuplesUpdated.P50 > 0 || tp.TuplesDeleted.P50 > 0 || tp.ReadWriteRatio > 0 {
		t = table.NewWriter()
		t.SetOutputMirror(w)
		t.SetStyle(table.StyleLight)
		t.SetTitle("THROUGHPUT")
		t.AppendHeader(table.Row{"Metric", "P50", "P99", "Unit"})

		if tp.TuplesReturned.P50 > 0 {
			t.AppendRow(table.Row{
				"Tuples Returned/sec",
				formatFloat(tp.TuplesReturned.P50),
				formatFloat(tp.TuplesReturned.P99),
				"tuples/s",
			})
		}

		if tp.TuplesFetched.P50 > 0 {
			t.AppendRow(table.Row{
				"Tuples Fetched/sec",
				formatFloat(tp.TuplesFetched.P50),
				formatFloat(tp.TuplesFetched.P99),
				"tuples/s",
			})
		}

		if tp.TuplesInserted.P50 > 0 {
			t.AppendRow(table.Row{
				"Tuples Inserted/sec",
				formatFloat(tp.TuplesInserted.P50),
				formatFloat(tp.TuplesInserted.P99),
				"tuples/s",
			})
		}

		if tp.TuplesUpdated.P50 > 0 {
			t.AppendRow(table.Row{
				"Tuples Updated/sec",
				formatFloat(tp.TuplesUpdated.P50),
				formatFloat(tp.TuplesUpdated.P99),
				"tuples/s",
			})
		}

		if tp.TuplesDeleted.P50 > 0 {
			t.AppendRow(table.Row{
				"Tuples Deleted/sec",
				formatFloat(tp.TuplesDeleted.P50),
				formatFloat(tp.TuplesDeleted.P99),
				"tuples/s",
			})
		}

		if tp.ReadWriteRatio > 0 {
			t.AppendRow(table.Row{
				"Read/Write Ratio",
				fmt.Sprintf("%.2f", tp.ReadWriteRatio),
				"-",
				"",
			})
		}

		t.Render()
		fmt.Fprintln(w)
	}

	// DATABASE HEALTH table — only rendered when there's something actionable.
	// XID wraparound risk is surfaced in DERIVED INSIGHTS above.
	dbh := result.DBHealth
	hasDBHealth := dbh.DeadlockCount > 0 || dbh.OldestTransactionAgeSec > 0 ||
		dbh.AutovacuumCount > 0 || dbh.AnalyzeCount > 0 || dbh.VacuumCount > 0
	if hasDBHealth {
		t = table.NewWriter()
		t.SetOutputMirror(w)
		t.SetStyle(table.StyleLight)
		t.SetTitle("DATABASE HEALTH")
		t.AppendHeader(table.Row{"Metric", "Value"})

		if dbh.DeadlockCount > 0 {
			t.AppendRow(table.Row{"Deadlocks (current)", dbh.DeadlockCount})
		}
		if dbh.OldestTransactionAgeSec > 0 {
			t.AppendRow(table.Row{"Oldest Open Transaction", fmt.Sprintf("%ds", dbh.OldestTransactionAgeSec)})
		}
		if dbh.AutovacuumCount > 0 {
			t.AppendRow(table.Row{"Autovacuum Runs", dbh.AutovacuumCount})
		}
		if dbh.AnalyzeCount > 0 {
			t.AppendRow(table.Row{"Analyze Runs", dbh.AnalyzeCount})
		}
		if dbh.VacuumCount > 0 {
			t.AppendRow(table.Row{"Manual Vacuum Runs", dbh.VacuumCount})
		}

		t.Render()
		fmt.Fprintln(w)
	}

	// CHECKPOINTS table (skip if no data)
	cp := result.Checkpoints
	if cp.SyncLatencyMS.P50 > 0 || cp.SyncLatencyMS.P99 > 0 ||
		cp.WriteLatencyMS.P50 > 0 || cp.WriteLatencyMS.P99 > 0 {
		t = table.NewWriter()
		t.SetOutputMirror(w)
		t.SetStyle(table.StyleLight)
		t.SetTitle("CHECKPOINTS")
		t.AppendHeader(table.Row{"Metric", "P50", "P99", "Unit"})

		if cp.SyncLatencyMS.P50 > 0 || cp.SyncLatencyMS.P99 > 0 {
			t.AppendRow(table.Row{
				"Sync Latency",
				formatFloat(cp.SyncLatencyMS.P50),
				formatFloat(cp.SyncLatencyMS.P99),
				"ms",
			})
		}
		if cp.WriteLatencyMS.P50 > 0 || cp.WriteLatencyMS.P99 > 0 {
			t.AppendRow(table.Row{
				"Write Latency",
				formatFloat(cp.WriteLatencyMS.P50),
				formatFloat(cp.WriteLatencyMS.P99),
				"ms",
			})
		}

		t.Render()
		fmt.Fprintln(w)
	}

	// REPLICATION table (skip if no data)
	if result.Replication.ReplicaLagBytes.P50 > 0 || result.Replication.ReplicaLagBytes.P99 > 0 ||
		result.Replication.ReplicaLagSeconds.P50 > 0 || result.Replication.ReplicaLagSeconds.P99 > 0 {
		t = table.NewWriter()
		t.SetOutputMirror(w)
		t.SetStyle(table.StyleLight)
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

	// Recommendations table
	if result.Recommendations.Available && len(result.Recommendations.Items) > 0 {
		t = table.NewWriter()
		t.SetOutputMirror(w)
		t.SetStyle(table.StyleLight)
		t.SetTitle("RECOMMENDATIONS")
		t.AppendHeader(table.Row{"Impact", "State", "Description"})
		for _, r := range result.Recommendations.Items {
			t.AppendRow(table.Row{r.Impact, r.State, r.Description})
		}
		t.Render()
		fmt.Fprintln(w)
	}

	// Query Insights table
	if result.QueryInsights.Available && len(result.QueryInsights.TopQueries) > 0 {
		t = table.NewWriter()
		t.SetOutputMirror(w)
		t.SetStyle(table.StyleLight)
		t.SetTitle("QUERY INSIGHTS — Top Users by Execution Time (aggregated by user/database, no per-query breakdown)")
		t.AppendHeader(table.Row{"#", "User", "Database", "Client", "Samples", "Avg (ms)", "Total (ms)"})
		for i, q := range result.QueryInsights.TopQueries {
			t.AppendRow(table.Row{
				i + 1,
				q.User,
				q.Database,
				q.ClientAddr,
				q.SampleCount,
				fmt.Sprintf("%.1f", q.AvgLatencyMS),
				fmt.Sprintf("%.0f", q.TotalTimeMS),
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
