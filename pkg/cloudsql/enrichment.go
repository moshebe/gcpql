package cloudsql

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/moshebe/gcpql/pkg/monitoring"
)

// FetchRecommendations fetches Cloud Recommender suggestions for the given instance.
// Returns Recommendations{Available: false} (no error) on any non-200 response.
func FetchRecommendations(ctx context.Context, httpClient *http.Client, project, region string) (Recommendations, error) {
	url := fmt.Sprintf(
		"https://recommender.googleapis.com/v1/projects/%s/locations/%s/recommenders/google.cloudsql.instance.PerformanceRecommender/recommendations",
		project, region,
	)
	return fetchRecommendations(ctx, httpClient, url)
}

func fetchRecommendations(ctx context.Context, httpClient *http.Client, url string) (Recommendations, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return Recommendations{}, fmt.Errorf("building recommender request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return Recommendations{}, fmt.Errorf("recommender request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Recommendations{Available: false}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Recommendations{}, fmt.Errorf("reading recommender response: %w", err)
	}

	var parsed struct {
		Recommendations []struct {
			Description   string `json:"description"`
			Priority      string `json:"priority"`
			PrimaryImpact struct {
				Category string `json:"category"`
			} `json:"primaryImpact"`
			StateInfo struct {
				State string `json:"state"`
			} `json:"stateInfo"`
		} `json:"recommendations"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Recommendations{}, fmt.Errorf("parsing recommender response: %w", err)
	}

	items := make([]Recommendation, 0, len(parsed.Recommendations))
	for _, r := range parsed.Recommendations {
		items = append(items, Recommendation{
			Description: r.Description,
			Impact:      priorityToImpact(r.Priority),
			State:       r.StateInfo.State,
		})
	}

	return Recommendations{Available: true, Items: items}, nil
}

// FetchQueryInsights queries Cloud Monitoring for Query Insights execution time metrics.
// Returns QueryInsights{Available: false} (no error) if QI is not enabled or query fails.
// Data is grouped by (database, user, client_addr) — Cloud Monitoring aggregates at this level.
func FetchQueryInsights(ctx context.Context, client *monitoring.Client, project, instance string, since time.Duration, topN int) (QueryInsights, error) {
	now := time.Now()
	// Cloud Monitoring uses resource_id (not database_id) for Query Insights metrics.
	query := fmt.Sprintf(`{__name__="cloudsql.googleapis.com/database/postgresql/insights/aggregate/execution_time",resource_id="%s:%s"}`, project, instance)
	req := monitoring.QueryTimeSeriesRequest{
		Project:   project,
		Query:     query,
		StartTime: now.Add(-since),
		EndTime:   now,
	}

	resp, err := client.QueryTimeSeries(ctx, req)
	if err != nil {
		return QueryInsights{Available: false}, nil
	}

	// Re-marshal to a standard JSON envelope so parseQueryInsightsResponse
	// can work with a consistent format. This works because the monitoring client
	// already decoded the Prometheus response — metric labels are map[string]string
	// and values are []interface{}{timestamp_float, value_string}.
	wrapped := map[string]interface{}{
		"data": map[string]interface{}{
			"result": resp.TimeSeries,
		},
	}
	raw, err := json.Marshal(wrapped)
	if err != nil {
		return QueryInsights{Available: false}, nil
	}

	return parseQueryInsightsResponse(raw, topN), nil
}

// parseQueryInsightsResponse parses raw JSON ({"data":{"result":[...]}}) from the
// Cloud Monitoring PromQL endpoint and returns top-N rows sorted by total time.
// Each series is a DELTA metric grouped by (database, user, client_addr).
func parseQueryInsightsResponse(raw []byte, topN int) QueryInsights {
	var envelope struct {
		Data struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Values [][]interface{}   `json:"values"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return QueryInsights{Available: false}
	}

	results := envelope.Data.Result
	if len(results) == 0 {
		return QueryInsights{Available: false}
	}

	queries := make([]TopQuery, 0, len(results))
	for _, ts := range results {
		if len(ts.Values) < 2 {
			continue
		}
		// Values are cumulative counters; delta = last - first gives window total.
		firstUS := extractFloat(ts.Values[0])
		lastUS := extractFloat(ts.Values[len(ts.Values)-1])
		deltaUS := lastUS - firstUS
		if deltaUS <= 0 {
			continue
		}
		count := int64(len(ts.Values))
		totalMS := deltaUS / 1000.0
		// avg = execution time per sampling interval (usually 1 min)
		avgMS := totalMS / float64(count-1)
		queries = append(queries, TopQuery{
			Database:     ts.Metric["database"],
			User:         ts.Metric["user"],
			ClientAddr:   ts.Metric["client_addr"],
			SampleCount:  count,
			TotalTimeMS:  totalMS,
			AvgLatencyMS: avgMS,
		})
	}

	sort.Slice(queries, func(i, j int) bool {
		return queries[i].TotalTimeMS > queries[j].TotalTimeMS
	})

	if topN > 0 && len(queries) > topN {
		queries = queries[:topN]
	}

	return QueryInsights{Available: true, TopQueries: queries}
}

// extractFloat extracts a float64 from a Prometheus value pair [timestamp, "value_string"].
func extractFloat(v []interface{}) float64 {
	if len(v) < 2 {
		return 0
	}
	valStr, ok := v[1].(string)
	if !ok {
		return 0
	}
	val, _ := strconv.ParseFloat(valStr, 64)
	return val
}

// priorityToImpact maps GCP Recommender priority strings to impact levels.
func priorityToImpact(priority string) string {
	switch priority {
	case "P1":
		return "HIGH"
	case "P2":
		return "MEDIUM"
	case "P3", "P4":
		return "LOW"
	default:
		return "UNKNOWN"
	}
}
