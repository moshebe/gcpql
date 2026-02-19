package bigquery

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gcp-metrics/gcp-metrics/pkg/monitoring"
)

// CheckOptions configures the check command
type CheckOptions struct {
	Project string
	Dataset string
	Since   time.Duration
}

// CollectCheckMetrics fetches all check metrics in parallel
func CollectCheckMetrics(ctx context.Context, client *Client, opts CheckOptions) (*CheckResult, error) {
	startTime := time.Now()

	result := &CheckResult{
		Project:   opts.Project,
		Dataset:   opts.Dataset,
		Timestamp: time.Now(),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Goroutine 1: Slot metrics from Cloud Monitoring
	wg.Add(1)
	go func() {
		defer wg.Done()
		slots, err := collectSlotMetrics(ctx, client, opts)
		if err != nil {
			log.Printf("Warning: Failed to collect slot metrics: %v", err)
			mu.Lock()
			result.Metadata.MetricsNoData++
			mu.Unlock()
			return
		}
		mu.Lock()
		result.Slots = slots
		result.Metadata.MetricsCollected += 3
		mu.Unlock()
	}()

	// Goroutine 2: Cost metrics
	wg.Add(1)
	go func() {
		defer wg.Done()
		cost, err := collectCostMetrics(ctx, client, opts)
		if err != nil {
			log.Printf("Warning: Failed to collect cost metrics: %v", err)
			mu.Lock()
			result.Metadata.MetricsNoData++
			mu.Unlock()
			return
		}
		mu.Lock()
		result.Cost = cost
		result.Metadata.MetricsCollected += 2
		mu.Unlock()
	}()

	// Goroutine 3: Top queries from INFORMATION_SCHEMA
	wg.Add(1)
	go func() {
		defer wg.Done()
		queries, err := collectTopQueries(ctx, client, opts)
		if err != nil {
			log.Printf("Warning: Failed to fetch top queries: %v", err)
			mu.Lock()
			result.Metadata.MetricsNoData++
			mu.Unlock()
			return
		}
		mu.Lock()
		result.TopQueries = queries
		result.Metadata.MetricsCollected++
		mu.Unlock()
	}()

	wg.Wait()

	result.Metadata.CollectionDurationMS = time.Since(startTime).Milliseconds()

	return result, nil
}

// formatDuration converts time.Duration to PromQL range format
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	if hours >= 24 {
		return fmt.Sprintf("%dd", hours/24)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	minutes := int(d.Minutes())
	if minutes > 0 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}

// collectSlotMetrics fetches slot utilization from Cloud Monitoring
func collectSlotMetrics(ctx context.Context, client *Client, opts CheckOptions) (SlotMetrics, error) {
	if client.monitoringClient == nil {
		return SlotMetrics{}, fmt.Errorf("monitoring client not initialized")
	}

	end := time.Now()
	start := end.Add(-opts.Since)
	rangeSelector := formatDuration(opts.Since)

	// Query: slots allocated for project
	allocatedQuery := fmt.Sprintf(`{__name__="bigquery.googleapis.com/slots/allocated_for_project",project_id="%s"}[%s]`,
		opts.Project, rangeSelector)

	allocatedPoints, err := queryMetric(ctx, client, allocatedQuery, start, end)
	if err != nil {
		return SlotMetrics{}, fmt.Errorf("query allocated slots: %w", err)
	}

	// Query: current slot usage
	currentQuery := fmt.Sprintf(`{__name__="bigquery.googleapis.com/slots/total_allocated",project_id="%s"}[%s]`,
		opts.Project, rangeSelector)

	currentPoints, err := queryMetric(ctx, client, currentQuery, start, end)
	if err != nil {
		return SlotMetrics{}, fmt.Errorf("query current slots: %w", err)
	}

	// Query: queries in flight
	inflightQuery := fmt.Sprintf(`{__name__="bigquery.googleapis.com/job/num_in_flight",project_id="%s"}[%s]`,
		opts.Project, rangeSelector)

	inflightPoints, err := queryMetric(ctx, client, inflightQuery, start, end)
	if err != nil {
		// Non-fatal
		inflightPoints = []float64{}
	}

	// Calculate stats
	allocatedStats := CalculateStats(allocatedPoints)
	currentStats := CalculateStats(currentPoints)

	var allocated int64
	if len(allocatedPoints) > 0 {
		allocated = int64(allocatedStats.Current)
	}

	var current int64
	if len(currentPoints) > 0 {
		current = int64(currentStats.Current)
	}

	var utilization float64
	if allocated > 0 && current <= allocated {
		utilization = float64(current) / float64(allocated) * 100
	}

	var inFlight int
	if len(inflightPoints) > 0 {
		inFlightStats := CalculateStats(inflightPoints)
		inFlight = int(inFlightStats.Current)
	}

	return SlotMetrics{
		Allocated:       allocated,
		Current:         current,
		Peak:            int64(currentStats.Max),
		Utilization:     utilization,
		QueriesInFlight: inFlight,
		QueriesQueued:   0, // TODO: Add queued queries metric if available
	}, nil
}

// queryMetric is a helper that queries a metric and extracts float64 values
func queryMetric(ctx context.Context, client *Client, query string, start, end time.Time) ([]float64, error) {
	resp, err := client.monitoringClient.QueryTimeSeries(ctx, monitoring.QueryTimeSeriesRequest{
		Project:   client.project,
		Query:     query,
		StartTime: start,
		EndTime:   end,
	})
	if err != nil {
		return nil, err
	}

	// Extract points from Prometheus response format
	var points []float64
	for _, ts := range resp.TimeSeries {
		// Each ts is a map[string]interface{} with "values" key (range query)
		if tsMap, ok := ts.(map[string]interface{}); ok {
			// PromQL range query returns: "values": [[timestamp, "value_string"], ...]
			if values, ok := tsMap["values"].([]interface{}); ok {
				for _, v := range values {
					if valueArr, ok := v.([]interface{}); ok && len(valueArr) >= 2 {
						if valStr, ok := valueArr[1].(string); ok {
							var val float64
							if _, err := fmt.Sscanf(valStr, "%f", &val); err != nil {
								continue // Skip invalid values
							}
							points = append(points, val)
						}
					}
				}
			}
		}
	}

	return points, nil
}

// collectCostMetrics fetches cost indicators from Cloud Monitoring
func collectCostMetrics(ctx context.Context, client *Client, opts CheckOptions) (CostMetrics, error) {
	if client.monitoringClient == nil {
		return CostMetrics{}, fmt.Errorf("monitoring client not initialized")
	}

	end := time.Now()
	start := end.Add(-opts.Since)

	// Query: storage bytes
	storageQuery := fmt.Sprintf(`{__name__="bigquery.googleapis.com/storage/stored_bytes",project_id="%s"}[%s]`,
		opts.Project, formatDuration(opts.Since))

	storagePoints, err := queryMetric(ctx, client, storageQuery, start, end)
	if err != nil {
		return CostMetrics{}, fmt.Errorf("query storage: %w", err)
	}

	// Query: bytes scanned
	scannedQuery := fmt.Sprintf(`{__name__="bigquery.googleapis.com/query/scanned_bytes",project_id="%s"}[%s]`,
		opts.Project, formatDuration(opts.Since))

	scannedPoints, err := queryMetric(ctx, client, scannedQuery, start, end)
	if err != nil {
		// Non-fatal - no queries may have run
		scannedPoints = []float64{}
	}

	// Calculate storage size and cost
	var storageGB float64
	var storageCostDaily float64
	if len(storagePoints) > 0 {
		storageStats := CalculateStats(storagePoints)
		storageGB = storageStats.Current / 1e9 // bytes to GB
		// $0.02 per GB per month = $0.02/30 per day
		storageCostDaily = storageGB * 0.02 / 30
	}

	// Calculate bytes scanned and estimated cost
	var bytesScannedTotal int64
	var estimatedCost float64
	if len(scannedPoints) > 0 {
		var sum float64
		for _, v := range scannedPoints {
			sum += v
		}
		bytesScannedTotal = int64(sum)
		// $5 per TB = $5 / 1e12 per byte
		estimatedCost = float64(bytesScannedTotal) / 1e12 * 5.0
	}

	return CostMetrics{
		StorageGB:         storageGB,
		StorageCostDaily:  storageCostDaily,
		BytesScannedTotal: bytesScannedTotal,
		EstimatedCost:     estimatedCost,
	}, nil
}

// collectTopQueries fetches expensive queries from INFORMATION_SCHEMA
func collectTopQueries(ctx context.Context, client *Client, opts CheckOptions) ([]ExpensiveQuery, error) {
	// TODO: Query INFORMATION_SCHEMA.JOBS_BY_PROJECT

	// Placeholder implementation
	return []ExpensiveQuery{}, fmt.Errorf("not implemented")
}
