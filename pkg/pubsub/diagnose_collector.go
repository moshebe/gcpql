package pubsub

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/moshebe/gcpql/pkg/monitoring"
)

// DiagnoseOptions configures the diagnose command.
type DiagnoseOptions struct {
	Project      string
	Subscription string
	Since        time.Duration
}

// CollectDiagnoseMetrics fetches subscription + topic metrics for a single subscription.
func CollectDiagnoseMetrics(ctx context.Context, monClient *monitoring.Client, opts DiagnoseOptions) (*DiagnoseData, error) {
	startTime := time.Now()

	// Step 1: resolve topic name (serial — topic metrics depend on it).
	topicName, err := getSubscriptionTopic(ctx, monClient.HTTPClient(), opts.Project, opts.Subscription)
	if err != nil {
		log.Printf("Warning: resolving topic for %s: %v — topic metrics will be skipped", opts.Subscription, err)
		topicName = ""
	}

	data := &DiagnoseData{
		Project:      opts.Project,
		Subscription: opts.Subscription,
		TopicName:    topicName,
		Since:        opts.Since,
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Goroutine group A: subscription metrics.
	wg.Add(1)
	go func() {
		defer wg.Done()
		sub, collected, noData := collectSubMetrics(ctx, monClient, opts)
		mu.Lock()
		data.Sub = sub
		data.Metadata.MetricsCollected += collected
		data.Metadata.MetricsNoData += noData
		mu.Unlock()
	}()

	// Goroutine group B: topic metrics (only if topic name is known).
	wg.Add(1)
	go func() {
		defer wg.Done()
		if topicName == "" {
			mu.Lock()
			data.Topic = TopicMetrics{Available: false}
			mu.Unlock()
			return
		}
		topic, collected, noData := collectTopicMetrics(ctx, monClient, opts.Project, topicName, opts.Since)
		mu.Lock()
		data.Topic = topic
		data.Metadata.MetricsCollected += collected
		data.Metadata.MetricsNoData += noData
		mu.Unlock()
	}()

	wg.Wait()
	data.Metadata.CollectionDurationMS = time.Since(startTime).Milliseconds()
	return data, nil
}

// collectSubMetrics fetches all subscription-level metrics for diagnose.
// Returns (SubMetrics, metricsCollected, metricsNoData).
func collectSubMetrics(ctx context.Context, monClient *monitoring.Client, opts DiagnoseOptions) (SubMetrics, int, int) {
	rangeStr := formatDuration(opts.Since)
	now := time.Now()
	windowStart := now.Add(-opts.Since)
	sub := opts.Subscription
	proj := opts.Project

	type querySpec struct {
		key   string
		query string
	}

	specs := []querySpec{
		{
			key:   "backlog",
			query: fmt.Sprintf(`{__name__="pubsub.googleapis.com/subscription/num_undelivered_messages",project_id="%s",subscription_id="%s"}[%s]`, proj, sub, rangeStr),
		},
		{
			key:   "oldest",
			query: fmt.Sprintf(`{__name__="pubsub.googleapis.com/subscription/oldest_unacked_message_age",project_id="%s",subscription_id="%s"}[%s]`, proj, sub, rangeStr),
		},
		{
			key:   "expired",
			query: fmt.Sprintf(`{__name__="pubsub.googleapis.com/subscription/expired_ack_deadlines_count",project_id="%s",subscription_id="%s"}[%s]`, proj, sub, rangeStr),
		},
		{
			key:   "dlq",
			query: fmt.Sprintf(`{__name__="pubsub.googleapis.com/subscription/dead_letter_message_count",project_id="%s",subscription_id="%s"}[%s]`, proj, sub, rangeStr),
		},
		{
			key:   "ack",
			query: fmt.Sprintf(`{__name__="pubsub.googleapis.com/subscription/ack_message_count",project_id="%s",subscription_id="%s"}[%s]`, proj, sub, rangeStr),
		},
		{
			key:   "pull_all",
			query: fmt.Sprintf(`{__name__="pubsub.googleapis.com/subscription/pull_message_operation_count",project_id="%s",subscription_id="%s"}[%s]`, proj, sub, rangeStr),
		},
		{
			key:   "pull_err",
			query: fmt.Sprintf(`{__name__="pubsub.googleapis.com/subscription/pull_message_operation_count",project_id="%s",subscription_id="%s",response_code!="OK"}[%s]`, proj, sub, rangeStr),
		},
		{
			key:   "push_all",
			query: fmt.Sprintf(`{__name__="pubsub.googleapis.com/subscription/push_request_count",project_id="%s",subscription_id="%s"}[%s]`, proj, sub, rangeStr),
		},
		{
			key:   "push_err",
			query: fmt.Sprintf(`{__name__="pubsub.googleapis.com/subscription/push_request_count",project_id="%s",subscription_id="%s",response_code!="OK"}[%s]`, proj, sub, rangeStr),
		},
	}

	type result struct {
		key    string
		points []float64
		err    error
	}

	ch := make(chan result, len(specs))
	var wg sync.WaitGroup
	for _, s := range specs {
		wg.Add(1)
		go func(s querySpec) {
			defer wg.Done()
			resp, err := monClient.QueryTimeSeries(ctx, monitoring.QueryTimeSeriesRequest{
				Project:   proj,
				Query:     s.query,
				StartTime: windowStart,
				EndTime:   now,
			})
			if err != nil {
				ch <- result{key: s.key, err: err}
				return
			}
			pts := extractAllPoints(resp)
			ch <- result{key: s.key, points: pts}
		}(s)
	}
	go func() { wg.Wait(); close(ch) }()

	raw := make(map[string][]float64)
	collected, noData := 0, 0
	for res := range ch {
		if res.err != nil {
			log.Printf("Warning: collecting subscription metric %s: %v", res.key, res.err)
			noData++
			continue
		}
		raw[res.key] = res.points
		collected++
	}

	var m SubMetrics
	m.Backlog = computeStats(raw["backlog"])
	m.OldestUnackedSec = computeStats(raw["oldest"])
	m.ExpiredAckCount = int64(sumPoints(raw["expired"]))
	m.DLQCount = int64(lastPoint(raw["dlq"]))

	totalAck := sumPoints(raw["ack"])
	if secs := opts.Since.Seconds(); secs > 0 {
		m.AckRatePerSec = totalAck / secs
	}

	totalPull := sumPoints(raw["pull_all"])
	errPull := sumPoints(raw["pull_err"])
	if totalPull > 0 {
		m.PullErrorRate = errPull / totalPull
	}

	totalPush := sumPoints(raw["push_all"])
	errPush := sumPoints(raw["push_err"])
	if totalPush > 0 {
		m.PushErrorRate = errPush / totalPush
	}

	return m, collected, noData
}

// collectTopicMetrics fetches topic-level metrics for diagnose.
func collectTopicMetrics(ctx context.Context, monClient *monitoring.Client, project, topic string, since time.Duration) (TopicMetrics, int, int) {
	rangeStr := formatDuration(since)
	now := time.Now()
	windowStart := now.Add(-since)

	type querySpec struct {
		key   string
		query string
	}

	specs := []querySpec{
		{
			key:   "pub_all",
			query: fmt.Sprintf(`{__name__="pubsub.googleapis.com/topic/send_message_operation_count",project_id="%s",topic_id="%s"}[%s]`, project, topic, rangeStr),
		},
		{
			key:   "pub_err",
			query: fmt.Sprintf(`{__name__="pubsub.googleapis.com/topic/send_message_operation_count",project_id="%s",topic_id="%s",response_code!="OK"}[%s]`, project, topic, rangeStr),
		},
		{
			key:   "msg_size",
			query: fmt.Sprintf(`{__name__="pubsub.googleapis.com/topic/message_sizes",project_id="%s",topic_id="%s"}[%s]`, project, topic, rangeStr),
		},
	}

	type result struct {
		key    string
		points []float64
		err    error
	}

	ch := make(chan result, len(specs))
	var wg sync.WaitGroup
	for _, s := range specs {
		wg.Add(1)
		go func(s querySpec) {
			defer wg.Done()
			resp, err := monClient.QueryTimeSeries(ctx, monitoring.QueryTimeSeriesRequest{
				Project:   project,
				Query:     s.query,
				StartTime: windowStart,
				EndTime:   now,
			})
			if err != nil {
				ch <- result{key: s.key, err: err}
				return
			}
			ch <- result{key: s.key, points: extractAllPoints(resp)}
		}(s)
	}
	go func() { wg.Wait(); close(ch) }()

	raw := make(map[string][]float64)
	collected, noData := 0, 0
	for res := range ch {
		if res.err != nil {
			log.Printf("Warning: collecting topic metric %s: %v", res.key, res.err)
			noData++
			continue
		}
		raw[res.key] = res.points
		collected++
	}

	tm := TopicMetrics{Available: true}

	totalPub := sumPoints(raw["pub_all"])
	errPub := sumPoints(raw["pub_err"])
	if secs := since.Seconds(); secs > 0 {
		tm.PublishRatePerSec = totalPub / secs
	}
	if totalPub > 0 {
		tm.PublishErrorRate = errPub / totalPub
	}

	// message_sizes: best-effort average from available samples.
	if pts := raw["msg_size"]; len(pts) > 0 {
		var sum float64
		for _, v := range pts {
			sum += v
		}
		tm.AvgMessageSizeB = sum / float64(len(pts))
	}

	return tm, collected, noData
}

// ── helpers ───────────────────────────────────────────────────────────────────

// extractAllPoints flattens all values across all time series into a single slice.
func extractAllPoints(resp *monitoring.QueryTimeSeriesResponse) []float64 {
	var pts []float64
	for _, ts := range resp.TimeSeries {
		tsMap, ok := ts.(map[string]interface{})
		if !ok {
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
				pts = append(pts, val)
			}
		}
	}
	return pts
}

// computeStats computes current (last), min, and max from a slice of values.
func computeStats(pts []float64) Stats {
	if len(pts) == 0 {
		return Stats{}
	}
	s := Stats{Current: pts[len(pts)-1], Min: pts[0], Max: pts[0]}
	for _, v := range pts {
		s.Min = min(s.Min, v)
		s.Max = max(s.Max, v)
	}
	return s
}

// sumPoints returns the sum of all values in pts.
func sumPoints(pts []float64) float64 {
	var sum float64
	for _, v := range pts {
		sum += v
	}
	return sum
}

// lastPoint returns the last value in pts, or 0 if empty.
func lastPoint(pts []float64) float64 {
	if len(pts) == 0 {
		return 0
	}
	return pts[len(pts)-1]
}
