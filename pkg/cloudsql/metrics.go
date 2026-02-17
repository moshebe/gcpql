package cloudsql

// MetricDefinition defines a CloudSQL metric to fetch
type MetricDefinition struct {
	Name       string
	MetricType string
	Category   string
}

// AllMetrics returns all CloudSQL metrics to collect
func AllMetrics() []MetricDefinition {
	return []MetricDefinition{
		// Resources - CPU
		{Name: "cpu_utilization", MetricType: "cloudsql.googleapis.com/database/cpu/utilization", Category: "resources"},
		{Name: "cpu_reserved_cores", MetricType: "cloudsql.googleapis.com/database/cpu/reserved_cores", Category: "resources"},

		// Resources - Memory
		{Name: "memory_utilization", MetricType: "cloudsql.googleapis.com/database/memory/utilization", Category: "resources"},
		{Name: "memory_quota", MetricType: "cloudsql.googleapis.com/database/memory/quota", Category: "resources"},
		{Name: "memory_usage", MetricType: "cloudsql.googleapis.com/database/memory/total_usage", Category: "resources"},

		// Resources - Disk
		{Name: "disk_utilization", MetricType: "cloudsql.googleapis.com/database/disk/utilization", Category: "resources"},
		{Name: "disk_quota", MetricType: "cloudsql.googleapis.com/database/disk/quota", Category: "resources"},
		{Name: "disk_bytes_used", MetricType: "cloudsql.googleapis.com/database/disk/bytes_used", Category: "resources"},
		{Name: "disk_read_ops", MetricType: "cloudsql.googleapis.com/database/disk/read_ops_count", Category: "resources"},
		{Name: "disk_write_ops", MetricType: "cloudsql.googleapis.com/database/disk/write_ops_count", Category: "resources"},

		// Connections
		{Name: "num_backends", MetricType: "cloudsql.googleapis.com/database/postgresql/num_backends", Category: "connections"},

		// Query Performance (requires Query Insights)
		{Name: "query_latencies", MetricType: "cloudsql.googleapis.com/database/postgresql/insights/aggregate/latencies", Category: "query_performance"},
		{Name: "query_execution_time", MetricType: "cloudsql.googleapis.com/database/postgresql/insights/aggregate/execution_time", Category: "query_performance"},
		{Name: "query_io_time", MetricType: "cloudsql.googleapis.com/database/postgresql/insights/aggregate/io_time", Category: "query_performance"},
		{Name: "query_lock_time", MetricType: "cloudsql.googleapis.com/database/postgresql/insights/aggregate/lock_time", Category: "query_performance"},
		{Name: "query_row_count", MetricType: "cloudsql.googleapis.com/database/postgresql/insights/aggregate/row_count", Category: "query_performance"},

		// Database Health
		{Name: "transaction_id_utilization", MetricType: "cloudsql.googleapis.com/database/postgresql/transaction_id_utilization", Category: "database_health"},
		{Name: "transaction_count", MetricType: "cloudsql.googleapis.com/database/postgresql/transaction_count", Category: "database_health"},
		{Name: "autovacuum_count", MetricType: "cloudsql.googleapis.com/database/postgresql/autovacuum_count", Category: "database_health"},
		{Name: "vacuum_count", MetricType: "cloudsql.googleapis.com/database/postgresql/vacuum_count", Category: "database_health"},

		// Temp Data
		{Name: "temp_bytes_written", MetricType: "cloudsql.googleapis.com/database/postgresql/temp_bytes_written", Category: "temp_data"},
		{Name: "temp_files", MetricType: "cloudsql.googleapis.com/database/postgresql/temp_files", Category: "temp_data"},

		// Checkpoints
		{Name: "checkpoint_sync_latency", MetricType: "cloudsql.googleapis.com/database/postgresql/checkpoint_sync_latency", Category: "checkpoints"},
		{Name: "checkpoint_write_latency", MetricType: "cloudsql.googleapis.com/database/postgresql/checkpoint_write_latency", Category: "checkpoints"},

		// Replication
		{Name: "replica_lag_bytes", MetricType: "cloudsql.googleapis.com/database/replication/replica_lag", Category: "replication"},
		{Name: "replica_lag_seconds", MetricType: "cloudsql.googleapis.com/database/replication/replica_lag_seconds", Category: "replication"},

		// Network
		{Name: "network_received_bytes", MetricType: "cloudsql.googleapis.com/database/network/received_bytes_count", Category: "network"},
		{Name: "network_sent_bytes", MetricType: "cloudsql.googleapis.com/database/network/sent_bytes_count", Category: "network"},
	}
}
