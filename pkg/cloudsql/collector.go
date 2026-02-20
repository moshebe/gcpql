package cloudsql

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moshebe/gcpql/pkg/monitoring"
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
func (c *Collector) CollectMetrics(ctx context.Context, project, instance string, since time.Duration, queryInsights bool) (*CheckResult, error) {
	now := time.Now()
	startTime := now
	start := now.Add(-since)
	end := now

	// Fetch instance metadata from Cloud SQL Admin API first.
	// This gives us region, database version, and authoritative max_connections.
	instanceInfo, instanceCfg, err := FetchInstanceInfo(ctx, c.client.HTTPClient(), project, instance)
	if err != nil {
		return nil, fmt.Errorf("cloud SQL admin API: %w", err)
	}

	databaseID := fmt.Sprintf("%s:%s", project, instance)

	result := &CheckResult{
		Instance:        databaseID,
		Project:         project,
		Region:          instanceInfo.Region,
		DatabaseVersion: instanceInfo.DatabaseVersion,
		Timestamp:       end,
		TimeWindow:      formatDuration(since),
		Metadata: Metadata{
			MetricsUnavailable: []string{},
		},
	}
	result.InstanceConfig = instanceCfg

	// Fetch all metrics in parallel
	metrics := AllMetrics()
	type metricResult struct {
		name       string
		points     []float64
		currentSum float64 // sum of last values across all time series
		err        error
	}

	results := make(chan metricResult, len(metrics))

	for _, metric := range metrics {
		go func(m MetricDefinition) {
			query := fmt.Sprintf(`{__name__="%s",database_id="%s"}`, m.MetricType, databaseID)

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

			// Extract points from Prometheus response format.
			// Many Cloud SQL metrics are per-Postgres-database (multiple time series per instance).
			// We merge all series into one points slice for P50/P99 stats, and separately
			// track the sum of each series' last value for the accurate "current" reading.
			var points []float64
			var currentSum float64
			for _, ts := range resp.TimeSeries {
				// Each ts is a map[string]interface{} with "values" key (range query)
				if tsMap, ok := ts.(map[string]interface{}); ok {
					// PromQL range query returns: "values": [[timestamp, "value_string"], ...]
					if values, ok := tsMap["values"].([]interface{}); ok {
						for _, v := range values {
							if valueArr, ok := v.([]interface{}); ok && len(valueArr) >= 2 {
								if valStr, ok := valueArr[1].(string); ok {
									if val, err := strconv.ParseFloat(valStr, 64); err == nil {
										points = append(points, val)
									}
								}
							}
						}
						// Sum the last value of this series for per-series current aggregation.
						if n := len(values); n > 0 {
							if valueArr, ok := values[n-1].([]interface{}); ok && len(valueArr) >= 2 {
								if valStr, ok := valueArr[1].(string); ok {
									if val, err := strconv.ParseFloat(valStr, 64); err == nil {
										currentSum += val
									}
								}
							}
						}
					}
				}
			}

			results <- metricResult{name: m.Name, points: points, currentSum: currentSum}
		}(metric)
	}

	// Start enrichment goroutine in parallel with metric collection.
	type enrichResult struct {
		recs Recommendations
		qi   QueryInsights
	}
	enrichCh := make(chan enrichResult, 1)
	go func() {
		// Errors are not propagated — both functions return Available=false on failure.
		recs, _ := FetchRecommendations(ctx, c.client.HTTPClient(), project, instanceInfo.Region)
		var qi QueryInsights
		if queryInsights {
			qi, _ = FetchQueryInsights(ctx, c.client, project, instance, since, 10)
		}
		enrichCh <- enrichResult{recs: recs, qi: qi}
	}()

	// Collect results
	metricData := make(map[string][]float64)
	currentSums := make(map[string]float64)
	var unavailable []string
	noDataCount := 0

	for i := 0; i < len(metrics); i++ {
		res := <-results
		if res.err != nil {
			unavailable = append(unavailable, res.name)
			continue
		}
		if len(res.points) == 0 {
			noDataCount++
		}
		metricData[res.name] = res.points
		currentSums[res.name] = res.currentSum
	}

	// Populate result structure
	c.populateResult(result, metricData, currentSums)

	// Override max_connections with authoritative value from Admin API.
	result.Connections.MaxConnections = instanceInfo.MaxConnections

	// Wait for enrichment and attach results.
	enrich := <-enrichCh
	result.Recommendations = enrich.recs
	result.QueryInsights = enrich.qi

	result.Metadata.MetricsUnavailable = unavailable
	result.Metadata.MetricsCollected = len(metrics) - len(unavailable)
	result.Metadata.MetricsNoData = noDataCount
	result.Metadata.CollectionDurationMS = time.Since(startTime).Milliseconds()

	// Compute derived metrics
	computeDerivedMetrics(result, since)

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

// estimateMaxConnections estimates max_connections based on instance memory
// GCP uses: LEAST(memory_in_GB * 25, 4000) but applies per-tier limits
func estimateMaxConnections(memoryGB float64) int {
	if memoryGB <= 0 {
		return 100 // Fallback for unknown
	}

	// GCP formula: memory * 25
	estimated := int(memoryGB * 25)

	// Apply GCP's tier-based caps
	if memoryGB <= 3.75 {
		// Micro/small instances
		if estimated > 250 {
			estimated = 250
		}
	} else if memoryGB <= 60 {
		// Standard/medium instances (most common)
		if estimated > 600 {
			estimated = 600
		}
	} else {
		// Large instances
		if estimated > 4000 {
			estimated = 4000
		}
	}

	return estimated
}

// statsFromData builds Stats from a metric's merged points and overrides Current with
// the correct per-series sum so multi-database metrics (num_backends, cache blocks, etc.)
// report the true instance-wide current value instead of the last per-database sample.
func statsFromData(data map[string][]float64, sums map[string]float64, name, unit string) (Stats, bool) {
	points, ok := data[name]
	if !ok || len(points) == 0 {
		return Stats{}, false
	}
	s := CalculateStats(points, unit)
	s.Current = sums[name]
	return s, true
}

// populateResult fills the CheckResult from collected metric data
func (c *Collector) populateResult(result *CheckResult, data map[string][]float64, sums map[string]float64) {
	// CPU
	if s, ok := statsFromData(data, sums, "cpu_utilization", "percent"); ok {
		result.Resources.CPU.Utilization = s
	}
	if points, ok := data["cpu_reserved_cores"]; ok && len(points) > 0 {
		result.Resources.CPU.ReservedCores = int(points[len(points)-1])
		result.InstanceSize.VCPU = result.Resources.CPU.ReservedCores
	}

	// Memory
	if s, ok := statsFromData(data, sums, "memory_utilization", "percent"); ok {
		result.Resources.Memory.Utilization = s
	}
	if points, ok := data["memory_quota"]; ok && len(points) > 0 {
		result.Resources.Memory.QuotaBytes = int64(points[len(points)-1])
		result.InstanceSize.MemoryGB = float64(result.Resources.Memory.QuotaBytes) / 1e9
	}
	if points, ok := data["memory_usage"]; ok && len(points) > 0 {
		result.Resources.Memory.UsageBytes = int64(points[len(points)-1])
	}

	// Disk
	if s, ok := statsFromData(data, sums, "disk_utilization", "percent"); ok {
		result.Resources.Disk.Utilization = s
	}
	if points, ok := data["disk_quota"]; ok && len(points) > 0 {
		result.Resources.Disk.QuotaBytes = int64(points[len(points)-1])
	}
	if points, ok := data["disk_bytes_used"]; ok && len(points) > 0 {
		result.Resources.Disk.BytesUsed = int64(points[len(points)-1])
	}
	if s, ok := statsFromData(data, sums, "disk_read_ops", "ops/sec"); ok {
		result.Resources.Disk.ReadOps = s
	}
	if s, ok := statsFromData(data, sums, "disk_write_ops", "ops/sec"); ok {
		result.Resources.Disk.WriteOps = s
	}

	// Connections — num_backends is per Postgres database; currentSum gives the instance total.
	if s, ok := statsFromData(data, sums, "num_backends", ""); ok {
		result.Connections.Count = s
	}

	// Max connections - fetch from metric or estimate from instance size
	if points, ok := data["max_connections"]; ok && len(points) > 0 {
		result.Connections.MaxConnections = int(points[len(points)-1])
	} else {
		// GCP only emits max_connections metric if explicitly configured
		// Estimate from instance memory using GCP's formula
		result.Connections.MaxConnections = estimateMaxConnections(result.InstanceSize.MemoryGB)
	}

	// Query Performance
	hasQueryMetrics := false
	if s, ok := statsFromData(data, sums, "query_latencies", "microseconds"); ok {
		result.QueryPerf.LatencyUS = s
		hasQueryMetrics = true
	}
	if s, ok := statsFromData(data, sums, "query_execution_time", "microseconds"); ok {
		result.QueryPerf.DatabaseLoadUS = s
		hasQueryMetrics = true
	}
	if s, ok := statsFromData(data, sums, "query_io_time", "microseconds"); ok {
		result.QueryPerf.IOTimeUS = s
		hasQueryMetrics = true
	}
	if s, ok := statsFromData(data, sums, "query_lock_time", "microseconds"); ok {
		result.QueryPerf.LockTimeUS = s
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
	if s, ok := statsFromData(data, sums, "transaction_id_utilization", "percent"); ok {
		result.DBHealth.TransactionIDUtilization = s
	}
	if points, ok := data["transaction_count"]; ok && len(points) > 0 {
		var total int64
		for _, p := range points {
			total += int64(p)
		}
		result.DBHealth.TransactionCount = total
	}
	if _, ok := data["deadlock_count"]; ok {
		result.DBHealth.DeadlockCount = int(sums["deadlock_count"])
	}
	if _, ok := data["oldest_transaction_age"]; ok {
		result.DBHealth.OldestTransactionAgeSec = int64(sums["oldest_transaction_age"])
	}
	if points, ok := data["autovacuum_count"]; ok && len(points) > 0 {
		result.DBHealth.AutovacuumCount = int(points[len(points)-1])
	}
	if points, ok := data["analyze_count"]; ok && len(points) > 0 {
		result.DBHealth.AnalyzeCount = int(points[len(points)-1])
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
	if s, ok := statsFromData(data, sums, "checkpoint_sync_latency", "ms"); ok {
		result.Checkpoints.SyncLatencyMS = s
	}
	if s, ok := statsFromData(data, sums, "checkpoint_write_latency", "ms"); ok {
		result.Checkpoints.WriteLatencyMS = s
	}

	// Replication
	if s, ok := statsFromData(data, sums, "replica_lag_bytes", "bytes"); ok {
		result.Replication.ReplicaLagBytes = s
	}
	if s, ok := statsFromData(data, sums, "replica_lag_seconds", "seconds"); ok {
		result.Replication.ReplicaLagSeconds = s
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

	// Cache
	if s, ok := statsFromData(data, sums, "shared_blocks_hit", "blocks"); ok {
		result.Cache.BlocksHit = s
	}
	if s, ok := statsFromData(data, sums, "shared_blocks_read", "blocks"); ok {
		result.Cache.BlocksRead = s
	}
	if s, ok := statsFromData(data, sums, "temp_blocks_read", "blocks"); ok {
		result.Cache.TempBlocksRead = s
	}
	if s, ok := statsFromData(data, sums, "temp_blocks_written", "blocks"); ok {
		result.Cache.TempBlocksWritten = s
	}

	// Throughput
	if s, ok := statsFromData(data, sums, "tuples_returned", "tuples/sec"); ok {
		result.Throughput.TuplesReturned = s
	}
	if s, ok := statsFromData(data, sums, "tuples_fetched", "tuples/sec"); ok {
		result.Throughput.TuplesFetched = s
	}
	if s, ok := statsFromData(data, sums, "tuples_inserted", "tuples/sec"); ok {
		result.Throughput.TuplesInserted = s
	}
	if s, ok := statsFromData(data, sums, "tuples_updated", "tuples/sec"); ok {
		result.Throughput.TuplesUpdated = s
	}
	if s, ok := statsFromData(data, sums, "tuples_deleted", "tuples/sec"); ok {
		result.Throughput.TuplesDeleted = s
	}
}
