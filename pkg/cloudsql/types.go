package cloudsql

import "time"

// CheckResult represents the complete health check output
type CheckResult struct {
	Instance        string            `json:"instance"`
	Project         string            `json:"project"`
	Region          string            `json:"region"`
	DatabaseVersion string            `json:"database_version,omitempty"`
	Timestamp       time.Time         `json:"timestamp"`
	TimeWindow      string            `json:"timeWindow"`
	InstanceSize    InstanceSize      `json:"instance_size"`
	Resources       Resources         `json:"resources"`
	Connections     Connections       `json:"connections"`
	QueryPerf       QueryPerf         `json:"query_performance"`
	DBHealth        DBHealth          `json:"database_health"`
	TempData        TempData          `json:"temp_data"`
	Checkpoints     Checkpoints       `json:"checkpoints"`
	Replication     Replication       `json:"replication"`
	Network         Network           `json:"network"`
	InstanceConfig  InstanceConfig    `json:"instance_config"`
	Recommendations Recommendations   `json:"recommendations"`
	QueryInsights   QueryInsights     `json:"query_insights"`
	Cache           CacheMetrics      `json:"cache"`
	Throughput      ThroughputMetrics `json:"throughput"`
	DerivedInsights DerivedInsights   `json:"derived_insights"`
	Metadata        Metadata          `json:"metadata"`
}

// InstanceSize represents vCPU and memory
type InstanceSize struct {
	VCPU     int     `json:"vcpu"`
	MemoryGB float64 `json:"memory_gb"`
}

// Stats represents statistical aggregates for a metric
type Stats struct {
	Current float64 `json:"current,omitempty"`
	P50     float64 `json:"p50,omitempty"`
	P99     float64 `json:"p99,omitempty"`
	Max     float64 `json:"max,omitempty"`
	Min     float64 `json:"min,omitempty"`
	Avg     float64 `json:"avg,omitempty"`
	Unit    string  `json:"unit"`
}

// Resources represents CPU, memory, and disk metrics
type Resources struct {
	CPU    CPUMetrics    `json:"cpu"`
	Memory MemoryMetrics `json:"memory"`
	Disk   DiskMetrics   `json:"disk"`
}

// CPUMetrics represents CPU utilization and cores
type CPUMetrics struct {
	Utilization   Stats `json:"utilization"`
	ReservedCores int   `json:"reserved_cores"`
}

// MemoryMetrics represents memory usage
type MemoryMetrics struct {
	Utilization Stats `json:"utilization"`
	QuotaBytes  int64 `json:"quota_bytes"`
	UsageBytes  int64 `json:"usage_bytes"`
}

// DiskMetrics represents disk usage and I/O
type DiskMetrics struct {
	Utilization Stats `json:"utilization"`
	QuotaBytes  int64 `json:"quota_bytes"`
	BytesUsed   int64 `json:"bytes_used"`
	ReadOps     Stats `json:"read_ops"`
	WriteOps    Stats `json:"write_ops"`
}

// CacheMetrics represents cache performance
type CacheMetrics struct {
	HitRatio          float64 `json:"hit_ratio_pct"`
	BlocksHit         Stats   `json:"blocks_hit"`
	BlocksRead        Stats   `json:"blocks_read"`
	TempBlocksRead    Stats   `json:"temp_blocks_read"`
	TempBlocksWritten Stats   `json:"temp_blocks_written"`
}

// ThroughputMetrics represents tuple throughput rates
type ThroughputMetrics struct {
	TuplesReturned Stats   `json:"tuples_returned"`
	TuplesFetched  Stats   `json:"tuples_fetched"`
	TuplesInserted Stats   `json:"tuples_inserted"`
	TuplesUpdated  Stats   `json:"tuples_updated"`
	TuplesDeleted  Stats   `json:"tuples_deleted"`
	ReadWriteRatio float64 `json:"read_write_ratio"`
}

// DerivedInsights represents computed diagnostic insights
type DerivedInsights struct {
	CacheHitRatio              float64 `json:"cache_hit_ratio_pct"`
	ConnectionUtilizationPct   float64 `json:"connection_utilization_pct"`
	LongTransactionDetected    bool    `json:"long_transaction_detected"`
	OldestTransactionAgeSec    int64   `json:"oldest_transaction_age_sec"`
	ReadWriteRatio             float64 `json:"read_write_ratio"`
	TempDataRateMBPerSec       float64 `json:"temp_data_rate_mb_per_sec"`
	AutovacuumFrequencyPerHour float64 `json:"autovacuum_frequency_per_hour"`
}

// Connections represents connection metrics
type Connections struct {
	Count          Stats               `json:"count"`
	MaxConnections int                 `json:"max_connections"`
	ByStatus       ConnectionsByStatus `json:"by_status"`
}

