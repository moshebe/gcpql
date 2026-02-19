package cloudsql

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
)

// Severity levels for diagnostic findings.
type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityWarning  Severity = "WARNING"
	SeverityInfo     Severity = "INFO"
)

// Finding is a single diagnostic issue with suggested remediation.
type Finding struct {
	Severity Severity `json:"severity"`
	Title    string   `json:"title"`
	Detail   string   `json:"detail"`
	Actions  []string `json:"actions"`
}

// LoadSummary provides a quick view of database activity.
type LoadSummary struct {
	AvgTPS            float64 `json:"avg_tps,omitempty"`
	ReadTuplesPerSec  float64 `json:"read_tuples_per_sec_p50,omitempty"`
	WriteTuplesPerSec float64 `json:"write_tuples_per_sec_p50,omitempty"`
	AvgQueryLatencyMS float64 `json:"avg_query_latency_ms_p50,omitempty"`
	P99QueryLatencyMS float64 `json:"p99_query_latency_ms,omitempty"`
}

// DiagnoseResult is the full output of a diagnose run.
type DiagnoseResult struct {
	Instance   string      `json:"instance"`
	Region     string      `json:"region"`
	TimeWindow string      `json:"time_window"`
	Load       LoadSummary `json:"load"`
	Findings   []Finding   `json:"findings"`
}

// Diagnose analyzes a CheckResult and returns actionable findings sorted by severity.
func Diagnose(r *CheckResult, timeWindow time.Duration) *DiagnoseResult {
	dr := &DiagnoseResult{
		Instance:   r.Instance,
		Region:     r.Region,
		TimeWindow: r.TimeWindow,
		Load:       computeLoadSummary(r, timeWindow),
	}

	var findings []Finding
	add := func(f Finding) { findings = append(findings, f) }

	// Critical
	diagXIDWraparound(r, add)
	diagConnectionExhaustion(r, add)
	diagDiskFull(r, add)
	diagLongTransactions(r, add)

	// Warning
	diagCPUPressure(r, add)
	diagMemoryPressure(r, add)
	diagCacheHitRatio(r, add)
	diagDeadlocks(r, add)
	diagTempDataSpill(r, add)
	diagCheckpointLatency(r, add)
	diagReplicaLag(r, add)
	diagLockContention(r, add)
	diagAutovacuum(r, add)

	// Underutilization
	diagUnderutilization(r, add)

	// Query insights
	diagQueryInsights(r, add)

	// Config / INFO
	diagConfig(r, add)

	// Sort: CRITICAL → WARNING → INFO
	sortFindings(findings)
	dr.Findings = findings
	return dr
}

func computeLoadSummary(r *CheckResult, window time.Duration) LoadSummary {
	var ls LoadSummary

	// TPS: transaction_count is a DELTA metric — each sample is the number of
	// transactions in the past 60s scrape interval. Sum / totalSeconds = avg TPS.
	if window.Seconds() > 0 && r.DBHealth.TransactionCount > 0 {
		ls.AvgTPS = float64(r.DBHealth.TransactionCount) / window.Seconds()
	}

	ls.ReadTuplesPerSec = r.Throughput.TuplesReturned.P50
	ls.WriteTuplesPerSec = r.Throughput.TuplesInserted.P50 +
		r.Throughput.TuplesUpdated.P50 +
		r.Throughput.TuplesDeleted.P50

	if r.QueryPerf.LatencyUS.P50 > 0 {
		ls.AvgQueryLatencyMS = r.QueryPerf.LatencyUS.P50 / 1000
	}
	if r.QueryPerf.LatencyUS.P99 > 0 {
		ls.P99QueryLatencyMS = r.QueryPerf.LatencyUS.P99 / 1000
	}
	return ls
}

// ── Rule implementations ──────────────────────────────────────────────────────

