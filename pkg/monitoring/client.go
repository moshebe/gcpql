package monitoring

import (
	"context"
	"fmt"
	"strings"
	"time"

	monitoring "google.golang.org/api/monitoring/v3"
	"google.golang.org/api/option"
)

// Client wraps the GCP Monitoring API
type Client struct {
	service *monitoring.Service
}

// NewClient creates a new monitoring client using Application Default Credentials
func NewClient(ctx context.Context) (*Client, error) {
	service, err := monitoring.NewService(ctx, option.WithScopes(monitoring.MonitoringReadScope))
	if err != nil {
		return nil, fmt.Errorf("failed to create monitoring service: %w", err)
	}

	return &Client{service: service}, nil
}

// QueryTimeSeries executes an MQL query
func (c *Client) QueryTimeSeries(ctx context.Context, req QueryTimeSeriesRequest) (*QueryTimeSeriesResponse, error) {
	projectName := fmt.Sprintf("projects/%s", req.Project)

	query := req.Query

	// MQL queries require time ranges in the query string using 'within' operator
	// If user provided time range and query doesn't already have 'within', append it
	if !req.StartTime.IsZero() && !req.EndTime.IsZero() && !strings.Contains(strings.ToLower(query), "within") {
		duration := req.EndTime.Sub(req.StartTime)
		withinClause := formatDuration(duration)
		query = fmt.Sprintf("%s | within %s", query, withinClause)
	}

	// Build API request
	apiReq := c.service.Projects.TimeSeries.Query(projectName, &monitoring.QueryTimeSeriesRequest{
		Query: query,
	})

	// Execute query
	resp, err := apiReq.Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	// Convert to our response type
	// For now, store time series as interface{} to avoid complex mapping
	timeSeries := make([]interface{}, len(resp.TimeSeriesData))
	for i, ts := range resp.TimeSeriesData {
		timeSeries[i] = ts
	}

	return &QueryTimeSeriesResponse{
		TimeSeries: timeSeries,
	}, nil
}

// formatDuration converts a time.Duration to MQL within syntax (e.g., "5m", "1h")
func formatDuration(d time.Duration) string {
	// MQL supports: s, m, h, d
	if d < time.Hour {
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
