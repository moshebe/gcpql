package bigquery

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/gcp-metrics/gcp-metrics/pkg/monitoring"
	"google.golang.org/api/iterator"
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

// QueryJobs fetches job history from INFORMATION_SCHEMA
func (c *Client) QueryJobs(ctx context.Context, opts JobQueryOptions) ([]ExpensiveQuery, error) {
	if c.bqClient == nil {
		return nil, fmt.Errorf("bigquery client not initialized")
	}

	// Build INFORMATION_SCHEMA query
	query := fmt.Sprintf(`
		SELECT
			job_id,
			user_email,
			query,
			total_slot_ms,
			total_bytes_processed,
			TIMESTAMP_DIFF(end_time, creation_time, SECOND) as duration_seconds,
			creation_time as start_time
		FROM `+"`%s.region-%s`"+`.INFORMATION_SCHEMA.JOBS_BY_PROJECT
		WHERE creation_time >= TIMESTAMP_SUB(CURRENT_TIMESTAMP(), INTERVAL %s)
			AND job_type = 'QUERY'
			AND state = 'DONE'`,
		c.project, c.location, opts.Since)

	if opts.Dataset != "" {
		query += fmt.Sprintf(" AND referenced_tables LIKE '%%%s%%'", opts.Dataset)
	}

	if opts.OrderBy == "" {
		opts.OrderBy = "total_bytes_processed DESC"
	}
	query += " ORDER BY " + opts.OrderBy

	if opts.Limit == 0 {
		opts.Limit = 10
	}
	query += fmt.Sprintf(" LIMIT %d", opts.Limit)

	q := c.bqClient.Query(query)
	it, err := q.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}

	var results []ExpensiveQuery
	for {
		var row struct {
			JobID               string
			UserEmail           string
			Query               string
			TotalSlotMS         int64
			TotalBytesProcessed int64
			DurationSeconds     float64
			StartTime           time.Time
		}

		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}

		// Truncate query to 200 chars
		query := row.Query
		if len(query) > 200 {
			query = query[:197] + "..."
		}

		// Estimate cost: $5 per TB
		estimatedCost := float64(row.TotalBytesProcessed) / 1e12 * 5.0

		results = append(results, ExpensiveQuery{
			JobID:           row.JobID,
			UserEmail:       row.UserEmail,
			Query:           query,
			SlotMS:          row.TotalSlotMS,
			BytesProcessed:  row.TotalBytesProcessed,
			DurationSeconds: row.DurationSeconds,
			StartTime:       row.StartTime,
			EstimatedCost:   estimatedCost,
		})
	}

	return results, nil
}