func diagXIDWraparound(r *CheckResult, add func(Finding)) {
	xid := r.DerivedInsights.XIDWraparoundRisk
	if xid > 80 {
		add(Finding{
			Severity: SeverityCritical,
			Title:    "XID Wraparound Imminent",
			Detail:   fmt.Sprintf("%.1f%% of PostgreSQL transaction IDs consumed (critical threshold: 80%%)", xid),
			Actions: []string{
				"Run VACUUM FREEZE on all databases immediately to reclaim XIDs",
				"Check oldest XID: SELECT datname, age(datfrozenxid) FROM pg_database ORDER BY 2 DESC",
				"Review autovacuum_freeze_max_age and vacuum_freeze_min_age settings",
				"Emergency manual VACUUM FREEZE may be needed if autovacuum is lagging",
			},
		})
	} else if xid > 50 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "XID Wraparound Risk Growing",
			Detail:   fmt.Sprintf("%.1f%% of PostgreSQL transaction IDs consumed (alert at 80%%)", xid),
			Actions: []string{
				"Schedule VACUUM FREEZE during a low-traffic window",
				"Monitor: SELECT datname, age(datfrozenxid) FROM pg_database ORDER BY 2 DESC",
				"Review autovacuum_freeze_max_age to ensure freezing runs often enough",
			},
		})
	}
}

func diagConnectionExhaustion(r *CheckResult, add func(Finding)) {
	util := r.DerivedInsights.ConnectionUtilizationPct
	cur := r.Connections.Count.Current
	maxConn := r.Connections.MaxConnections
	if util > 90 {
		add(Finding{
			Severity: SeverityCritical,
			Title:    "Connection Pool Near Exhaustion",
			Detail:   fmt.Sprintf("%.1f%% of max connections used (%.0f / %d)", util, cur, maxConn),
			Actions: []string{
				"Deploy PgBouncer or a connection pooler immediately",
				"Kill idle connections: SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE state='idle' AND state_change < now()-interval '10 minutes'",
				"Investigate connection leak in application code",
				"Increase max_connections flag in Cloud SQL settings (requires instance restart)",
			},
		})
	} else if util > 75 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "High Connection Utilization",
			Detail:   fmt.Sprintf("%.1f%% of max connections used (%.0f / %d)", util, cur, maxConn),
			Actions: []string{
				"Consider deploying a connection pooler (PgBouncer, pgpool-II)",
				"Review application connection pool settings (min/max pool size, idle timeout)",
				"Check for idle connections holding resources",
			},
		})
	}
}

func diagDiskFull(r *CheckResult, add func(Finding)) {
	diskPct := r.Resources.Disk.Utilization.Current * 100
	autoResize := r.InstanceConfig.StorageAutoResize
	if diskPct > 90 {
		actions := []string{
			"Increase disk size immediately in Cloud SQL console",
			"Enable storage auto-resize to prevent future outages",
			"Identify large tables: SELECT relname, pg_size_pretty(pg_total_relation_size(oid)) FROM pg_class ORDER BY 2 DESC LIMIT 20",
		}
		if autoResize {
			actions = []string{
				"Auto-resize is on — check if the auto-resize limit has been reached in Cloud SQL settings",
				"Increase or remove the auto-resize cap",
				"Identify and purge large/bloated tables or indexes",
			}
		}
		add(Finding{
			Severity: SeverityCritical,
			Title:    "Disk Nearly Full",
			Detail:   fmt.Sprintf("%.1f%% disk used (auto-resize: %v)", diskPct, autoResize),
			Actions:  actions,
		})
	} else if diskPct > 75 && !autoResize {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Disk Growing — No Auto-resize",
			Detail:   fmt.Sprintf("%.1f%% disk used and storage auto-resize is disabled", diskPct),
			Actions: []string{
				"Enable storage auto-resize in Cloud SQL settings",
				"Or schedule manual disk expansion before hitting 90%",
			},
		})
	}
}

