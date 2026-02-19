package cloudsql

import (
	"testing"
	"time"
)

func TestComputeCacheHitRatio(t *testing.T) {
	tests := []struct {
		name        string
		blocksHit   float64
		blocksRead  float64
		expectedPct float64
	}{
		{
			name:        "high cache hit ratio",
			blocksHit:   9500,
			blocksRead:  500,
			expectedPct: 95.0,
		},
		{
			name:        "low cache hit ratio",
			blocksHit:   3000,
			blocksRead:  7000,
			expectedPct: 30.0,
		},
		{
			name:        "zero blocks",
			blocksHit:   0,
			blocksRead:  0,
			expectedPct: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &CheckResult{
				Cache: CacheMetrics{
					BlocksHit:  Stats{Current: tt.blocksHit},
					BlocksRead: Stats{Current: tt.blocksRead},
				},
			}

			computeCacheHitRatio(result)

			if result.Cache.HitRatio != tt.expectedPct {
				t.Errorf("Cache.HitRatio: expected %v, got %v", tt.expectedPct, result.Cache.HitRatio)
			}
			if result.DerivedInsights.CacheHitRatio != tt.expectedPct {
				t.Errorf("DerivedInsights.CacheHitRatio: expected %v, got %v", tt.expectedPct, result.DerivedInsights.CacheHitRatio)
			}
		})
	}
}

func TestComputeConnectionUtilization(t *testing.T) {
	result := &CheckResult{
		Connections: Connections{
			MaxConnections: 100,
			Count:          Stats{P99: 75.0},
		},
	}

	computeConnectionUtilization(result)

	expected := 75.0
	if result.DerivedInsights.ConnectionUtilizationPct != expected {
		t.Errorf("expected %v, got %v", expected, result.DerivedInsights.ConnectionUtilizationPct)
	}
}

func TestDetectLongTransactions(t *testing.T) {
	tests := []struct {
		name             string
		transactionAge   int64
		expectedDetected bool
	}{
		{
			name:             "short transaction",
			transactionAge:   60,
			expectedDetected: false,
		},
		{
			name:             "long transaction",
			transactionAge:   600,
			expectedDetected: true,
		},
		{
			name:             "exactly at threshold",
			transactionAge:   300,
			expectedDetected: false,
		},
		{
			name:             "just over threshold",
			transactionAge:   301,
			expectedDetected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &CheckResult{
				DBHealth: DBHealth{
					OldestTransactionAgeSec: tt.transactionAge,
				},
			}

			detectLongTransactions(result)

			if result.DerivedInsights.OldestTransactionAgeSec != tt.transactionAge {
				t.Errorf("OldestTransactionAgeSec: expected %v, got %v", tt.transactionAge, result.DerivedInsights.OldestTransactionAgeSec)
			}
			if result.DerivedInsights.LongTransactionDetected != tt.expectedDetected {
				t.Errorf("LongTransactionDetected: expected %v, got %v", tt.expectedDetected, result.DerivedInsights.LongTransactionDetected)
			}
		})
	}
}

func TestComputeReadWriteRatio(t *testing.T) {
	result := &CheckResult{
		Throughput: ThroughputMetrics{
			TuplesReturned: Stats{Current: 10000},
			TuplesInserted: Stats{Current: 100},
			TuplesUpdated:  Stats{Current: 50},
			TuplesDeleted:  Stats{Current: 50},
		},
	}

	computeReadWriteRatio(result)

	expected := 50.0 // 10000 / (100 + 50 + 50)
	if result.Throughput.ReadWriteRatio != expected {
		t.Errorf("Throughput.ReadWriteRatio: expected %v, got %v", expected, result.Throughput.ReadWriteRatio)
	}
	if result.DerivedInsights.ReadWriteRatio != expected {
		t.Errorf("DerivedInsights.ReadWriteRatio: expected %v, got %v", expected, result.DerivedInsights.ReadWriteRatio)
	}
}

