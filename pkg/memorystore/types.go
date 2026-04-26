package memorystore

import "time"

// Severity levels for health status.
type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityWarning  Severity = "WARNING"
	SeverityInfo     Severity = "INFO"
)

// InstanceSnapshot is one row in the check output.
type InstanceSnapshot struct {
	Name            string   `json:"name"`
	MemoryUsage     float64  `json:"memory_usage_ratio"`
	ConnectedClients int64   `json:"connected_clients"`
	CacheHitRatio   float64  `json:"cache_hit_ratio"`
	KeyCount        int64    `json:"key_count"`
	EvictedKeys     int64    `json:"evicted_keys"`
	UptimeSec       float64  `json:"uptime_sec"`
	Status          Severity `json:"status"`
	StatusReason    string   `json:"status_reason"`
}

// CheckResult is the full output of `memorystore check`.
type CheckResult struct {
	Project   string             `json:"project"`
	Timestamp time.Time          `json:"timestamp"`
	Instances []InstanceSnapshot `json:"instances"`
	Metadata  Metadata           `json:"metadata"`
}

// Metadata tracks collection quality.
type Metadata struct {
	MetricsCollected     int      `json:"metrics_collected"`
	MetricsNoData        int      `json:"metrics_no_data"`
	MetricsUnavailable   []string `json:"metrics_unavailable,omitempty"`
	CollectionDurationMS int64    `json:"collection_duration_ms"`
}