func diagLongTransactions(r *CheckResult, add func(Finding)) {
	age := r.DerivedInsights.OldestTransactionAgeSec
	if age > 3600 {
		add(Finding{
			Severity: SeverityCritical,
			Title:    "Long-running Transaction (>1h)",
			Detail:   fmt.Sprintf("Oldest open transaction: %s — blocks autovacuum and causes table bloat", fmtAge(age)),
			Actions: []string{
				"Find it: SELECT pid, now()-xact_start AS age, state, left(query,100) FROM pg_stat_activity WHERE xact_start IS NOT NULL ORDER BY xact_start LIMIT 5",
				"Kill with: SELECT pg_terminate_backend(<pid>)",
				"Review application for missing COMMIT/ROLLBACK",
				"Set idle_in_transaction_session_timeout to auto-kill stale sessions",
			},
		})
	} else if age > longTransactionThresholdSec {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Long-running Transaction (>5min)",
			Detail:   fmt.Sprintf("Oldest open transaction: %s — may block autovacuum", fmtAge(age)),
			Actions: []string{
				"Find it: SELECT pid, now()-xact_start AS age, state, left(query,100) FROM pg_stat_activity WHERE xact_start IS NOT NULL ORDER BY xact_start",
				"Review application for long-running or leaked transactions",
				"Set idle_in_transaction_session_timeout to auto-kill stale sessions",
			},
		})
	}
}

func diagCPUPressure(r *CheckResult, add func(Finding)) {
	p99 := r.Resources.CPU.Utilization.P99 * 100
	if p99 > 90 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "High CPU Pressure",
			Detail:   fmt.Sprintf("CPU P99: %.1f%% — sustained load may cause query latency spikes", p99),
			Actions: []string{
				"Identify expensive queries via Query Insights (enable if not already on)",
				"Check for missing indexes causing full table scans",
				"Review autovacuum/analyze frequency — excessive vacuum can spike CPU",
				"Consider upgrading to a larger instance tier if CPU is consistently high",
			},
		})
	} else if p99 > 80 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Elevated CPU Usage",
			Detail:   fmt.Sprintf("CPU P99: %.1f%%", p99),
			Actions: []string{
				"Monitor for trends — consider upgrade if sustained over 7d",
				"Enable Query Insights to identify expensive queries",
			},
		})
	}
}

func diagMemoryPressure(r *CheckResult, add func(Finding)) {
	cur := r.Resources.Memory.Utilization.Current * 100
	if cur > 90 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "High Memory Pressure",
			Detail:   fmt.Sprintf("Memory at %.1f%% — risk of OOM and PostgreSQL backend crashes", cur),
			Actions: []string{
				"Upgrade instance to a tier with more RAM",
				"Reduce shared_buffers or effective_cache_size if over-allocated",
				"Lower max_connections — each idle connection consumes ~5–10 MB",
				"Check for memory-leaking queries with large sorts/hash joins (work_mem)",
			},
		})
	}
}

func diagCacheHitRatio(r *CheckResult, add func(Finding)) {
	ratio := r.DerivedInsights.CacheHitRatio
	if ratio == 0 {
		return
	}
	if ratio < 80 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Poor Buffer Cache Hit Ratio",
			Detail:   fmt.Sprintf("%.1f%% hit rate — >20%% of reads go to disk (target: >95%%)", ratio),
			Actions: []string{
				"Increase instance memory to allow larger shared_buffers",
				"Identify tables with high disk reads: SELECT relname, heap_blks_read, heap_blks_hit FROM pg_statio_user_tables ORDER BY heap_blks_read DESC LIMIT 10",
				"Investigate queries doing large sequential scans that flush the cache",
				"Consider partitioning large hot tables",
			},
		})
	} else if ratio < 90 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Low Buffer Cache Hit Ratio",
			Detail:   fmt.Sprintf("%.1f%% hit rate (target: >95%%)", ratio),
			Actions: []string{
				"Consider upgrading to an instance with more RAM",
				"Check pg_statio_user_tables for high heap_blks_read tables",
			},
		})
	}
}

