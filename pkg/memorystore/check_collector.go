package memorystore

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/moshebe/gcpql/pkg/monitoring"
)

// CheckOptions configures the check command.
type CheckOptions struct {
	Project string
	Since   time.Duration
	Top     int
}

// formatDuration converts a time.Duration to PromQL range selector format.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

// shortInstanceName extracts the instance name from the full resource path.
// "projects/my-proj/locations/us-central1/instances/my-redis" → "my-redis"
func shortInstanceName(fullID string) string {
	if idx := strings.LastIndex(fullID, "/"); idx >= 0 {
		return fullID[idx+1:]
	}
	return fullID
}

// computeStatus assigns a status and one-line reason to a snapshot.
func computeStatus(s InstanceSnapshot) (Severity, string) {
	if s.MemoryUsage > 0.90 {
		return SeverityCritical, fmt.Sprintf("memory at %.0f%% (>90%%)", s.MemoryUsage*100)
	}
	if s.UptimeSec > 0 && s.UptimeSec < 300 {
		return SeverityCritical, fmt.Sprintf("recently restarted (uptime %s)", fmtDurationShort(s.UptimeSec))
	}
	if s.MemoryUsage > 0.75 {
		return SeverityWarning, fmt.Sprintf("memory at %.0f%% (>75%%)", s.MemoryUsage*100)
	}
	if s.RejectedConnections > 0 {
		return SeverityCritical, fmt.Sprintf("%d rejected connections", s.RejectedConnections)
	}
	if s.EvictedKeys > 0 {
		return SeverityWarning, fmt.Sprintf("%d evicted keys (memory pressure)", s.EvictedKeys)
	}
	if s.CacheHitRatio > 0 && s.CacheHitRatio < 0.30 && s.KeyCount > 100 {
		return SeverityWarning, fmt.Sprintf("low hit ratio %.0f%% (<30%%)", s.CacheHitRatio*100)
	}
	return SeverityInfo, "OK"
}

// fmtDurationShort formats seconds to a human-readable duration.
func fmtDurationShort(sec float64) string {
	d := time.Duration(sec) * time.Second
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd%dh", int(d.Hours())/24, int(d.Hours())%24)
}

// sortSnapshots sorts in-place: CRITICAL first, then WARNING, then INFO.
// Within a tier, sorts by MemoryUsage descending.
func sortSnapshots(s []InstanceSnapshot) {
	order := map[Severity]int{SeverityCritical: 0, SeverityWarning: 1, SeverityInfo: 2}
	sort.SliceStable(s, func(i, j int) bool {
		if order[s[i].Status] != order[s[j].Status] {
			return order[s[i].Status] < order[s[j].Status]
		}
		return s[i].MemoryUsage > s[j].MemoryUsage
	})
}

// extractSeriesLast returns the last value per instance_id across all time series.
// For metrics with multiple series per instance (replicas), uses the reducer function.
func extractSeriesLast(resp *monitoring.QueryTimeSeriesResponse) map[string]float64 {
	result := make(map[string]float64)
	for _, ts := range resp.TimeSeries {
		tsMap, ok := ts.(map[string]interface{})
		if !ok {
			continue
		}
		metricLabels, ok := tsMap["metric"].(map[string]interface{})
		if !ok {
			continue
		}
		instanceID, ok := metricLabels["instance_id"].(string)
		if !ok || instanceID == "" {
			continue
		}
		values, ok := tsMap["values"].([]interface{})
		if !ok || len(values) == 0 {
			continue
		}
		last := values[len(values)-1]
		valArr, ok := last.([]interface{})
		if !ok || len(valArr) < 2 {
			continue
		}
		valStr, ok := valArr[1].(string)
		if !ok {
			continue
		}
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			continue
		}
		// For multiple series per instance, take the max (worst case for ratios, largest for counts).
		if existing, exists := result[instanceID]; !exists || val > existing {
			result[instanceID] = val
		}
	}
	return result
}

// extractSeriesSums returns the sum of all values per instance_id across all time series.
// Used for DELTA metrics where samples represent increments.
func extractSeriesSums(resp *monitoring.QueryTimeSeriesResponse) map[string]float64 {
	result := make(map[string]float64)
	for _, ts := range resp.TimeSeries {
		tsMap, ok := ts.(map[string]interface{})
		if !ok {
			continue
		}
		metricLabels, ok := tsMap["metric"].(map[string]interface{})
		if !ok {
			continue
		}
		instanceID, ok := metricLabels["instance_id"].(string)
		if !ok || instanceID == "" {
			continue
		}
		values, ok := tsMap["values"].([]interface{})
		if !ok {
			continue
		}
		for _, v := range values {
			valArr, ok := v.([]interface{})
			if !ok || len(valArr) < 2 {
				continue
			}
			valStr, ok := valArr[1].(string)
			if !ok {
				continue
			}
			if val, err := strconv.ParseFloat(valStr, 64); err == nil {
				result[instanceID] += val
			}
		}
	}
	return result
}

