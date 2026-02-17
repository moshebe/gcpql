package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/moshebeladev/gcp-metrics/internal/config"
	"github.com/moshebeladev/gcp-metrics/pkg/cloudsql"
	"github.com/moshebeladev/gcp-metrics/pkg/monitoring"
	"github.com/spf13/cobra"
)

var (
	checkSince  string
	checkFormat string
)

var checkCmd = &cobra.Command{
	Use:   "check <instance-id>",
	Short: "Check CloudSQL instance health metrics",
	Long: `Fetch comprehensive CloudSQL metrics including CPU, memory, disk, connections,
query performance, and database health. Returns objective metric values without judgment.

Instance ID formats:
  - Short: my-instance (requires --project)
  - Full: my-project:my-instance
  - Database ID: my-project:us-central1:my-instance

Examples:
  gcp-metrics cloudsql check my-instance
  gcp-metrics cloudsql check my-instance --since 7d
  gcp-metrics cloudsql check my-instance --format table`,
	Args: cobra.ExactArgs(1),
	RunE: runCheck,
}

func init() {
	cloudsqlCmd.AddCommand(checkCmd)
	checkCmd.Flags().StringVar(&checkSince, "since", "24h", "Time window for metrics (e.g., 1h, 24h, 7d)")
	checkCmd.Flags().StringVar(&checkFormat, "format", "json", "Output format: json or table")
}

func runCheck(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	instanceID := args[0]

	// Parse time window
	sinceDuration, err := time.ParseDuration(checkSince)
	if err != nil {
		return fmt.Errorf("invalid --since value: %w", err)
	}

	// Resolve project
	resolvedProject, err := config.ResolveProject(projectID)
	if err != nil && projectID == "" {
		// Try to parse from instance ID
		resolvedProject = ""
	}

	// Parse instance ID
	project, instance, err := cloudsql.ParseInstanceID(instanceID, resolvedProject)
	if err != nil {
		return err
	}

	// Create monitoring client
	monClient, err := monitoring.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create monitoring client: %w", err)
	}

	// Create collector
	collector := cloudsql.NewCollector(monClient)

	// Collect metrics
	result, err := collector.CollectMetrics(ctx, project, instance, sinceDuration)
	if err != nil {
		return fmt.Errorf("failed to collect metrics: %w", err)
	}

	// Format output
	switch checkFormat {
	case "json":
		return cloudsql.FormatJSON(os.Stdout, result)
	case "table":
		return cloudsql.FormatTable(os.Stdout, result)
	default:
		return fmt.Errorf("unknown format: %s", checkFormat)
	}
}
