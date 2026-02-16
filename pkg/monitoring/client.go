package monitoring

import (
	"context"
	"fmt"

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

	// Build API request
	apiReq := c.service.Projects.TimeSeries.Query(projectName, &monitoring.QueryTimeSeriesRequest{
		Query: req.Query,
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