func diagDeadlocks(r *CheckResult, add func(Finding)) {
	count := r.DBHealth.DeadlockCount
	if count > 10 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Frequent Deadlocks",
			Detail:   fmt.Sprintf("%d deadlocks in the time window", count),
			Actions: []string{
				"Enable log_lock_waits=on and deadlock_timeout to log conflicting queries",
				"Ensure application always acquires locks in a consistent row/table order",
				"Use SELECT ... FOR UPDATE NOWAIT or SKIP LOCKED for concurrent patterns",
				"Review bulk update/delete operations that may conflict",
			},
		})
	} else if count > 0 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Deadlocks Detected",
			Detail:   fmt.Sprintf("%d deadlock(s) in the time window", count),
			Actions: []string{
				"Enable log_lock_waits=on to identify conflicting queries",
				"Ensure consistent lock ordering in application transactions",
			},
		})
	}
}

func diagTempDataSpill(r *CheckResult, add func(Finding)) {
	rate := r.DerivedInsights.TempDataRateMBPerSec
	if rate > 100 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Severe Disk Spill (work_mem too low)",
			Detail:   fmt.Sprintf("%.1f MB/s of temp data written — queries are heavily spilling to disk", rate),
			Actions: []string{
				"Increase work_mem in Cloud SQL flags (caution: multiplied by max_connections)",
				"Find spilling queries with EXPLAIN (ANALYZE, BUFFERS) — look for 'external merge Disk' or 'Batches > 1'",
				"Add indexes to reduce sort/hash operations in hot queries",
				"Consider increasing instance memory to allow a larger work_mem budget",
			},
		})
	} else if rate > 10 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Elevated Disk Spill (work_mem)",
			Detail:   fmt.Sprintf("%.1f MB/s of temp data written", rate),
			Actions: []string{
				"Review work_mem setting — spill indicates memory pressure during sorts/hash joins",
				"Use EXPLAIN (ANALYZE, BUFFERS) to find queries with 'external merge Disk'",
			},
		})
	}
}

func diagCheckpointLatency(r *CheckResult, add func(Finding)) {
	p99 := r.Checkpoints.SyncLatencyMS.P99
	if p99 > 1000 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Slow Checkpoint Sync",
			Detail:   fmt.Sprintf("Checkpoint sync P99: %.0fms — indicates I/O saturation", p99),
			Actions: []string{
				"Upgrade to SSD storage (PD-SSD) if currently on HDD",
				"Tune checkpoint_completion_target=0.9 to spread writes over more time",
				"Increase checkpoint_timeout to reduce frequency",
				"Check disk write IOPS against Cloud SQL tier limits",
			},
		})
	} else if p99 > 500 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "High Checkpoint Sync Latency",
			Detail:   fmt.Sprintf("Checkpoint sync P99: %.0fms", p99),
			Actions: []string{
				"Tune checkpoint_completion_target=0.9 to spread I/O",
				"Monitor disk write IOPS — high latency often precedes I/O bottlenecks",
			},
		})
	}
}

func diagReplicaLag(r *CheckResult, add func(Finding)) {
	lagSec := r.Replication.ReplicaLagSeconds.Current
	if lagSec > 30 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "High Replica Lag",
			Detail:   fmt.Sprintf("Replica lag: %.0fs — reads from replica may be significantly stale", lagSec),
			Actions: []string{
				"Check for long-running transactions on primary blocking replication",
				"Verify replica has sufficient CPU/disk to keep up with write load",
				"Reduce bulk write operations or add rate limiting",
				"Consider write throttling during high-lag periods",
			},
		})
	} else if lagSec > 5 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Replica Lag Detected",
			Detail:   fmt.Sprintf("Replica lag: %.0fs", lagSec),
			Actions: []string{
				"Monitor for trends — occasional lag is normal during heavy writes",
				"Check for long transactions on primary blocking WAL apply",
			},
		})
	}
}

