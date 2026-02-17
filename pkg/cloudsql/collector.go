package cloudsql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moshebeladev/gcp-metrics/pkg/monitoring"
)

// Collector fetches CloudSQL metrics
type Collector struct {
	client *monitoring.Client
}

// NewCollector creates a new CloudSQL metrics collector
func NewCollector(client *monitoring.Client) *Collector {
	return &Collector{client: client}
}

// ParseInstanceID parses instance ID in various formats
func ParseInstanceID(instanceID, fallbackProject string) (project, instance string, err error) {
	parts := strings.Split(instanceID, ":")

	switch len(parts) {
	case 1:
		// Short form: "my-instance"
		if fallbackProject == "" {
			return "", "", fmt.Errorf("project required: use --project or provide full instance ID")
		}
		return fallbackProject, parts[0], nil

	case 2:
		// Full form: "my-project:my-instance"
		return parts[0], parts[1], nil

	case 3:
		// Database ID format: "my-project:region:my-instance"
		return parts[0], parts[2], nil

	default:
		return "", "", fmt.Errorf("invalid instance ID format: %s", instanceID)
	}
}

// CollectMetrics fetches all metrics for an instance
func (c *Collector) CollectMetrics(ctx context.Context, project, instance string, since time.Duration) (*CheckResult, error) {
	startTime := time.Now()
	start := time.Now().Add(-since)
	end := time.Now()

	databaseID := fmt.Sprintf("%s:%s", project, instance)

	result := &CheckResult{
		Instance:   databaseID,
		Project:    project,
		Timestamp:  end,
		TimeWindow: formatDuration(since),
		Metadata: Metadata{
			MetricsUnavailable: []string{},
		},
	}

	// Fetch all metrics in parallel
	metrics := AllMetrics()
	type metricResult struct {
		name   string
		points []float64
		err    error
	}

	results := make(chan metricResult, len(metrics))

	for _, metric := range metrics {
		go func(m MetricDefinition) {
			query := fmt.Sprintf(`{__name__="%s",resource.database_id="%s"}`, m.MetricType, databaseID)

			resp, err := c.client.QueryTimeSeries(ctx, monitoring.QueryTimeSeriesRequest{
				Project:   project,
				Query:     query,
				StartTime: start,
				EndTime:   end,
			})

			if err != nil {
				results <- metricResult{name: m.Name, err: err}
				return
			}

			// Extract points from Prometheus response format
			var points []float64
			for _, ts := range resp.TimeSeries {
				// Each ts is a map[string]interface{} with "value" key
				if tsMap, ok := ts.(map[string]interface{}); ok {
					// PromQL instant query returns: "value": [timestamp, "value_string"]
					if value, ok := tsMap["value"].([]interface{}); ok && len(value) >= 2 {
						if valStr, ok := value[1].(string); ok {
							var val float64
							fmt.Sscanf(valStr, "%f", &val)
							points = append(points, val)
						}
					}
				}
			}

			results <- metricResult{name: m.Name, points: points}
		}(metric)
	}

	// Collect results
	metricData := make(map[string][]float64)
	var unavailable []string

	for i := 0; i < len(metrics); i++ {
		res := <-results
		if res.err != nil {
			unavailable = append(unavailable, res.name)
			continue
		}
		metricData[res.name] = res.points
	}

	// Populate result structure
	c.populateResult(result, metricData)
	result.Metadata.MetricsUnavailable = unavailable
	result.Metadata.MetricsCollected = len(metrics) - len(unavailable)
	result.Metadata.CollectionDurationMS = time.Since(startTime).Milliseconds()

	return result, nil
}

func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.0fh", d.Hours())
	}
	return fmt.Sprintf("%.0fd", d.Hours()/24)
}

