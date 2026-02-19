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

// TablesResult contains BigQuery table analysis results
type TablesResult struct {
	Project         string                `json:"project"`
	Dataset         string                `json:"dataset"`
	Timestamp       time.Time             `json:"timestamp"`
	Tables          []TableInfo           `json:"tables"`
	Recommendations []TableRecommendation `json:"recommendations,omitempty"`
}

// TableInfo contains metadata about a BigQuery table
type TableInfo struct {
	Schema          string    `json:"schema"`
	Name            string    `json:"name"`
	SizeGB          float64   `json:"size_gb"`
	RowCount        int64     `json:"row_count"`
	Created         time.Time `json:"created"`
	LastModified    time.Time `json:"last_modified"`
	Partitioned     bool      `json:"partitioned"`
	PartitionField  string    `json:"partition_field,omitempty"`
	PartitionType   string    `json:"partition_type,omitempty"`
	Clustered       bool      `json:"clustered"`
	ClusterFields   []string  `json:"cluster_fields,omitempty"`
	HasDescription  bool      `json:"has_description"`
	ColumnCount     int       `json:"column_count"`
}

// TableRecommendation contains optimization recommendations for tables
type TableRecommendation struct {
	Table            string  `json:"table"`             // Fully qualified table name (schema.name)
	Severity         string  `json:"severity"`          // "HIGH", "MEDIUM", or "LOW"
	Category         string  `json:"category"`          // e.g., "PARTITIONING", "CLUSTERING", "STORAGE", "SCHEMA"
	Message          string  `json:"message"`
	EstimatedSavings float64 `json:"estimated_savings,omitempty"` // e.g., "~$50/month" or "N/A"
}

// QueriesResult contains BigQuery query analysis results
type QueriesResult struct {
	Project    string       `json:"project"`
	Dataset    string       `json:"dataset,omitempty"`
	TimeWindow string       `json:"time_window"`
	Queries    []QueryInfo  `json:"queries"`
	Summary    QuerySummary `json:"summary"`
}

// QueryInfo contains detailed information about a BigQuery query
type QueryInfo struct {
	JobID           string    `json:"job_id"`
	UserEmail       string    `json:"user_email"`
	QueryText       string    `json:"query_text"`
	BytesProcessed  int64     `json:"bytes_processed"`
	SlotMS          int64     `json:"slot_ms"`
	DurationSeconds float64   `json:"duration_seconds"`
	CacheHit        bool      `json:"cache_hit"`
	CreatedAt       time.Time `json:"created_at"`
	EstimatedCost   float64   `json:"estimated_cost"`
}

// QuerySummary contains aggregate statistics for queries
type QuerySummary struct {
	TotalQueries    int     `json:"total_queries"`
	TotalBytes      int64   `json:"total_bytes"`
	TotalCost       float64 `json:"total_cost"`
	AvgDurationSec  float64 `json:"avg_duration_sec"`
	CacheHitRate    float64 `json:"cache_hit_rate"`
}