func diagLockContention(r *CheckResult, add func(Finding)) {
	lockP99 := r.QueryPerf.LockTimeUS.P99
	latencyP99 := r.QueryPerf.LatencyUS.P99
	if latencyP99 <= 0 || lockP99 <= 0 {
		return
	}
	ratio := (lockP99 / latencyP99) * 100
	if ratio > 50 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "High Lock Contention",
			Detail:   fmt.Sprintf("Lock wait is %.0f%% of P99 query latency (%.1fms lock / %.1fms total)", ratio, lockP99/1000, latencyP99/1000),
			Actions: []string{
				"Identify blocked queries: SELECT pid, wait_event_type, wait_event, left(query,100) FROM pg_stat_activity WHERE wait_event_type='Lock'",
				"Find the blocker: SELECT blocking_pids FROM pg_stat_activity WHERE cardinality(pg_blocking_pids(pid)) > 0",
				"Review DDL operations (ALTER TABLE, CREATE INDEX) scheduled during peak hours",
				"Use SELECT ... FOR UPDATE SKIP LOCKED for queue-like patterns",
			},
		})
	} else if ratio > 25 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Lock Contention",
			Detail:   fmt.Sprintf("Lock wait is %.0f%% of P99 query latency", ratio),
			Actions: []string{
				"Check: SELECT pid, wait_event, left(query,100) FROM pg_stat_activity WHERE wait_event_type='Lock'",
				"Review long-running transactions that may be holding locks",
			},
		})
	}
}

func diagAutovacuum(r *CheckResult, add func(Finding)) {
	freq := r.DerivedInsights.AutovacuumFrequencyPerHour
	if freq > 60 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Excessive Autovacuum Activity",
			Detail:   fmt.Sprintf("%.0f autovacuum runs/hour — may indicate high table bloat or aggressive settings", freq),
			Actions: []string{
				"Check table bloat: SELECT relname, n_dead_tup, last_autovacuum FROM pg_stat_user_tables ORDER BY n_dead_tup DESC LIMIT 10",
				"Consider lowering autovacuum_vacuum_scale_factor for large, high-churn tables",
				"Ensure long transactions aren't preventing dead tuple cleanup",
				"Excessive vacuum on a single table may need table-level autovacuum tuning",
			},
		})
	} else if freq > 20 {
		add(Finding{
			Severity: SeverityInfo,
			Title:    "High Autovacuum Frequency",
			Detail:   fmt.Sprintf("%.0f autovacuum runs/hour — normal for write-heavy workloads", freq),
			Actions: []string{
				"Monitor if autovacuum causes I/O contention during peak hours",
				"Consider scheduling heavy VACUUM ANALYZE during off-peak if needed",
			},
		})
	}
}

