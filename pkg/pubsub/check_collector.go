package pubsub

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
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

// formatDuration converts a time.Duration to PromQL range selector format (e.g. "5m", "1h").
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

// computeStatus assigns a status and one-line reason to a snapshot.
func computeStatus(s SubscriptionSnapshot) (Severity, string) {
	if s.OldestUnackedSec > 3600 {
		return SeverityCritical, fmt.Sprintf("oldest unacked %.0fs (>1h)", s.OldestUnackedSec)
	}
	if s.DLQCount > 0 {
		return SeverityCritical, fmt.Sprintf("DLQ has %d messages", s.DLQCount)
	}
	if s.OldestUnackedSec > 600 {
		return SeverityWarning, fmt.Sprintf("oldest unacked %.0fs (>10m)", s.OldestUnackedSec)
	}
	if s.Backlog > 10000 {
		return SeverityWarning, fmt.Sprintf("backlog %d (>10k)", s.Backlog)
	}
	if s.ExpiredAckCount > 0 {
		return SeverityWarning, fmt.Sprintf("%d expired ack deadlines", s.ExpiredAckCount)
	}
	return SeverityInfo, "OK"
}

// sortSnapshots sorts in-place: CRITICAL first, then WARNING, then INFO.
// Within a tier, sorts by OldestUnackedSec descending.
func sortSnapshots(s []SubscriptionSnapshot) {
	order := map[Severity]int{SeverityCritical: 0, SeverityWarning: 1, SeverityInfo: 2}
	sort.SliceStable(s, func(i, j int) bool {
		if order[s[i].Status] != order[s[j].Status] {
			return order[s[i].Status] < order[s[j].Status]
		}
		return s[i].OldestUnackedSec > s[j].OldestUnackedSec
	})
}

// extractSeriesLast returns the last value per label across all time series.
// Used for GAUGE metrics where the most recent sample is the current value.
func extractSeriesLast(resp *monitoring.QueryTimeSeriesResponse, labelKey string) map[string]float64 {
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
		labelVal, ok := metricLabels[labelKey].(string)
		if !ok || labelVal == "" {
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
		if val, err := strconv.ParseFloat(valStr, 64); err == nil {
			result[labelVal] = val
		}
	}
	return result
}

// extractSeriesSums returns the sum of all values per label across all time series.
// Used for DELTA metrics (e.g. expired_ack_deadlines_count) where samples represent
// increments and the total over the window is what matters.
func extractSeriesSums(resp *monitoring.QueryTimeSeriesResponse, labelKey string) map[string]float64 {
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
		labelVal, ok := metricLabels[labelKey].(string)
		if !ok || labelVal == "" {
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
				result[labelVal] += val
			}
		}
	}
	return result
}

// CollectCheckMetrics fetches a health snapshot of all subscriptions in the project.
func CollectCheckMetrics(ctx context.Context, monClient *monitoring.Client, opts CheckOptions) (*CheckResult, error) {
	startTime := time.Now()
	rangeStr := formatDuration(opts.Since)
	now := time.Now()
	windowStart := now.Add(-opts.Since)

	type querySpec struct {
		key   string
		query string
		sum   bool
	}

	specs := []querySpec{
		{
			key:   "backlog",
			query: fmt.Sprintf(`{__name__="pubsub.googleapis.com/subscription/num_undelivered_messages",project_id="%s"}[%s]`, opts.Project, rangeStr),
		},
		{
			key:   "oldest",
			query: fmt.Sprintf(`{__name__="pubsub.googleapis.com/subscription/oldest_unacked_message_age",project_id="%s"}[%s]`, opts.Project, rangeStr),
		},
		{
			key:   "expired",
			query: fmt.Sprintf(`{__name__="pubsub.googleapis.com/subscription/expired_ack_deadlines_count",project_id="%s"}[%s]`, opts.Project, rangeStr),
			sum:   true,
		},
		{
			key:   "dlq",
			query: fmt.Sprintf(`{__name__="pubsub.googleapis.com/subscription/dead_letter_message_count",project_id="%s"}[%s]`, opts.Project, rangeStr),
		},
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
				vals = extractSeriesSums(resp, "subscription_id")
			} else {
				vals = extractSeriesLast(resp, "subscription_id")
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
			log.Printf("collecting pubsub %s: %v", res.key, res.err)
			meta.MetricsNoData++
			continue
		}
		allMetrics[res.key] = res.values
		meta.MetricsCollected++
	}

	subNames := make(map[string]struct{})
	for _, vals := range allMetrics {
		for name := range vals {
			subNames[name] = struct{}{}
		}
	}

	snapshots := make([]SubscriptionSnapshot, 0, len(subNames))
	for name := range subNames {
		snap := SubscriptionSnapshot{Name: name}
		if v, ok := allMetrics["backlog"][name]; ok {
			snap.Backlog = int64(v)
		}
		if v, ok := allMetrics["oldest"][name]; ok {
			snap.OldestUnackedSec = v
		}
		if v, ok := allMetrics["expired"][name]; ok {
			snap.ExpiredAckCount = int64(v)
		}
		if v, ok := allMetrics["dlq"][name]; ok {
			snap.DLQCount = int64(v)
		}
		snap.Status, snap.StatusReason = computeStatus(snap)
		snapshots = append(snapshots, snap)
	}

	sortSnapshots(snapshots)
	meta.CollectionDurationMS = time.Since(startTime).Milliseconds()

	return &CheckResult{
		Project:       opts.Project,
		Timestamp:     time.Now(),
		Subscriptions: snapshots,
		Metadata:      meta,
	}, nil
}