// ConnectionsByStatus breaks down connections by state
type ConnectionsByStatus struct {
	Active                   int `json:"active"`
	Idle                     int `json:"idle"`
	IdleInTransaction        int `json:"idle_in_transaction"`
	IdleInTransactionAborted int `json:"idle_in_transaction_aborted"`
}

// QueryPerf represents query performance metrics
type QueryPerf struct {
	Available      bool  `json:"available"`
	LatencyUS      Stats `json:"latency_us,omitempty"`
	DatabaseLoadUS Stats `json:"database_load_us,omitempty"`
	IOTimeUS       Stats `json:"io_time_us,omitempty"`
	LockTimeUS     Stats `json:"lock_time_us,omitempty"`
	RowsProcessed  int64 `json:"rows_processed,omitempty"`
}

// DBHealth represents database health metrics
type DBHealth struct {
	TransactionIDUtilization Stats `json:"transaction_id_utilization"`
	TransactionCount         int64 `json:"transaction_count"`
	OldestTransactionAgeSec  int64 `json:"oldest_transaction_age_seconds"`
	DeadlockCount            int   `json:"deadlock_count"`
	AutovacuumCount          int   `json:"autovacuum_count"`
	AnalyzeCount             int   `json:"analyze_count"`
	VacuumCount              int   `json:"vacuum_count"`
}

// TempData represents temp data metrics
type TempData struct {
	BytesWritten int64  `json:"bytes_written"`
	FilesCreated int    `json:"files_created"`
	Unit         string `json:"unit"`
}

// Checkpoints represents checkpoint performance
type Checkpoints struct {
	SyncLatencyMS  Stats `json:"sync_latency_ms"`
	WriteLatencyMS Stats `json:"write_latency_ms"`
}

// Replication represents replication lag
type Replication struct {
	ReplicaLagBytes   Stats `json:"replica_lag_bytes"`
	ReplicaLagSeconds Stats `json:"replica_lag_seconds"`
}

// Network represents network throughput
type Network struct {
	IngressBytes int64  `json:"ingress_bytes"`
	EgressBytes  int64  `json:"egress_bytes"`
	Unit         string `json:"unit"`
}

// DBFlag represents a database flag name/value pair
type DBFlag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// InstanceConfig holds enriched config from the Cloud SQL Admin API
type InstanceConfig struct {
	Labels               map[string]string `json:"labels,omitempty"`
	AvailabilityType     string            `json:"availability_type,omitempty"`        // "ZONAL" | "REGIONAL"
	BackupEnabled        bool              `json:"backup_enabled"`
	BackupStartTime      string            `json:"backup_start_time,omitempty"`
	PITREnabled          bool              `json:"pitr_enabled"`
	StorageType          string            `json:"storage_type,omitempty"`             // "PD_SSD" | "PD_HDD"
	StorageAutoResize    bool              `json:"storage_auto_resize"`
	StorageAutoResizeGB  int64             `json:"storage_auto_resize_limit_gb,omitempty"`
	DatabaseFlags        []DBFlag          `json:"database_flags,omitempty"`
	QueryInsightsEnabled bool              `json:"query_insights_enabled"`
	DeletionProtection   bool              `json:"deletion_protection"`
	State                string            `json:"state,omitempty"`
	ConnectionName       string            `json:"connection_name,omitempty"`
}

// Recommendation is a single Cloud Recommender suggestion
type Recommendation struct {
	Description string `json:"description"`
	Impact      string `json:"impact"` // "HIGH" | "MEDIUM" | "LOW"
	State       string `json:"state"`  // "ACTIVE" | "DISMISSED"
}

// Recommendations holds Cloud Recommender results
type Recommendations struct {
	Available bool             `json:"available"`
	Items     []Recommendation `json:"items,omitempty"`
}

// TopQuery is one row in the Query Insights top-queries list
type TopQuery struct {
	QueryText    string  `json:"query_text,omitempty"`
	QueryHash    string  `json:"query_hash,omitempty"`
	// SampleCount is the number of monitoring sample intervals that contained
	// data for this query, not the number of SQL executions.
	SampleCount     int64   `json:"sample_count"`
	AvgLatencyMS    float64 `json:"avg_latency_ms"`
	TotalTimeMS     float64 `json:"total_time_ms"`
	AvgRowsReturned float64 `json:"avg_rows_returned"`
}

// QueryInsights holds top-N query data (only populated with --query-insights)
type QueryInsights struct {
	Available  bool       `json:"available"`
	TopQueries []TopQuery `json:"top_queries,omitempty"`
}

// Metadata represents collection metadata
type Metadata struct {
	MetricsCollected     int      `json:"metrics_collected"`
	MetricsNoData        int      `json:"metrics_no_data"`
	MetricsUnavailable   []string `json:"metrics_unavailable"`
	CollectionDurationMS int64    `json:"collection_duration_ms"`
}