func diagUnderutilization(r *CheckResult, add func(Finding)) {
	vcpu := r.InstanceSize.VCPU
	memGB := r.InstanceSize.MemoryGB

	// Only flag for non-trivial instances
	if vcpu < 4 && memGB < 8 {
		return
	}

	cpuP99 := r.Resources.CPU.Utilization.P99 * 100
	memCur := r.Resources.Memory.Utilization.Current * 100
	connUtil := r.DerivedInsights.ConnectionUtilizationPct

	underCPU := cpuP99 > 0 && cpuP99 < 20
	underMem := memCur > 0 && memCur < 25 && memGB >= 8
	underConn := connUtil > 0 && connUtil < 10 && r.Connections.MaxConnections >= 200

	var signals []string
	if underCPU {
		signals = append(signals, fmt.Sprintf("CPU P99 %.1f%%", cpuP99))
	}
	if underMem {
		signals = append(signals, fmt.Sprintf("memory %.1f%%", memCur))
	}
	if underConn {
		signals = append(signals, fmt.Sprintf("connections %.1f%% of max", connUtil))
	}

	if len(signals) >= 2 {
		nextVCPU := max(vcpu/2, 1)
		nextMemGB := max(memGB/2, 1.0)
		add(Finding{
			Severity: SeverityInfo,
			Title:    "Instance May Be Oversized",
			Detail:   fmt.Sprintf("%d vCPU, %.0f GB RAM — low utilization: %s", vcpu, memGB, strings.Join(signals, ", ")),
			Actions: []string{
				fmt.Sprintf("Consider downsizing to ~%d vCPU / %.0f GB to reduce costs", nextVCPU, nextMemGB),
				"Verify low utilization isn't due to a traffic dip — check with --since 7d",
				"Review peak usage before downsizing to avoid capacity surprises",
			},
		})
	} else if underCPU && vcpu >= 8 {
		add(Finding{
			Severity: SeverityInfo,
			Title:    "CPU Potentially Underutilized",
			Detail:   fmt.Sprintf("CPU P99 %.1f%% on %d-vCPU instance", cpuP99, vcpu),
			Actions: []string{
				"Verify over a 7d window before downsizing",
				"Consider reducing vCPU count to lower compute costs",
			},
		})
	}
}

func diagQueryInsights(r *CheckResult, add func(Finding)) {
	if !r.QueryInsights.Available || len(r.QueryInsights.TopQueries) == 0 {
		return
	}

	queries := r.QueryInsights.TopQueries
	top := queries[0]

	// Slow top query
	if top.AvgLatencyMS > 10_000 {
		add(Finding{
			Severity: SeverityCritical,
			Title:    "Critically Slow Query Pattern",
			Detail:   fmt.Sprintf("Top contributor '%s'@'%s': avg %.0fms (%.1fs) latency — queries taking >10s on average", top.User, top.Database, top.AvgLatencyMS, top.AvgLatencyMS/1000),
			Actions: []string{
				"Run EXPLAIN (ANALYZE, BUFFERS) on queries from this user/database",
				"Check for missing indexes, sequential scans on large tables",
				"Look for lock waits or I/O saturation causing query delays",
				"Review pg_stat_statements for specific slow query hashes",
			},
		})
	} else if top.AvgLatencyMS > 500 {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Slow Query Pattern Detected",
			Detail:   fmt.Sprintf("Top contributor '%s'@'%s': avg %.0fms latency", top.User, top.Database, top.AvgLatencyMS),
			Actions: []string{
				"Run EXPLAIN (ANALYZE, BUFFERS) on queries from this user/database",
				"Check for missing indexes on frequently filtered columns",
				"Review pg_stat_statements for specific slow query hashes",
				"Enable Query Insights query sampling for per-query visibility",
			},
		})
	} else if top.AvgLatencyMS > 100 {
		add(Finding{
			Severity: SeverityInfo,
			Title:    "Notable Query Latency",
			Detail:   fmt.Sprintf("Top contributor '%s'@'%s': avg %.0fms", top.User, top.Database, top.AvgLatencyMS),
			Actions: []string{
				"Monitor latency trends in Query Insights dashboard",
				"Review EXPLAIN plans for high-latency queries from this user",
			},
		})
	}

	// Load concentration
	if len(queries) > 1 {
		var totalTime float64
		for _, q := range queries {
			totalTime += q.TotalTimeMS
		}
		if totalTime > 0 {
			topShare := (top.TotalTimeMS / totalTime) * 100
			if topShare > 80 {
				add(Finding{
					Severity: SeverityInfo,
					Title:    "Query Load Highly Concentrated",
					Detail:   fmt.Sprintf("'%s'@'%s' accounts for %.0f%% of total query time in the top-%d", top.User, top.Database, topShare, len(queries)),
					Actions: []string{
						"Optimize this user/database's queries first for maximum impact",
						"Review indexes, query plans, and connection patterns for this user",
					},
				})
			}
		}
	}

	// I/O-bound queries
	if r.QueryPerf.IOTimeUS.P99 > 0 && r.QueryPerf.LatencyUS.P99 > 0 {
		ioRatio := (r.QueryPerf.IOTimeUS.P99 / r.QueryPerf.LatencyUS.P99) * 100
		if ioRatio > 50 {
			add(Finding{
				Severity: SeverityWarning,
				Title:    "Queries I/O-Bound",
				Detail:   fmt.Sprintf("I/O accounts for %.0f%% of P99 query latency — cache may be insufficient", ioRatio),
				Actions: []string{
					"Increase instance memory to improve buffer cache hit ratio",
					"Check pg_statio_user_tables for tables with high heap_blks_read",
					"Add indexes to reduce full-table-scan I/O",
				},
			})
		}
	}
}

