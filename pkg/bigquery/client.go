package bigquery

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"
	"github.com/gcp-metrics/gcp-metrics/pkg/monitoring"
)

// Client wraps BigQuery and Monitoring clients
type Client struct {
	bqClient         *bigquery.Client
	monitoringClient *monitoring.Client
	project          string
	location         string
}

// NewClient creates a new BigQuery client
func NewClient(ctx context.Context, project string, monClient *monitoring.Client) (*Client, error) {
	bqClient, err := bigquery.NewClient(ctx, project)
	if err != nil {
		return nil, fmt.Errorf("create bigquery client: %w", err)
	}

	return &Client{
		bqClient:         bqClient,
		monitoringClient: monClient,
		project:          project,
		location:         "us", // default location for INFORMATION_SCHEMA
	}, nil
}

// Close closes the BigQuery client
// Note: monitoring.Client uses http.Client which doesn't need explicit closing
func (c *Client) Close() error {
	if c.bqClient != nil {
		return c.bqClient.Close()
	}
	return nil
}

// SetLocation sets the multi-region location (us, eu, etc.)
func (c *Client) SetLocation(location string) {
	c.location = location
}

// JobQueryOptions configures INFORMATION_SCHEMA queries
type JobQueryOptions struct {
	Since      string
	Dataset    string
	Limit      int
	OrderBy    string
	UserEmail  string
	JobPattern string
}