// CollectCheckMetrics fetches a health snapshot of all Redis instances in the project.
func CollectCheckMetrics(ctx context.Context, monClient *monitoring.Client, opts CheckOptions) (*CheckResult, error) {
	now := time.Now()
	rangeStr := formatDuration(opts.Since)
	windowStart := now.Add(-opts.Since)

	type querySpec struct {
		key   string
		query string
		sum   bool // true for DELTA metrics
	}

	specs := []querySpec{
		{key: "memory_usage", query: fmt.Sprintf(`{__name__="redis.googleapis.com/stats/memory/usage_ratio"}[%s]`, rangeStr)},
		{key: "clients", query: fmt.Sprintf(`{__name__="redis.googleapis.com/clients/connected"}[%s]`, rangeStr)},
		{key: "hit_ratio", query: fmt.Sprintf(`{__name__="redis.googleapis.com/stats/cache_hit_ratio"}[%s]`, rangeStr)},
		{key: "keys", query: fmt.Sprintf(`{__name__="redis.googleapis.com/keyspace/keys"}[%s]`, rangeStr)},
		{key: "evicted", query: fmt.Sprintf(`{__name__="redis.googleapis.com/stats/evicted_keys"}[%s]`, rangeStr), sum: true},
		{key: "rejected", query: fmt.Sprintf(`{__name__="redis.googleapis.com/stats/reject_connections_count"}[%s]`, rangeStr), sum: true},
		{key: "uptime", query: fmt.Sprintf(`{__name__="redis.googleapis.com/server/uptime"}[%s]`, rangeStr)},
	}

	type metricResult struct {
		key    string
		values map[string]float64
		err    error
	}

	ch := make(chan metricResult, len(specs))
	var wg sync.WaitGroup

	for _, spec := range specs {
		wg.Add(1)
		go func(s querySpec) {
			defer wg.Done()
			resp, err := monClient.QueryTimeSeries(ctx, monitoring.QueryTimeSeriesRequest{
				Project:   opts.Project,
				Query:     s.query,
				StartTime: windowStart,
				EndTime:   now,
			})
			if err != nil {
				ch <- metricResult{key: s.key, err: err}
				return
			}
			var vals map[string]float64
			if s.sum {
				vals = extractSeriesSums(resp)
			} else {
				vals = extractSeriesLast(resp)
			}
			ch <- metricResult{key: s.key, values: vals}
		}(spec)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	allMetrics := make(map[string]map[string]float64)
	meta := Metadata{}
	for res := range ch {
		if res.err != nil {
			log.Printf("Warning: collecting redis %s: %v", res.key, res.err)
			meta.MetricsNoData++
			continue
		}
		allMetrics[res.key] = res.values
		meta.MetricsCollected++
	}

	// Collect all unique instance IDs
	instanceIDs := make(map[string]struct{})
	for _, vals := range allMetrics {
		for id := range vals {
			instanceIDs[id] = struct{}{}
		}
	}

	snapshots := make([]InstanceSnapshot, 0, len(instanceIDs))
	for id := range instanceIDs {
		snap := InstanceSnapshot{Name: shortInstanceName(id)}
		if v, ok := allMetrics["memory_usage"][id]; ok {
			snap.MemoryUsage = v
		}
		if v, ok := allMetrics["clients"][id]; ok {
			snap.ConnectedClients = int64(v)
		}
		if v, ok := allMetrics["hit_ratio"][id]; ok {
			snap.CacheHitRatio = v
		}
		if v, ok := allMetrics["keys"][id]; ok {
			snap.KeyCount = int64(v)
		}
		if v, ok := allMetrics["evicted"][id]; ok {
			snap.EvictedKeys = int64(v)
		}
		if v, ok := allMetrics["rejected"][id]; ok {
			snap.RejectedConnections = int64(v)
		}
		if v, ok := allMetrics["uptime"][id]; ok {
			snap.UptimeSec = v
		}
		snap.Status, snap.StatusReason = computeStatus(snap)
		snapshots = append(snapshots, snap)
	}

	sortSnapshots(snapshots)
	insights := generateInsights(snapshots)
	meta.CollectionDurationMS = time.Since(now).Milliseconds()

	return &CheckResult{
		Project:   opts.Project,
		Timestamp: now,
		Instances: snapshots,
		Insights:  insights,
		Metadata:  meta,
	}, nil
}

const (
	highClientThreshold = 500
	longUptimeDays      = 90
)

// generateInsights produces actionable observations that don't warrant a WARNING/CRITICAL status.
func generateInsights(snapshots []InstanceSnapshot) []Insight {
	var insights []Insight

	// High client counts
	for _, s := range snapshots {
		if s.ConnectedClients > highClientThreshold {
			insights = append(insights, Insight{
				Instance: s.Name,
				Message:  fmt.Sprintf("%d connected clients — review connection pooling", s.ConnectedClients),
			})
		}
	}

	// Long uptime: if most instances are old, summarize; otherwise list individually.
	var longUptime []InstanceSnapshot
	for _, s := range snapshots {
		if s.UptimeSec/86400 > longUptimeDays {
			longUptime = append(longUptime, s)
		}
	}
	if len(longUptime) > len(snapshots)/2 {
		insights = append(insights, Insight{
			Instance: "(all)",
			Message:  fmt.Sprintf("%d/%d instances have >%dd uptime — consider scheduling maintenance", len(longUptime), len(snapshots), longUptimeDays),
		})
	} else {
		for _, s := range longUptime {
			insights = append(insights, Insight{
				Instance: s.Name,
				Message:  fmt.Sprintf("uptime %dd — consider scheduling a maintenance window", int(s.UptimeSec/86400)),
			})
		}
	}

	return insights
}