// populateResult fills the CheckResult from collected metric data
func (c *Collector) populateResult(result *CheckResult, data map[string][]float64) {
	// CPU
	if points, ok := data["cpu_utilization"]; ok && len(points) > 0 {
		result.Resources.CPU.Utilization = CalculateStats(points, "percent")
	}
	if points, ok := data["cpu_reserved_cores"]; ok && len(points) > 0 {
		result.Resources.CPU.ReservedCores = int(points[len(points)-1])
		result.InstanceSize.VCPU = result.Resources.CPU.ReservedCores
	}

	// Memory
	if points, ok := data["memory_utilization"]; ok && len(points) > 0 {
		result.Resources.Memory.Utilization = CalculateStats(points, "percent")
	}
	if points, ok := data["memory_quota"]; ok && len(points) > 0 {
		result.Resources.Memory.QuotaBytes = int64(points[len(points)-1])
		result.InstanceSize.MemoryGB = float64(result.Resources.Memory.QuotaBytes) / 1e9
	}
	if points, ok := data["memory_usage"]; ok && len(points) > 0 {
		result.Resources.Memory.UsageBytes = int64(points[len(points)-1])
	}

	// Disk
	if points, ok := data["disk_utilization"]; ok && len(points) > 0 {
		result.Resources.Disk.Utilization = CalculateStats(points, "percent")
	}
	if points, ok := data["disk_quota"]; ok && len(points) > 0 {
		result.Resources.Disk.QuotaBytes = int64(points[len(points)-1])
	}
	if points, ok := data["disk_bytes_used"]; ok && len(points) > 0 {
		result.Resources.Disk.BytesUsed = int64(points[len(points)-1])
	}
	if points, ok := data["disk_read_ops"]; ok && len(points) > 0 {
		result.Resources.Disk.ReadOps = CalculateStats(points, "ops/sec")
	}
	if points, ok := data["disk_write_ops"]; ok && len(points) > 0 {
		result.Resources.Disk.WriteOps = CalculateStats(points, "ops/sec")
	}

	// Connections
	if points, ok := data["num_backends"]; ok && len(points) > 0 {
		result.Connections.Count = CalculateStats(points, "")
		result.Connections.MaxConnections = 100 // TODO: Get from config
	}

	// Query Performance
	hasQueryMetrics := false
	if points, ok := data["query_latencies"]; ok && len(points) > 0 {
		result.QueryPerf.LatencyUS = CalculateStats(points, "microseconds")
		hasQueryMetrics = true
	}
	if points, ok := data["query_execution_time"]; ok && len(points) > 0 {
		result.QueryPerf.DatabaseLoadUS = CalculateStats(points, "microseconds")
		hasQueryMetrics = true
	}
	if points, ok := data["query_io_time"]; ok && len(points) > 0 {
		result.QueryPerf.IOTimeUS = CalculateStats(points, "microseconds")
		hasQueryMetrics = true
	}
	if points, ok := data["query_lock_time"]; ok && len(points) > 0 {
		result.QueryPerf.LockTimeUS = CalculateStats(points, "microseconds")
		hasQueryMetrics = true
	}
	if points, ok := data["query_row_count"]; ok && len(points) > 0 {
		var total int64
		for _, p := range points {
			total += int64(p)
		}
		result.QueryPerf.RowsProcessed = total
		hasQueryMetrics = true
	}
	result.QueryPerf.Available = hasQueryMetrics

	// Database Health
	if points, ok := data["transaction_id_utilization"]; ok && len(points) > 0 {
		result.DBHealth.TransactionIDUtilization = CalculateStats(points, "percent")
	}
	if points, ok := data["transaction_count"]; ok && len(points) > 0 {
		var total int64
		for _, p := range points {
			total += int64(p)
		}
		result.DBHealth.TransactionCount = total
	}
	if points, ok := data["autovacuum_count"]; ok && len(points) > 0 {
		result.DBHealth.AutovacuumCount = int(points[len(points)-1])
	}
	if points, ok := data["vacuum_count"]; ok && len(points) > 0 {
		result.DBHealth.VacuumCount = int(points[len(points)-1])
	}

	// Temp Data
	if points, ok := data["temp_bytes_written"]; ok && len(points) > 0 {
		var total int64
		for _, p := range points {
			total += int64(p)
		}
		result.TempData.BytesWritten = total
		result.TempData.Unit = "bytes"
	}
	if points, ok := data["temp_files"]; ok && len(points) > 0 {
		result.TempData.FilesCreated = int(points[len(points)-1])
	}

	// Checkpoints
	if points, ok := data["checkpoint_sync_latency"]; ok && len(points) > 0 {
		result.Checkpoints.SyncLatencyMS = CalculateStats(points, "ms")
	}
	if points, ok := data["checkpoint_write_latency"]; ok && len(points) > 0 {
		result.Checkpoints.WriteLatencyMS = CalculateStats(points, "ms")
	}

	// Replication
	if points, ok := data["replica_lag_bytes"]; ok && len(points) > 0 {
		result.Replication.ReplicaLagBytes = CalculateStats(points, "bytes")
	}
	if points, ok := data["replica_lag_seconds"]; ok && len(points) > 0 {
		result.Replication.ReplicaLagSeconds = CalculateStats(points, "seconds")
	}

	// Network
	if points, ok := data["network_received_bytes"]; ok && len(points) > 0 {
		var total int64
		for _, p := range points {
			total += int64(p)
		}
		result.Network.IngressBytes = total
		result.Network.Unit = "bytes"
	}
	if points, ok := data["network_sent_bytes"]; ok && len(points) > 0 {
		var total int64
		for _, p := range points {
			total += int64(p)
		}
		result.Network.EgressBytes = total
	}
}