func diagConfig(r *CheckResult, add func(Finding)) {
	cfg := r.InstanceConfig

	if !cfg.BackupEnabled {
		add(Finding{
			Severity: SeverityWarning,
			Title:    "Automated Backups Disabled",
			Detail:   "No automated backups configured — data loss risk in case of failure",
			Actions: []string{
				"Enable automated backups in Cloud SQL instance settings",
				"Configure a backup window during low-traffic hours",
				"Enable PITR for point-in-time recovery capability",
			},
		})
	} else if !cfg.PITREnabled {
		add(Finding{
			Severity: SeverityInfo,
			Title:    "Point-in-Time Recovery Not Enabled",
			Detail:   "Backups are on but PITR is disabled — recovery limited to daily backup snapshots",
			Actions: []string{
				"Enable PITR in Cloud SQL settings",
				"PITR allows recovery to any second within the retention window",
			},
		})
	}

	if cfg.AvailabilityType == "ZONAL" {
		add(Finding{
			Severity: SeverityInfo,
			Title:    "No High Availability (Zonal Deployment)",
			Detail:   "Instance is ZONAL — a zone outage causes downtime and potential data loss",
			Actions: []string{
				"Enable Regional (HA) availability in Cloud SQL settings for production",
				"HA adds a hot standby in another zone with automatic failover",
			},
		})
	}

	if cfg.StorageType == "PD_HDD" {
		add(Finding{
			Severity: SeverityInfo,
			Title:    "Using HDD Storage",
			Detail:   "PD-HDD has significantly lower IOPS and higher latency than PD-SSD",
			Actions: []string{
				"Migrate to PD-SSD for better I/O performance",
				"Especially important for write-heavy or latency-sensitive workloads",
			},
		})
	}

	diskPct := r.Resources.Disk.Utilization.Current * 100
	if !cfg.StorageAutoResize && diskPct > 60 && diskPct <= 75 {
		add(Finding{
			Severity: SeverityInfo,
			Title:    "Storage Auto-resize Disabled",
			Detail:   fmt.Sprintf("Disk at %.1f%% with no auto-resize — manual intervention required if disk fills", diskPct),
			Actions: []string{
				"Enable storage auto-resize in Cloud SQL settings",
				"Note: Cloud SQL disk can only grow, not shrink",
			},
		})
	}

	if !cfg.DeletionProtection {
		add(Finding{
			Severity: SeverityInfo,
			Title:    "Deletion Protection Disabled",
			Detail:   "Instance can be accidentally deleted without additional safeguard",
			Actions: []string{
				"Enable deletion protection in Cloud SQL instance settings",
			},
		})
	}

	if !cfg.QueryInsightsEnabled {
		add(Finding{
			Severity: SeverityInfo,
			Title:    "Query Insights Disabled",
			Detail:   "Per-query latency, I/O, and lock time metrics are unavailable",
			Actions: []string{
				"Enable Query Insights in Cloud SQL settings (minimal overhead)",
				"Unlocks per-query observability and the --query-insights flag",
			},
		})
	}
}

