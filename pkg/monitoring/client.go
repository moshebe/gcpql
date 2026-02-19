package monitoring

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/transport"
)

// Client wraps the GCP Monitoring API
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// NewClient creates a new monitoring client using Application Default Credentials
func NewClient(ctx context.Context) (*Client, error) {
	// Create authenticated HTTP client using ADC.
	// Includes sqladmin scope so the same client can call the Cloud SQL Admin API.
	httpClient, _, err := transport.NewHTTPClient(ctx,
		option.WithScopes(
			"https://www.googleapis.com/auth/monitoring.read",
			"https://www.googleapis.com/auth/sqlservice.admin",
		))
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticated HTTP client: %w", err)
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    "https://monitoring.googleapis.com",
	}, nil
}

// NewClientForTesting creates a Client with a custom base URL and HTTP client.
// For use in tests only.
func NewClientForTesting(httpClient *http.Client, baseURL string) *Client {
	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
	}
}

// HTTPClient returns the underlying authenticated HTTP client.
// It can be reused to call other GCP APIs (e.g., Cloud SQL Admin API).
func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
}

// QueryTimeSeries executes a PromQL query
func (c *Client) QueryTimeSeries(ctx context.Context, req QueryTimeSeriesRequest) (*QueryTimeSeriesResponse, error) {
	query := req.Query

	// GCP requires metric names with dots/slashes to use __name__ label selector format
	query = normalizeMetricQuery(query)

	// PromQL queries use range vector syntax [5m] for time ranges
	// If user provided time range and query doesn't already have range selector, append it
	if !req.StartTime.IsZero() && !req.EndTime.IsZero() && !hasRangeSelector(query) {
		duration := req.EndTime.Sub(req.StartTime)
		rangeSelector := formatDuration(duration)
		query = injectRangeSelector(query, rangeSelector)
	}

	// Build PromQL API endpoint
	url := fmt.Sprintf("%s/v1/projects/%s/location/global/prometheus/api/v1/query",
		c.baseURL, req.Project)

	// Build request body
	reqBody := map[string]interface{}{
		"query": query,
	}
	if !req.EndTime.IsZero() {
		reqBody["time"] = req.EndTime.Format(time.RFC3339)
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for error response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var promResp struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string        `json:"resultType"`
			Result     []interface{} `json:"result"`
		} `json:"data"`
		Error     string `json:"error,omitempty"`
		ErrorType string `json:"errorType,omitempty"`
	}

	if err := json.Unmarshal(respBody, &promResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if promResp.Status != "success" {
		return nil, fmt.Errorf("query failed: %s (%s)", promResp.Error, promResp.ErrorType)
	}

	return &QueryTimeSeriesResponse{
		TimeSeries: promResp.Data.Result,
	}, nil
}

// normalizeMetricQuery wraps simple metric names in GCP's required __name__ format
func normalizeMetricQuery(query string) string {
	query = strings.TrimSpace(query)

	// If already has braces, assume it's valid PromQL
	if strings.Contains(query, "{") {
		return query
	}

	// If contains dots or slashes (GCP metric names), wrap in __name__ selector
	if strings.Contains(query, ".") || strings.Contains(query, "/") {
		return fmt.Sprintf(`{__name__="%s"}`, query)
	}

	// Otherwise, return as-is
	return query
}

// hasRangeSelector checks if query already has a PromQL range selector like [5m]
func hasRangeSelector(query string) bool {
	// Match PromQL range selector pattern: [5m], [1h], [30s], etc.
	matched, _ := regexp.MatchString(`\[\d+[smhd]\]`, query)
	return matched
}

// injectRangeSelector adds range selector to PromQL query
func injectRangeSelector(query, duration string) string {
	// For simple metric queries, append range selector
	// e.g., "metric_name" -> "metric_name[5m]"
	query = strings.TrimSpace(query)

	// If query ends with }, it's likely a selector, append range after it
	if strings.HasSuffix(query, "}") {
		return fmt.Sprintf("%s[%s]", query, duration)
	}

	// Otherwise, append to end
	return fmt.Sprintf("%s[%s]", query, duration)
}

// formatDuration converts a time.Duration to PromQL range syntax (e.g., "5m", "1h")
func formatDuration(d time.Duration) string {
	// PromQL supports: s, m, h, d
	if d < time.Minute {
		seconds := int(d.Seconds())
		return fmt.Sprintf("%ds", seconds)
	} else if d < time.Hour {
		minutes := int(d.Minutes())
		return fmt.Sprintf("%dm", minutes)
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		return fmt.Sprintf("%dh", hours)
	} else {
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	}
}
