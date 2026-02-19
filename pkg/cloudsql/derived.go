package cloudsql

import "time"

const longTransactionThresholdSec = 300 // 5 minutes

// computeDerivedMetrics calculates derived insights from collected metrics.
// It orchestrates all derived metric computations, populating result.DerivedInsights
// with actionable diagnostic data based on raw metrics. Should be called after
// metrics collection is complete.
func computeDerivedMetrics(result *CheckResult, timeWindow time.Duration) {
	// Cache hit ratio
	computeCacheHitRatio(result)

	// Connection utilization
	computeConnectionUtilization(result)

	// Long transaction detection
	detectLongTransactions(result)

	// Read/write ratio
	computeReadWriteRatio(result)

	// Temp data rate
	computeTempDataRate(result, timeWindow)

	// Autovacuum frequency
	computeAutovacuumFrequency(result, timeWindow)
}

// computeCacheHitRatio calculates the percentage of cache hits vs total cache operations.
// Returns 0 if no cache operations occurred. Result stored in both Cache.HitRatio and
// DerivedInsights.CacheHitRatio fields.
func computeCacheHitRatio(result *CheckResult) {
	hit := result.Cache.BlocksHit.Current
	read := result.Cache.BlocksRead.Current
	total := hit + read

	if total > 0 {
		result.Cache.HitRatio = (hit / total) * 100
		result.DerivedInsights.CacheHitRatio = result.Cache.HitRatio
	}
}

// computeConnectionUtilization calculates the percentage of max connections in use.
// Uses P99 of connection count to detect usage spikes. Returns 0 if MaxConnections
// is not set. Result stored in DerivedInsights.ConnectionUtilizationPct.
func computeConnectionUtilization(result *CheckResult) {
	if result.Connections.MaxConnections > 0 {
		// Use P99 to catch spikes
		result.DerivedInsights.ConnectionUtilizationPct =
			(result.Connections.Count.P99 / float64(result.Connections.MaxConnections)) * 100
	}
}

// detectLongTransactions identifies transactions running longer than 5 minutes.
// Long-running transactions can block autovacuum and cause table bloat. Copies
// OldestTransactionAgeSec to DerivedInsights and sets LongTransactionDetected flag.
func detectLongTransactions(result *CheckResult) {
	age := result.DBHealth.OldestTransactionAgeSec
	result.DerivedInsights.OldestTransactionAgeSec = age
	result.DerivedInsights.LongTransactionDetected = age > longTransactionThresholdSec
}

// computeReadWriteRatio calculates the ratio of read operations to write operations.
// Uses TuplesReturned as reads and sum of inserts/updates/deletes as writes. Returns 0
// if no writes occurred. Result stored in both Throughput.ReadWriteRatio and
// DerivedInsights.ReadWriteRatio.
func computeReadWriteRatio(result *CheckResult) {
	reads := result.Throughput.TuplesReturned.Current
	writes := result.Throughput.TuplesInserted.Current +
		result.Throughput.TuplesUpdated.Current +
		result.Throughput.TuplesDeleted.Current

	if writes > 0 {
		ratio := reads / writes
		result.Throughput.ReadWriteRatio = ratio
		result.DerivedInsights.ReadWriteRatio = ratio
	}
}

// computeTempDataRate calculates the rate of temporary data writes in MB/sec.
// High temp data rates indicate work_mem pressure causing disk spills. Returns 0
// if timeWindow is 0. Result stored in DerivedInsights.TempDataRateMBPerSec.
func computeTempDataRate(result *CheckResult, timeWindow time.Duration) {
	if timeWindow.Seconds() > 0 {
		bytesWritten := float64(result.TempData.BytesWritten)
		seconds := timeWindow.Seconds()
		result.DerivedInsights.TempDataRateMBPerSec = bytesWritten / seconds / 1024 / 1024
	}
}

// computeAutovacuumFrequency calculates the number of autovacuum runs per hour.
// Helps identify if autovacuum tuning is needed. Returns 0 if timeWindow is 0.
// Result stored in DerivedInsights.AutovacuumFrequencyPerHour.
func computeAutovacuumFrequency(result *CheckResult, timeWindow time.Duration) {
	if timeWindow.Hours() > 0 {
		count := float64(result.DBHealth.AutovacuumCount)
		hours := timeWindow.Hours()
		result.DerivedInsights.AutovacuumFrequencyPerHour = count / hours
	}
}