// ── Sorting ───────────────────────────────────────────────────────────────────

func sortFindings(fs []Finding) {
	order := map[Severity]int{SeverityCritical: 0, SeverityWarning: 1, SeverityInfo: 2}
	// stable sort preserves order within same severity
	for i := 1; i < len(fs); i++ {
		for j := i; j > 0 && order[fs[j].Severity] < order[fs[j-1].Severity]; j-- {
			fs[j], fs[j-1] = fs[j-1], fs[j]
		}
	}
}

// ── Formatters ────────────────────────────────────────────────────────────────

// FormatDiagnoseTable renders a DiagnoseResult as human-readable text.
func FormatDiagnoseTable(w io.Writer, dr *DiagnoseResult) error {
	fmt.Fprintf(w, "Instance:    %s\n", dr.Instance)
	fmt.Fprintf(w, "Region:      %s\n", dr.Region)
	fmt.Fprintf(w, "Time Window: %s\n\n", dr.TimeWindow)

	// Load summary
	ls := dr.Load
	if ls.AvgTPS > 0 || ls.ReadTuplesPerSec > 0 || ls.WriteTuplesPerSec > 0 || ls.AvgQueryLatencyMS > 0 {
		t := table.NewWriter()
		t.SetOutputMirror(w)
		t.SetStyle(table.StyleLight)
		t.SetTitle("LOAD SUMMARY")
		t.AppendHeader(table.Row{"Metric", "Value"})
		if ls.AvgTPS > 0 {
			t.AppendRow(table.Row{"Avg Transactions/sec", fmt.Sprintf("%.1f", ls.AvgTPS)})
		}
		if ls.ReadTuplesPerSec > 0 {
			t.AppendRow(table.Row{"Read Tuples/sec (P50)", fmt.Sprintf("%.0f", ls.ReadTuplesPerSec)})
		}
		if ls.WriteTuplesPerSec > 0 {
			t.AppendRow(table.Row{"Write Tuples/sec (P50)", fmt.Sprintf("%.0f", ls.WriteTuplesPerSec)})
		}
		if ls.AvgQueryLatencyMS > 0 {
			t.AppendRow(table.Row{"Query Latency P50", fmt.Sprintf("%.1f ms", ls.AvgQueryLatencyMS)})
		}
		if ls.P99QueryLatencyMS > 0 {
			t.AppendRow(table.Row{"Query Latency P99", fmt.Sprintf("%.1f ms", ls.P99QueryLatencyMS)})
		}
		t.Render()
		fmt.Fprintln(w)
	}

	if len(dr.Findings) == 0 {
		fmt.Fprintln(w, "✓  No issues detected.")
		return nil
	}

	fmt.Fprintf(w, "%d issue(s) found:\n\n", len(dr.Findings))

	for _, f := range dr.Findings {
		fmt.Fprintf(w, "%s %-8s  %s\n", severityIcon(f.Severity), string(f.Severity), f.Title)
		fmt.Fprintf(w, "            %s\n", f.Detail)
		for _, action := range f.Actions {
			fmt.Fprintf(w, "            → %s\n", action)
		}
		fmt.Fprintln(w)
	}

	return nil
}

// FormatDiagnoseJSON writes a DiagnoseResult as indented JSON.
func FormatDiagnoseJSON(w io.Writer, dr *DiagnoseResult) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(dr)
}

func severityIcon(s Severity) string {
	switch s {
	case SeverityCritical:
		return "🔴"
	case SeverityWarning:
		return "🟡"
	default:
		return "ℹ️ "
	}
}

func fmtAge(sec int64) string {
	if sec >= 3600 {
		return fmt.Sprintf("%dh%dm", sec/3600, (sec%3600)/60)
	}
	if sec >= 60 {
		return fmt.Sprintf("%dm%ds", sec/60, sec%60)
	}
	return fmt.Sprintf("%ds", sec)
}