func TestComputeTempDataRate(t *testing.T) {
	result := &CheckResult{
		TempData: TempData{
			BytesWritten: 10485760, // 10 MB
		},
	}
	timeWindow := 10 * time.Second

	computeTempDataRate(result, timeWindow)

	expected := 1.0 // 10 MB / 10 seconds = 1 MB/sec
	if result.DerivedInsights.TempDataRateMBPerSec != expected {
		t.Errorf("expected %v, got %v", expected, result.DerivedInsights.TempDataRateMBPerSec)
	}
}

func TestComputeAutovacuumFrequency(t *testing.T) {
	result := &CheckResult{
		DBHealth: DBHealth{
			AutovacuumCount: 6,
		},
	}
	timeWindow := 2 * time.Hour

	computeAutovacuumFrequency(result, timeWindow)

	expected := 3.0 // 6 runs / 2 hours = 3 per hour
	if result.DerivedInsights.AutovacuumFrequencyPerHour != expected {
		t.Errorf("expected %v, got %v", expected, result.DerivedInsights.AutovacuumFrequencyPerHour)
	}
}

func TestComputeDerivedMetrics(t *testing.T) {
	result := &CheckResult{
		Cache: CacheMetrics{
			BlocksHit:  Stats{Current: 9000},
			BlocksRead: Stats{Current: 1000},
		},
		Connections: Connections{
			MaxConnections: 100,
			Count:          Stats{P99: 80.0},
		},
		DBHealth: DBHealth{
			OldestTransactionAgeSec: 400,
			AutovacuumCount:         12,
		},
		Throughput: ThroughputMetrics{
			TuplesReturned: Stats{Current: 5000},
			TuplesInserted: Stats{Current: 100},
			TuplesUpdated:  Stats{Current: 50},
			TuplesDeleted:  Stats{Current: 50},
		},
		TempData: TempData{
			BytesWritten: 20971520, // 20 MB
		},
	}
	timeWindow := 1 * time.Hour

	computeDerivedMetrics(result, timeWindow)

	// Verify cache hit ratio
	expectedCacheHit := 90.0
	if result.DerivedInsights.CacheHitRatio != expectedCacheHit {
		t.Errorf("CacheHitRatio: expected %v, got %v", expectedCacheHit, result.DerivedInsights.CacheHitRatio)
	}

	// Verify connection utilization
	expectedConnUtil := 80.0
	if result.DerivedInsights.ConnectionUtilizationPct != expectedConnUtil {
		t.Errorf("ConnectionUtilizationPct: expected %v, got %v", expectedConnUtil, result.DerivedInsights.ConnectionUtilizationPct)
	}

	// Verify long transaction detection
	if !result.DerivedInsights.LongTransactionDetected {
		t.Error("LongTransactionDetected: expected true, got false")
	}
	if result.DerivedInsights.OldestTransactionAgeSec != 400 {
		t.Errorf("OldestTransactionAgeSec: expected 400, got %v", result.DerivedInsights.OldestTransactionAgeSec)
	}

	// Verify read/write ratio
	expectedRWRatio := 25.0 // 5000 / (100 + 50 + 50)
	if result.DerivedInsights.ReadWriteRatio != expectedRWRatio {
		t.Errorf("ReadWriteRatio: expected %v, got %v", expectedRWRatio, result.DerivedInsights.ReadWriteRatio)
	}

	// Verify temp data rate
	// 20971520 bytes / 3600 seconds / 1024 / 1024 = 0.005555555555555556 MB/sec
	expectedTempRate := 0.005555555555555556
	if result.DerivedInsights.TempDataRateMBPerSec != expectedTempRate {
		t.Errorf("TempDataRateMBPerSec: expected %v, got %v", expectedTempRate, result.DerivedInsights.TempDataRateMBPerSec)
	}

	// Verify autovacuum frequency
	expectedAutoVacFreq := 12.0 // 12 runs / 1 hour
	if result.DerivedInsights.AutovacuumFrequencyPerHour != expectedAutoVacFreq {
		t.Errorf("AutovacuumFrequencyPerHour: expected %v, got %v", expectedAutoVacFreq, result.DerivedInsights.AutovacuumFrequencyPerHour)
	}
}
