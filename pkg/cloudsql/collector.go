package cloudsql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moshebeladev/gcp-metrics/pkg/monitoring"
)

// Collector fetches CloudSQL metrics
type Collector struct {
	client *monitoring.Client
}

// NewCollector creates a new CloudSQL metrics collector
func NewCollector(client *monitoring.Client) *Collector {
	return &Collector{client: client}
}

// ParseInstanceID parses instance ID in various formats
func ParseInstanceID(instanceID, fallbackProject string) (project, instance string, err error) {
	parts := strings.Split(instanceID, ":")

	switch len(parts) {
	case 1:
		// Short form: "my-instance"
		if fallbackProject == "" {
			return "", "", fmt.Errorf("project required: use --project or provide full instance ID")
		}
		return fallbackProject, parts[0], nil

	case 2:
		// Full form: "my-project:my-instance"
		return parts[0], parts[1], nil

	case 3:
		// Database ID format: "my-project:region:my-instance"
		return parts[0], parts[2], nil

	default:
		return "", "", fmt.Errorf("invalid instance ID format: %s", instanceID)
	}
}

// CollectMetrics fetches all metrics for an instance
func (c *Collector) CollectMetrics(ctx context.Context, project, instance string, since time.Duration) (*CheckResult, error) {
	startTime := time.Now().Add(-since)
	endTime := time.Now()

	// Build database_id filter for queries
	databaseID := fmt.Sprintf("%s:%s", project, instance)

	result := &CheckResult{
		Instance:   databaseID,
		Project:    project,
		Timestamp:  endTime,
		TimeWindow: formatDuration(since),
		Metadata: Metadata{
			MetricsUnavailable: []string{},
		},
	}

	// TODO: Fetch metrics in parallel
	// For now, return empty result
	_ = startTime // will be used when fetching metrics
	return result, nil
}

func formatDuration(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%.0fh", d.Hours())
	}
	return fmt.Sprintf("%.0fd", d.Hours()/24)
}
