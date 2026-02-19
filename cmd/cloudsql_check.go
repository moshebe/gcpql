package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/gcp-metrics/gcp-metrics/internal/config"
	"github.com/gcp-metrics/gcp-metrics/pkg/cloudsql"
	"github.com/gcp-metrics/gcp-metrics/pkg/monitoring"
	"github.com/gcp-metrics/gcp-metrics/pkg/timerange"
	"github.com/spf13/cobra"
)

var (
	checkSince         string
	checkFormat        string
	checkQueryInsights bool
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
	checkCmd.Flags().BoolVar(&checkQueryInsights, "query-insights", false, "Fetch top queries from Query Insights (opt-in, slower)")
}

func runCheck(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	instanceID := args[0]

	// Parse time window (supports m, h, d suffixes)
	start, end, err := timerange.Parse(checkSince, "")
	if err != nil {
		return fmt.Errorf("invalid --since value: %w", err)
	}
	sinceDuration := end.Sub(start)

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
	result, err := collector.CollectMetrics(ctx, project, instance, sinceDuration, checkQueryInsights)
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
