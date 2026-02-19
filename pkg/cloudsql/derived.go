package cloudsql

import "time"

// computeDerivedMetrics calculates derived insights from collected metrics
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

func computeCacheHitRatio(result *CheckResult) {
	hit := result.Cache.BlocksHit.Current
	read := result.Cache.BlocksRead.Current
	total := hit + read

	if total > 0 {
		result.Cache.HitRatio = (hit / total) * 100
		result.DerivedInsights.CacheHitRatio = result.Cache.HitRatio
	}
}

func computeConnectionUtilization(result *CheckResult) {
	if result.Connections.MaxConnections > 0 {
		// Use P99 to catch spikes
		result.DerivedInsights.ConnectionUtilizationPct =
			(result.Connections.Count.P99 / float64(result.Connections.MaxConnections)) * 100
	}
}

func detectLongTransactions(result *CheckResult) {
	age := result.DBHealth.OldestTransactionAgeSec
	result.DerivedInsights.OldestTransactionAgeSec = age
	result.DerivedInsights.LongTransactionDetected = age > 300 // 5 minutes
}

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

func computeTempDataRate(result *CheckResult, timeWindow time.Duration) {
	if timeWindow.Seconds() > 0 {
		bytesWritten := float64(result.TempData.BytesWritten)
		seconds := timeWindow.Seconds()
		result.DerivedInsights.TempDataRateMBPerSec = bytesWritten / seconds / 1024 / 1024
	}
}

func computeAutovacuumFrequency(result *CheckResult, timeWindow time.Duration) {
	if timeWindow.Hours() > 0 {
		count := float64(result.DBHealth.AutovacuumCount)
		hours := timeWindow.Hours()
		result.DerivedInsights.AutovacuumFrequencyPerHour = count / hours
	}
}
