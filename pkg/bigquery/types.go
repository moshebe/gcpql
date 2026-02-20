package bigquery

import "time"

// CheckResult contains BigQuery health diagnostics
type CheckResult struct {
	Project    string           `json:"project"`
	Dataset    string           `json:"dataset,omitempty"`
	Timestamp  time.Time        `json:"timestamp"`
	Slots      SlotMetrics      `json:"slots"`
	Cost       CostMetrics      `json:"cost"`
	Jobs       JobsSummary      `json:"jobs"`
	TopQueries []ExpensiveQuery `json:"top_queries,omitempty"`
	Metadata   Metadata         `json:"metadata"`
}

// JobsSummary contains aggregate query statistics from INFORMATION_SCHEMA
type JobsSummary struct {
	TotalJobs    int     `json:"total_jobs"`
	FailedJobs   int     `json:"failed_jobs"`
	CacheHits    int     `json:"cache_hits"`
	CacheHitRate float64 `json:"cache_hit_rate"`
	TotalBytes   int64   `json:"total_bytes"`
	TotalCost    float64 `json:"total_cost"`
}

// Metadata contains diagnostic metadata about the check operation
type Metadata struct {
	MetricsCollected     int      `json:"metrics_collected"`
	MetricsNoData        int      `json:"metrics_no_data"`
	MetricsUnavailable   []string `json:"metrics_unavailable,omitempty"`
	CollectionDurationMS int64    `json:"collection_duration_ms"`
}

// SlotMetrics contains BigQuery slot utilization metrics
type SlotMetrics struct {
	Allocated       int64   `json:"allocated"`
	Current         int64   `json:"current"`
	Peak            int64   `json:"peak"`
	Utilization     float64 `json:"utilization"`
	QueriesInFlight int     `json:"queries_in_flight"`
	QueriesQueued   int     `json:"queries_queued"`
}

// CostMetrics contains BigQuery cost-related metrics
type CostMetrics struct {
	StorageGB         float64 `json:"storage_gb"`
	StorageCostDaily  float64 `json:"storage_cost_daily"`
	BytesScannedTotal int64   `json:"bytes_scanned_total"`
	EstimatedCost     float64 `json:"estimated_cost"`
}

// ExpensiveQuery represents a high-cost query
type ExpensiveQuery struct {
	JobID           string    `json:"job_id"`
	UserEmail       string    `json:"user_email"`
	Query           string    `json:"query"`
	SlotMS          int64     `json:"slot_ms,omitempty"`
	BytesProcessed  int64     `json:"bytes_processed"`
	DurationSeconds float64   `json:"duration_seconds"`
	CacheHit        bool      `json:"cache_hit"`
	StartTime       time.Time `json:"start_time,omitempty"`
	EstimatedCost   float64   `json:"estimated_cost"`
}
